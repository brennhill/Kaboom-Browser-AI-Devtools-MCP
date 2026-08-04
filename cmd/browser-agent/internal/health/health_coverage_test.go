// health_coverage_test.go — Additional deterministic unit tests raising coverage of
// command-execution readiness, doctor setup/live checks, and health response builders.

package health

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func captureDiagnostics(t *testing.T, fn func()) string {
	t.Helper()
	previous := diag.Sink()
	var output bytes.Buffer
	diag.SetSink(&output)
	defer diag.SetSink(previous)
	fn()
	return output.String()
}

// newTestCapture builds a fresh capture Store and registers cleanup.
func newTestCapture(t *testing.T) *capture.Capture {
	t.Helper()
	c := capture.NewCapture()
	t.Cleanup(c.Close)
	return c
}

// fakeServerDeps satisfies ServerDeps for buffer/console tests.
type fakeServerDeps struct {
	terminalPort    int
	consoleEntries  int
	consoleCapacity int
	consoleDropped  int64
}

func (f fakeServerDeps) GetTerminalPort() int { return f.terminalPort }
func (f fakeServerDeps) GetConsoleStats() (int, int, int64) {
	return f.consoleEntries, f.consoleCapacity, f.consoleDropped
}

// freePort returns a currently-free TCP port on 127.0.0.1.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

// ---------------------------------------------------------------------------
// BuildCommandExecutionInfo / BuildCommandExecutionInfoAt
// ---------------------------------------------------------------------------

func TestBuildCommandExecutionInfo_NilCapture(t *testing.T) {
	info := BuildCommandExecutionInfo(nil)
	if info.Ready {
		t.Error("nil capture should not be ready")
	}
	if info.Status != "fail" {
		t.Errorf("Status: want fail, got %s", info.Status)
	}
	if info.Detail != "Capture not initialized" {
		t.Errorf("Detail: want 'Capture not initialized', got %q", info.Detail)
	}
}

func TestBuildCommandExecutionInfoAt_Empty(t *testing.T) {
	c := newTestCapture(t)
	info := BuildCommandExecutionInfoAt(c, time.Now())
	if !info.Ready || info.Status != "pass" {
		t.Errorf("empty capture: want ready/pass, got ready=%v status=%s", info.Ready, info.Status)
	}
	if info.RecentFailedCount != 0 {
		t.Errorf("RecentFailedCount: want 0, got %d", info.RecentFailedCount)
	}
	if !strings.Contains(info.Detail, "no recent command failures") {
		t.Errorf("Detail should mention no failures, got %q", info.Detail)
	}
	if info.WindowSeconds != 300 {
		t.Errorf("WindowSeconds: want 300, got %d", info.WindowSeconds)
	}
}

func TestBuildCommandExecutionInfoAt_SuccessCounted(t *testing.T) {
	c := newTestCapture(t)
	c.Queries().RegisterCommand("ok-1", "q", time.Minute)
	c.Queries().ApplyCommandResult("ok-1", "complete", json.RawMessage(`{"done":true}`), "")

	info := BuildCommandExecutionInfoAt(c, time.Now())
	if info.RecentSuccessCount != 1 {
		t.Errorf("RecentSuccessCount: want 1, got %d", info.RecentSuccessCount)
	}
	if info.LastSuccessAt == "" {
		t.Error("LastSuccessAt should be set after a completion")
	}
	if info.Status != "pass" {
		t.Errorf("Status: want pass, got %s", info.Status)
	}
}

func TestBuildCommandExecutionInfoAt_WarnOnSingleFailure(t *testing.T) {
	c := newTestCapture(t)
	c.Queries().RegisterCommand("f-1", "q", time.Minute)
	c.Queries().ExpireCommand("f-1")

	info := BuildCommandExecutionInfoAt(c, time.Now())
	if info.Status != "warn" {
		t.Errorf("Status: want warn for 1 failure, got %s", info.Status)
	}
	if info.RecentFailedCount != 1 || info.RecentExpiredCount != 1 {
		t.Errorf("failed=%d expired=%d, want 1/1", info.RecentFailedCount, info.RecentExpiredCount)
	}
	if info.Ready {
		t.Error("warn state should not be ready")
	}
	if !strings.Contains(info.Detail, "recent failures=1") {
		t.Errorf("Detail should report failures, got %q", info.Detail)
	}
}

func TestBuildCommandExecutionInfoAt_FailOnManyFailures(t *testing.T) {
	c := newTestCapture(t)
	c.Queries().RegisterCommand("ok-1", "q", time.Minute)
	c.Queries().ApplyCommandResult("ok-1", "complete", json.RawMessage(`{}`), "")
	c.Queries().RegisterCommand("f-exp", "q", time.Minute)
	c.Queries().ExpireCommand("f-exp")
	c.Queries().RegisterCommand("f-to", "q", time.Minute)
	c.Queries().ApplyCommandResult("f-to", "timeout", nil, "timed out")
	c.Queries().RegisterCommand("f-err", "q", time.Minute)
	c.Queries().ApplyCommandResult("f-err", "error", nil, "boom")
	c.Queries().RegisterCommand("f-cxl", "q", time.Minute)
	c.Queries().ApplyCommandResult("f-cxl", "cancelled", nil, "user cancelled")

	info := BuildCommandExecutionInfoAt(c, time.Now())
	if info.Status != "fail" {
		t.Fatalf("Status: want fail for >=3 failures, got %s", info.Status)
	}
	if info.RecentFailedCount != 4 {
		t.Errorf("RecentFailedCount: want 4, got %d", info.RecentFailedCount)
	}
	if info.RecentExpiredCount != 1 || info.RecentTimeoutCount != 1 ||
		info.RecentErrorCount != 1 || info.RecentCancelledCount != 1 {
		t.Errorf("per-status counts wrong: exp=%d to=%d err=%d cxl=%d",
			info.RecentExpiredCount, info.RecentTimeoutCount, info.RecentErrorCount, info.RecentCancelledCount)
	}
	// 4 failures / 5 attempts = 80%.
	if info.RecentFailureRatePct < 79 || info.RecentFailureRatePct > 81 {
		t.Errorf("RecentFailureRatePct: want ~80, got %f", info.RecentFailureRatePct)
	}
}

func TestBuildCommandExecutionInfoAt_PendingStallFail(t *testing.T) {
	c := newTestCapture(t)
	c.Queries().RegisterCommand("p-1", "q", 10*time.Minute) // long timeout so it stays pending

	now := time.Now().Add(3 * time.Minute)
	info := BuildCommandExecutionInfoAt(c, now)
	if info.Status != "fail" {
		t.Errorf("Status: want fail for stalled pending backlog, got %s", info.Status)
	}
	if info.PendingCount != 1 {
		t.Errorf("PendingCount: want 1, got %d", info.PendingCount)
	}
	if info.OldestPendingAgeMs <= 0 {
		t.Errorf("OldestPendingAgeMs should be positive, got %d", info.OldestPendingAgeMs)
	}
	if !strings.Contains(info.Detail, "pending backlog") {
		t.Errorf("Detail should mention pending backlog, got %q", info.Detail)
	}
}

func TestBuildCommandExecutionInfoAt_PendingStallWarn(t *testing.T) {
	c := newTestCapture(t)
	c.Queries().RegisterCommand("p-1", "q", 10*time.Minute)

	now := time.Now().Add(1 * time.Minute) // between 45s warn and 2m fail
	info := BuildCommandExecutionInfoAt(c, now)
	if info.Status != "warn" {
		t.Errorf("Status: want warn for aging pending backlog, got %s", info.Status)
	}
	if !strings.Contains(info.Detail, "pending backlog") {
		t.Errorf("Detail should mention pending backlog, got %q", info.Detail)
	}
}

func TestBuildCommandExecutionInfoAt_NegativeAgesClamped(t *testing.T) {
	c := newTestCapture(t)
	c.Queries().RegisterCommand("ok-1", "q", time.Minute)
	c.Queries().ApplyCommandResult("ok-1", "complete", json.RawMessage(`{}`), "")
	c.Queries().RegisterCommand("p-1", "q", 10*time.Minute)

	// now in the past relative to command creation -> ages clamp to 0 / are skipped.
	past := time.Now().Add(-1 * time.Minute)
	info := BuildCommandExecutionInfoAt(c, past)
	if info.Status != "pass" {
		t.Errorf("Status: want pass when events are in the future, got %s", info.Status)
	}
	if info.OldestPendingAgeMs != 0 {
		t.Errorf("OldestPendingAgeMs should clamp to 0, got %d", info.OldestPendingAgeMs)
	}
	if info.RecentSuccessCount != 0 {
		t.Errorf("future success should not count, got %d", info.RecentSuccessCount)
	}
}

// ---------------------------------------------------------------------------
// doctor.go — port + state directory checks
// ---------------------------------------------------------------------------

func TestIsLocalPortAvailable_InUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close() //nolint:errcheck
	port := ln.Addr().(*net.TCPAddr).Port
	if IsLocalPortAvailable(port) {
		t.Errorf("port %d is bound, should not be available", port)
	}
}

func TestSuggestAvailablePort_FindsFree(t *testing.T) {
	start := freePort(t)
	got, ok := SuggestAvailablePort(start, 25)
	if !ok {
		t.Fatal("expected to find an available port")
	}
	if got <= 0 {
		t.Errorf("suggested port should be positive, got %d", got)
	}
}

func TestSuggestAvailablePort_NonPositiveCandidates(t *testing.T) {
	got, ok := SuggestAvailablePort(-5, 3) // all candidates <= 0
	if ok {
		t.Errorf("expected no available port for non-positive candidates, got %d", got)
	}
}

func TestCheckPortAvailability_Available(t *testing.T) {
	port := freePort(t)
	out := captureDiagnostics(t, func() {
		CheckPortAvailability(port, func(p int) string { return "kill" })
	})
	if !strings.Contains(out, "OK") || !strings.Contains(out, "available") {
		t.Errorf("expected OK/available output, got %q", out)
	}
}

func TestCheckPortAvailability_InUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close() //nolint:errcheck
	port := ln.Addr().(*net.TCPAddr).Port

	out := captureDiagnostics(t, func() {
		CheckPortAvailability(port, func(p int) string { return "kill-hint" })
	})
	if !strings.Contains(out, "FAILED") {
		t.Errorf("expected FAILED output for in-use port, got %q", out)
	}
	if !strings.Contains(out, "kill-hint") {
		t.Errorf("expected kill hint in output, got %q", out)
	}
}

func TestCheckStateDirectory_Runs(t *testing.T) {
	out := captureDiagnostics(t, CheckStateDirectory)
	if !strings.Contains(out, "runtime state directory") {
		t.Errorf("expected state directory output, got %q", out)
	}
}

// ---------------------------------------------------------------------------
// RunSetupCheckWithOptions
// ---------------------------------------------------------------------------

func writeTelemetry(t *testing.T, successes, failures int) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "telemetry.jsonl")
	var sb strings.Builder
	for i := 0; i < successes; i++ {
		sb.WriteString(`{"event":"bridge_fastpath_method","success":true,"method":"GET"}` + "\n")
	}
	for i := 0; i < failures; i++ {
		sb.WriteString(`{"event":"bridge_fastpath_method","success":false,"method":"POST","error_code":500}` + "\n")
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("write telemetry: %v", err)
	}
	return path
}

func TestRunSetupCheckWithOptions_DefaultSkipsThreshold(t *testing.T) {
	port := freePort(t)
	logPath := writeTelemetry(t, 3, 0)
	deps := SetupDeps{
		Version:      "test-1.0",
		PortKillHint: func(p int) string { return "kill" },
		FastPathTelemetryLogPath: func() (string, error) {
			return logPath, nil
		},
	}
	var ok bool
	out := captureDiagnostics(t, func() {
		ok = RunSetupCheckWithOptions(port, SetupCheckOptions{}, deps)
	})
	if !ok {
		t.Error("default options (no threshold) should return true")
	}
	if !strings.Contains(out, "KABOOM SETUP CHECK") {
		t.Errorf("expected setup check banner, got %q", out)
	}
}

func TestRunSetupCheckWithOptions_ThresholdPasses(t *testing.T) {
	port := freePort(t)
	logPath := writeTelemetry(t, 60, 0)
	deps := SetupDeps{
		Version:      "test-1.0",
		PortKillHint: func(p int) string { return "kill" },
		FastPathTelemetryLogPath: func() (string, error) {
			return logPath, nil
		},
	}
	var ok bool
	out := captureDiagnostics(t, func() {
		ok = RunSetupCheckWithOptions(port, SetupCheckOptions{MinSamples: 50, MaxFailureRatio: 0.5}, deps)
	})
	if !ok {
		t.Error("within-threshold telemetry should return true")
	}
	if !strings.Contains(out, "within threshold") {
		t.Errorf("expected threshold OK output, got %q", out)
	}
}

func TestRunSetupCheckWithOptions_ThresholdFailsInsufficientSamples(t *testing.T) {
	port := freePort(t)
	logPath := writeTelemetry(t, 2, 0) // fewer than MinSamples
	deps := SetupDeps{
		Version:      "test-1.0",
		PortKillHint: func(p int) string { return "kill" },
		FastPathTelemetryLogPath: func() (string, error) {
			return logPath, nil
		},
	}
	var ok bool
	out := captureDiagnostics(t, func() {
		ok = RunSetupCheckWithOptions(port, SetupCheckOptions{MinSamples: 50, MaxFailureRatio: 0.5}, deps)
	})
	if ok {
		t.Error("insufficient samples should return false")
	}
	if !strings.Contains(out, "FAILED") {
		t.Errorf("expected FAILED output, got %q", out)
	}
}

// ---------------------------------------------------------------------------
// PrintFastPathTelemetryDiagnostics
// ---------------------------------------------------------------------------

func TestPrintFastPathTelemetryDiagnostics_PathError(t *testing.T) {
	var summary FastPathTelemetrySummary
	var ok bool
	out := captureDiagnostics(t, func() {
		summary, ok = PrintFastPathTelemetryDiagnostics(100, func() (string, error) {
			return "", os.ErrInvalid
		})
	})
	if ok {
		t.Error("path error should return ok=false")
	}
	if summary.Total != 0 {
		t.Errorf("summary total: want 0, got %d", summary.Total)
	}
	if !strings.Contains(out, "FAILED") {
		t.Errorf("expected FAILED output, got %q", out)
	}
}

func TestPrintFastPathTelemetryDiagnostics_NoFileYet(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.jsonl")
	var ok bool
	out := captureDiagnostics(t, func() {
		_, ok = PrintFastPathTelemetryDiagnostics(100, func() (string, error) {
			return missing, nil
		})
	})
	if ok {
		t.Error("missing telemetry file should return ok=false")
	}
	if !strings.Contains(out, "no fast-path telemetry recorded yet") {
		t.Errorf("expected 'no telemetry yet' output, got %q", out)
	}
}

func TestPrintFastPathTelemetryDiagnostics_WithData(t *testing.T) {
	logPath := writeTelemetry(t, 3, 2) // 3 GET success, 2 POST failure w/ error_code 500
	var summary FastPathTelemetrySummary
	var ok bool
	out := captureDiagnostics(t, func() {
		summary, ok = PrintFastPathTelemetryDiagnostics(100, func() (string, error) {
			return logPath, nil
		})
	})
	if !ok {
		t.Fatal("existing telemetry file should return ok=true")
	}
	if summary.Total != 5 || summary.Success != 3 || summary.Failure != 2 {
		t.Errorf("summary: total=%d success=%d failure=%d, want 5/3/2", summary.Total, summary.Success, summary.Failure)
	}
	if !strings.Contains(out, "Methods:") {
		t.Errorf("expected methods breakdown, got %q", out)
	}
	if !strings.Contains(out, "Error codes: 500=2") {
		t.Errorf("expected error code breakdown, got %q", out)
	}
}

func TestPrintFastPathTelemetryDiagnostics_NoErrorCodes(t *testing.T) {
	logPath := writeTelemetry(t, 4, 0)
	out := captureDiagnostics(t, func() {
		PrintFastPathTelemetryDiagnostics(100, func() (string, error) {
			return logPath, nil
		})
	})
	if !strings.Contains(out, "Error codes: none") {
		t.Errorf("expected 'Error codes: none', got %q", out)
	}
}

// ---------------------------------------------------------------------------
// RunDoctorChecks / HandleDoctorHTTP
// ---------------------------------------------------------------------------

// findCheck returns the DoctorCheck with the given name, or fails.
func findCheck(t *testing.T, checks []DoctorCheck, name string) DoctorCheck {
	t.Helper()
	for _, c := range checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("check %q not found", name)
	return DoctorCheck{}
}

func TestRunDoctorChecks_HealthyState(t *testing.T) {
	c := newTestCapture(t)
	capturefixture.Connect(c)
	capturefixture.SetPilot(c, true)
	capturefixture.Track(c, 42, "https://example.com")

	checks := RunDoctorChecks(c)
	if got := findCheck(t, checks, "extension_connected").Status; got != "pass" {
		t.Errorf("extension_connected: want pass, got %s", got)
	}
	if got := findCheck(t, checks, "pilot_enabled").Status; got != "pass" {
		t.Errorf("pilot_enabled: want pass, got %s", got)
	}
	if got := findCheck(t, checks, "tracked_tab").Status; got != "pass" {
		t.Errorf("tracked_tab: want pass, got %s", got)
	}
	if got := findCheck(t, checks, "circuit_breaker").Status; got != "pass" {
		t.Errorf("circuit_breaker: want pass, got %s", got)
	}
	if got := findCheck(t, checks, "command_queue").Status; got != "pass" {
		t.Errorf("command_queue: want pass, got %s", got)
	}
}

func TestRunDoctorChecks_DegradedDefaults(t *testing.T) {
	c := newTestCapture(t) // not connected, pilot assumed, no tracked tab

	checks := RunDoctorChecks(c)
	if got := findCheck(t, checks, "extension_connected").Status; got != "fail" {
		t.Errorf("extension_connected: want fail, got %s", got)
	}
	pilot := findCheck(t, checks, "pilot_enabled")
	if pilot.Status != "warn" {
		t.Errorf("pilot_enabled: want warn (assumed), got %s", pilot.Status)
	}
	if got := findCheck(t, checks, "tracked_tab").Status; got != "warn" {
		t.Errorf("tracked_tab: want warn, got %s", got)
	}
}

func TestRunDoctorChecks_PilotExplicitlyDisabled(t *testing.T) {
	c := newTestCapture(t)
	capturefixture.SetPilot(c, false) // explicitly disabled

	pilot := findCheck(t, RunDoctorChecks(c), "pilot_enabled")
	if pilot.Status != "warn" {
		t.Errorf("pilot_enabled: want warn, got %s", pilot.Status)
	}
	if !strings.Contains(pilot.Detail, "explicitly disabled") {
		t.Errorf("expected explicitly-disabled detail, got %q", pilot.Detail)
	}
}

func TestRunDoctorChecks_CommandQueuePendingPass(t *testing.T) {
	c := newTestCapture(t)
	for i := 0; i < 2; i++ {
		if _, err := c.Queries().CreatePendingQuery(queries.PendingQuery{Type: "dom", Params: json.RawMessage("{}")}); err != nil {
			t.Fatalf("CreatePendingQuery: %v", err)
		}
	}
	cq := findCheck(t, RunDoctorChecks(c), "command_queue")
	if cq.Status != "pass" {
		t.Errorf("command_queue with 2 pending: want pass, got %s", cq.Status)
	}
	if !strings.Contains(cq.Detail, "2 pending") {
		t.Errorf("expected '2 pending' detail, got %q", cq.Detail)
	}
}

func TestRunDoctorChecks_CommandQueueWarn(t *testing.T) {
	c := newTestCapture(t)
	for i := 0; i < 6; i++ {
		if _, err := c.Queries().CreatePendingQuery(queries.PendingQuery{Type: "dom", Params: json.RawMessage("{}")}); err != nil {
			t.Fatalf("CreatePendingQuery: %v", err)
		}
	}
	cq := findCheck(t, RunDoctorChecks(c), "command_queue")
	if cq.Status != "warn" {
		t.Errorf("command_queue with 6 pending: want warn, got %s", cq.Status)
	}
	if cq.Fix == "" {
		t.Error("warn command_queue should include a fix hint")
	}
}

func TestRunDoctorChecks_CommandExecutionFailSetsFix(t *testing.T) {
	c := newTestCapture(t)
	for _, id := range []string{"a", "b", "c"} {
		c.Queries().RegisterCommand(id, "q", time.Minute)
		c.Queries().ApplyCommandResult(id, "error", nil, "boom")
	}
	ce := findCheck(t, RunDoctorChecks(c), "command_execution")
	if ce.Status == "pass" {
		t.Errorf("command_execution should not be pass with 3 errors, got %s", ce.Status)
	}
	if ce.Fix == "" {
		t.Error("non-pass command_execution should set a Fix")
	}
}

func TestHandleDoctorHTTP_Unhealthy(t *testing.T) {
	c := newTestCapture(t) // extension not connected -> fail
	w := httptest.NewRecorder()
	HandleDoctorHTTP(w, c, "9.9.9")

	var body struct {
		Status              string        `json:"status"`
		ReadyForInteraction bool          `json:"ready_for_interaction"`
		Version             string        `json:"version"`
		Checks              []DoctorCheck `json:"checks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal doctor body: %v", err)
	}
	if body.Status != "unhealthy" {
		t.Errorf("status: want unhealthy, got %s", body.Status)
	}
	if body.ReadyForInteraction {
		t.Error("unhealthy should not be ready for interaction")
	}
	if body.Version != "9.9.9" {
		t.Errorf("version: want 9.9.9, got %s", body.Version)
	}
	if len(body.Checks) == 0 {
		t.Error("expected checks in response")
	}
}

func TestHandleDoctorHTTP_Healthy(t *testing.T) {
	c := newTestCapture(t)
	capturefixture.Connect(c)
	capturefixture.SetPilot(c, true)
	capturefixture.Track(c, 1, "https://example.com")

	w := httptest.NewRecorder()
	HandleDoctorHTTP(w, c, "1.0")

	var body struct {
		Status              string `json:"status"`
		ReadyForInteraction bool   `json:"ready_for_interaction"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Status != "healthy" {
		t.Errorf("status: want healthy, got %s", body.Status)
	}
	if !body.ReadyForInteraction {
		t.Error("healthy should be ready for interaction")
	}
}

func TestHandleDoctorHTTP_Degraded(t *testing.T) {
	c := newTestCapture(t)
	capturefixture.Connect(c)
	capturefixture.SetPilot(c, true)
	// No tracked tab -> tracked_tab warn (no fail) -> degraded.

	w := httptest.NewRecorder()
	HandleDoctorHTTP(w, c, "1.0")

	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Status != "degraded" {
		t.Errorf("status: want degraded, got %s", body.Status)
	}
}

func TestAIAuthDoctorCheckClassifiesSubscriptionAndAPIBilling(t *testing.T) {
	runtime := defaultDoctorCommandRuntime()
	runtime.lookPath = func(name string) (string, error) { return "/usr/local/bin/" + name, nil }
	runtime.commandOutput = func(_ time.Duration, name string, _ ...string) ([]byte, error) {
		if name == "claude" {
			return []byte(`{"loggedIn":true,"authMethod":"claude.ai","subscriptionType":"max"}`), nil
		}
		return []byte("Logged in using an API key"), nil
	}

	claude := runAIAuthDoctorCheck(runtime, "claude")
	if claude.Status != "pass" || !strings.Contains(claude.Detail, "subscription") {
		t.Fatalf("Claude check = %+v, want subscription pass", claude)
	}
	codex := runAIAuthDoctorCheck(runtime, "codex")
	if codex.Status != "warn" || !strings.Contains(codex.Detail, "API billing") {
		t.Fatalf("Codex check = %+v, want API billing warning", codex)
	}
}

func TestAIAuthDoctorCheckSurfacesKeychainFailureWithoutAccountData(t *testing.T) {
	runtime := defaultDoctorCommandRuntime()
	runtime.lookPath = func(name string) (string, error) { return "/usr/local/bin/" + name, nil }
	runtime.commandOutput = func(_ time.Duration, _ string, _ ...string) ([]byte, error) {
		return []byte(`Keychain Not Found: cannot store "private-user@example.com"`), errors.New("exit 1")
	}

	check := runAIAuthDoctorCheck(runtime, "claude")
	if check.Status != "fail" || !strings.Contains(strings.ToLower(check.Detail), "keychain") {
		t.Fatalf("check = %+v, want keychain failure", check)
	}
	if strings.Contains(check.Detail, "private-user") {
		t.Fatalf("doctor detail leaked account identifier: %q", check.Detail)
	}
}

// ---------------------------------------------------------------------------
// GetHealth / response builders
// ---------------------------------------------------------------------------

func TestGetHealth_FullResponse(t *testing.T) {
	m := NewMetrics()
	m.IncrementRequest("observe")
	c := newTestCapture(t)
	server := fakeServerDeps{terminalPort: 9222, consoleEntries: 5, consoleCapacity: 100, consoleDropped: 2}
	up := &fakeUpgradeProvider{pending: true, newVer: "1.2.3", detectedAt: time.Now()}
	getLaunch := func() LaunchModeInfo {
		return LaunchModeInfo{Mode: "cli", Reason: "manual", ParentProcess: "sh"}
	}

	resp := m.GetHealth(c, server, up, getLaunch, nil, "8.8.8")
	if resp.Server.Version != "8.8.8" {
		t.Errorf("Server.Version: want 8.8.8, got %s", resp.Server.Version)
	}
	if resp.Server.TerminalPort != 9222 {
		t.Errorf("TerminalPort: want 9222, got %d", resp.Server.TerminalPort)
	}
	if resp.Server.LaunchMode != "cli" {
		t.Errorf("LaunchMode: want cli, got %s", resp.Server.LaunchMode)
	}
	if resp.Buffers.Console.Entries != 5 {
		t.Errorf("Console.Entries: want 5, got %d", resp.Buffers.Console.Entries)
	}
	if resp.Buffers.Console.DroppedCount != 2 {
		t.Errorf("Console.DroppedCount: want 2, got %d", resp.Buffers.Console.DroppedCount)
	}
	if resp.Upgrade == nil || !resp.Upgrade.Pending {
		t.Error("expected pending upgrade info")
	}
	if resp.Audit.TotalCalls != 1 {
		t.Errorf("Audit.TotalCalls: want 1, got %d", resp.Audit.TotalCalls)
	}
}

func TestGetHealth_NilServerAndUpgrade(t *testing.T) {
	m := NewMetrics()
	c := newTestCapture(t)
	getLaunch := func() LaunchModeInfo { return LaunchModeInfo{Mode: "auto"} }

	resp := m.GetHealth(c, nil, nil, getLaunch, nil, "1.0")
	if resp.Server.TerminalPort != 0 {
		t.Errorf("nil server: TerminalPort should be 0, got %d", resp.Server.TerminalPort)
	}
	if resp.Upgrade != nil {
		t.Error("nil upgrade provider should yield nil Upgrade")
	}
	// Console falls back to defaults when server is nil.
	if resp.Buffers.Console.Capacity != defaultMaxEntries {
		t.Errorf("Console.Capacity: want %d, got %d", defaultMaxEntries, resp.Buffers.Console.Capacity)
	}
}

func TestBuildBuffersInfo_NilCaptureAndServer(t *testing.T) {
	info := BuildBuffersInfo(nil, nil)
	if info.Console.Capacity != defaultMaxEntries {
		t.Errorf("Console.Capacity: want %d, got %d", defaultMaxEntries, info.Console.Capacity)
	}
	if info.Network.Entries != 0 {
		t.Errorf("Network.Entries: want 0, got %d", info.Network.Entries)
	}
	if info.Network.Capacity <= 0 {
		t.Error("Network.Capacity should be positive")
	}
}

func TestBuildRateLimitInfo(t *testing.T) {
	nilInfo := BuildRateLimitInfo(nil)
	if nilInfo.Threshold <= 0 {
		t.Errorf("Threshold should be positive even for nil capture, got %d", nilInfo.Threshold)
	}
	if nilInfo.CircuitOpen {
		t.Error("nil capture: CircuitOpen should be false")
	}

	c := newTestCapture(t)
	info := BuildRateLimitInfo(c)
	if info.Threshold <= 0 {
		t.Errorf("Threshold should be positive, got %d", info.Threshold)
	}
	if info.CircuitOpen {
		t.Error("fresh capture: circuit should be closed")
	}
}

func TestBuildPilotInfo(t *testing.T) {
	if got := BuildPilotInfo(nil); got.Source != "never_connected" {
		t.Errorf("nil capture: Source want never_connected, got %s", got.Source)
	}

	c := newTestCapture(t)
	capturefixture.SetPilot(c, true)
	info := BuildPilotInfo(c)
	if !info.Enabled {
		t.Error("pilot should be enabled")
	}
	if info.Source == "" {
		t.Error("Source should be populated")
	}
}
