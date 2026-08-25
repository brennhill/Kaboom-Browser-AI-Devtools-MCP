// health_test.go — Unit tests for the health sub-package exported API.

package health

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestHandleDoctorMCPReportsReadinessExtraChecksAndHint(t *testing.T) {
	t.Parallel()
	captureStore := newTestCapture(t)
	response := HandleDoctorMCP(
		DoctorMCPDeps{
			Metrics:        NewMetrics(),
			Capture:        captureStore,
			DiagnosticHint: func() string { return "inspect local lifecycle logs" },
			ExtraChecks:    []DoctorCheck{{Name: "fixture_recovery", Status: "warn", Detail: "recovery pending"}},
		},
		mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 42},
		"test-version",
	)
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.IsError || len(result.Content) == 0 {
		t.Fatalf("Doctor response = %+v", result)
	}
	for _, want := range []string{"Doctor: unhealthy", "ready_for_interaction", "fixture_recovery", "server_uptime", "inspect local lifecycle logs"} {
		if !strings.Contains(result.Content[0].Text, want) {
			t.Errorf("Doctor response missing %q: %s", want, result.Content[0].Text)
		}
	}
}

func TestRunDoctorChecksSurfacesExtensionStateRecovery(t *testing.T) {
	c := newTestCapture(t)
	c.ExtensionLogs().Add([]types.ExtensionLog{{
		Timestamp: time.Date(2026, 7, 30, 8, 0, 0, 0, time.UTC),
		Level:     "warn", Category: "state_recovery", Message: "Persisted extension state recovered",
		Data: json.RawMessage(`{"name":"tracked_tab_state","detail":"Saved tracking state was malformed; automatic tracking is active.","fix":"Choose a tracked tab again.","lifecycle":"active","correlation_id":"track-123","expected_next_transition":"tracking_confirmed","recovery_attempt":2,"recovery_outcome":"retrying"}`),
	}, {
		Timestamp: time.Date(2026, 7, 30, 8, 1, 0, 0, time.UTC),
		Level:     "info", Category: "state_recovery", Message: "Persisted extension state verified",
		Data: json.RawMessage(`{"name":"tracked_tab_state","detail":"","fix":"","lifecycle":"recovered"}`),
	}})

	check := findCheck(t, RunDoctorChecks(c), "tracked_tab_state")
	if check.Status != "pass" || check.Lifecycle != "recovered" || check.RecoveredAt == "" {
		t.Fatalf("extension recovery check = %#v, want recovered lifecycle", check)
	}
	if check.Occurrences != 1 || len(check.History) != 2 {
		t.Fatalf("extension recovery history = %#v, want one occurrence and two transitions", check)
	}
	if check.CorrelationID != "track-123" || check.LastSuccessfulTransition != "state_verified" ||
		check.RecoveryAttempt != 2 || check.RecoveryOutcome != "recovered" || check.History[0].Event != "failure_detected" {
		t.Fatalf("extension recovery timeline = %#v", check)
	}
}

func TestRunDoctorChecksIgnoresRecoveryWithoutPriorFailure(t *testing.T) {
	c := newTestCapture(t)
	c.ExtensionLogs().Add([]types.ExtensionLog{{
		Timestamp: time.Date(2026, 7, 30, 8, 1, 0, 0, time.UTC),
		Level:     "info", Category: "state_recovery", Message: "Persisted extension state verified",
		Data: json.RawMessage(`{"name":"popup_state","detail":"","fix":"","lifecycle":"recovered"}`),
	}})

	for _, check := range RunDoctorChecks(c) {
		if check.Name == "popup_state" {
			t.Fatalf("recovery-only transition created historical failure: %#v", check)
		}
	}
}

func TestRunDoctorChecksIncludesExtensionDiagnosticLifecycle(t *testing.T) {
	c := newTestCapture(t)
	c.ExtensionLogs().Add([]types.ExtensionLog{{
		Timestamp: time.Date(2026, 8, 2, 8, 0, 0, 0, time.UTC),
		Level:     "debug", Category: "diagnostic_lifecycle", Message: "Extension worker started",
		Data: json.RawMessage(`{"event":"worker_started","correlation_id":"ext-1"}`),
	}, {
		Timestamp: time.Date(2026, 8, 2, 8, 1, 0, 0, time.UTC),
		Level:     "warn", Category: "diagnostic_queue", Message: "Diagnostic queue saturated",
		Data: json.RawMessage(`{"dropped_count":7,"capacity":200}`),
	}, {
		Timestamp: time.Date(2026, 8, 2, 8, 2, 0, 0, time.UTC),
		Level:     "debug", Category: "diagnostic_lifecycle", Message: "Extension sync connected",
		Data: json.RawMessage(`{"event":"sync_connected","correlation_id":"ext-1","lifecycle_sequence":["worker_suspend","worker_started","sync_connected"]}`),
	}})

	check := findCheck(t, RunDoctorChecks(c), "extension_diagnostics")
	if check.Status != "warn" {
		t.Fatalf("extension diagnostics status = %q, want warn", check.Status)
	}
	if !strings.Contains(check.Detail, "worker_suspend -> worker_started -> sync_connected") || !strings.Contains(check.Detail, "7 dropped") {
		t.Fatalf("extension diagnostics detail = %q", check.Detail)
	}
}

func TestDoctorExtensionDiagnosticsRejectMalformedAndMapLegacyEvents(t *testing.T) {
	if _, ok := extensionDiagnosticLifecycleCheck(nil); ok {
		t.Fatal("nil capture produced extension diagnostics")
	}
	c := newTestCapture(t)
	c.ExtensionLogs().Add([]types.ExtensionLog{
		{Category: "connection", Message: "ignored", Data: json.RawMessage(`not-json`)},
		{Category: "connection", Message: "Sync connected", Data: json.RawMessage(`{}`)},
		{Category: "connection", Message: "Sync disconnected", Data: json.RawMessage(`{}`)},
		{Category: "connection", Message: "[Sync] Sync failed, retrying", Data: json.RawMessage(`{}`)},
	})
	check, ok := extensionDiagnosticLifecycleCheck(c)
	if !ok || check.Status != "pass" || !strings.Contains(check.Detail, "sync_connected -> sync_disconnected -> sync_failed") {
		t.Fatalf("legacy lifecycle check = %#v, %t", check, ok)
	}
}

func TestDoctorStateRecoveryDefaultsAndBoundsHistory(t *testing.T) {
	if checks := extensionStateRecoveryChecks(nil); checks != nil {
		t.Fatalf("nil capture checks = %#v", checks)
	}
	c := newTestCapture(t)
	logs := []types.ExtensionLog{
		{Category: "other", Data: json.RawMessage(`{}`)},
		{Category: "state_recovery", Data: json.RawMessage(`not-json`)},
		{Category: "state_recovery", Data: json.RawMessage(`{"name":"ignored","lifecycle":"unknown"}`)},
	}
	for i := 0; i < 22; i++ {
		logs = append(logs, types.ExtensionLog{
			Timestamp: time.Date(2026, 8, 2, 9, i, 0, 0, time.UTC),
			Category:  "state_recovery",
			Data:      json.RawMessage(`{"name":"storage_state","detail":"fallback active"}`),
		})
	}
	c.ExtensionLogs().Add(logs)
	checks := extensionStateRecoveryChecks(c)
	if len(checks) != 1 || checks[0].Status != "warn" || checks[0].ExpectedNextTransition != "state_verified" ||
		checks[0].RecoveryOutcome != "pending" || checks[0].RecoveryAttempt != 22 || len(checks[0].History) != 20 {
		t.Fatalf("bounded recovery = %#v", checks)
	}
}

func TestAIAuthDoctorCheckClassifiesAbsentUnauthenticatedAndUnknown(t *testing.T) {
	runtime := defaultDoctorCommandRuntime()
	runtime.lookPath = func(string) (string, error) { return "", errors.New("missing") }
	if check := runAIAuthDoctorCheck(runtime, "codex"); check.Status != "pass" {
		t.Fatalf("absent optional CLI = %#v", check)
	}
	runtime.lookPath = func(tool string) (string, error) { return "/bin/" + tool, nil }
	runtime.commandOutput = func(time.Duration, string, ...string) ([]byte, error) {
		return []byte("not logged in"), errors.New("exit 1")
	}
	if check := runAIAuthDoctorCheck(runtime, "codex"); check.Status != "warn" || !strings.Contains(check.Detail, "not authenticated") {
		t.Fatalf("unauthenticated CLI = %#v", check)
	}
	runtime.commandOutput = func(time.Duration, string, ...string) ([]byte, error) { return []byte("authenticated"), nil }
	if check := runAIAuthDoctorCheck(runtime, "claude"); check.Status != "warn" || !strings.Contains(check.Detail, "could not be determined") {
		t.Fatalf("unknown provider = %#v", check)
	}
}

// ---------------------------------------------------------------------------
// Metrics
// ---------------------------------------------------------------------------

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}
	if m.GetTotalRequests() != 0 {
		t.Errorf("expected 0 total requests, got %d", m.GetTotalRequests())
	}
	if m.GetTotalErrors() != 0 {
		t.Errorf("expected 0 total errors, got %d", m.GetTotalErrors())
	}
}

func TestMetrics_IncrementRequest(t *testing.T) {
	m := NewMetrics()
	m.IncrementRequest("observe")
	m.IncrementRequest("observe")
	m.IncrementRequest("interact")

	if got := m.GetRequestCount("observe"); got != 2 {
		t.Errorf("observe request count: want 2, got %d", got)
	}
	if got := m.GetRequestCount("interact"); got != 1 {
		t.Errorf("interact request count: want 1, got %d", got)
	}
	if got := m.GetRequestCount("unknown"); got != 0 {
		t.Errorf("unknown tool request count: want 0, got %d", got)
	}
	if got := m.GetTotalRequests(); got != 3 {
		t.Errorf("total requests: want 3, got %d", got)
	}
}

func TestMetrics_IncrementError(t *testing.T) {
	m := NewMetrics()
	m.IncrementError("observe")
	m.IncrementError("observe")
	m.IncrementError("generate")

	if got := m.GetErrorCount("observe"); got != 2 {
		t.Errorf("observe error count: want 2, got %d", got)
	}
	if got := m.GetTotalErrors(); got != 3 {
		t.Errorf("total errors: want 3, got %d", got)
	}
}

func TestMetrics_GetUptime(t *testing.T) {
	m := NewMetrics()
	m.startTime = time.Now().Add(-5 * time.Millisecond)
	uptime := m.GetUptime()
	if uptime < 5*time.Millisecond {
		t.Errorf("uptime too short: %v", uptime)
	}
}

func TestMetrics_ConcurrentAccess(t *testing.T) {
	m := NewMetrics()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			m.IncrementRequest("tool")
		}()
		go func() {
			defer wg.Done()
			m.IncrementError("tool")
		}()
	}
	wg.Wait()
	if got := m.GetTotalRequests(); got != 100 {
		t.Errorf("concurrent requests: want 100, got %d", got)
	}
	if got := m.GetTotalErrors(); got != 100 {
		t.Errorf("concurrent errors: want 100, got %d", got)
	}
}

func TestMetrics_BuildAuditInfo(t *testing.T) {
	m := NewMetrics()
	m.IncrementRequest("observe")
	m.IncrementRequest("observe")
	m.IncrementRequest("interact")
	m.IncrementError("observe")

	info := m.BuildAuditInfo()
	if info.TotalCalls != 3 {
		t.Errorf("TotalCalls: want 3, got %d", info.TotalCalls)
	}
	if info.TotalErrors != 1 {
		t.Errorf("TotalErrors: want 1, got %d", info.TotalErrors)
	}
	// 1 error / 3 calls = ~33.33%
	if info.ErrorRatePct < 33.0 || info.ErrorRatePct > 34.0 {
		t.Errorf("ErrorRatePct: want ~33.33, got %f", info.ErrorRatePct)
	}
	if info.CallsPerTool["observe"] != 2 {
		t.Errorf("CallsPerTool[observe]: want 2, got %d", info.CallsPerTool["observe"])
	}
}

func TestMetrics_BuildAuditInfo_ZeroCalls(t *testing.T) {
	m := NewMetrics()
	info := m.BuildAuditInfo()
	if info.ErrorRatePct != 0 {
		t.Errorf("ErrorRatePct with zero calls should be 0, got %f", info.ErrorRatePct)
	}
}

// ---------------------------------------------------------------------------
// CalcUtilization
// ---------------------------------------------------------------------------

func TestCalcUtilization(t *testing.T) {
	tests := []struct {
		name     string
		entries  int
		capacity int
		want     float64
	}{
		{"empty", 0, 100, 0},
		{"half", 50, 100, 50},
		{"full", 100, 100, 100},
		{"over", 150, 100, 150},
		{"zero capacity", 10, 0, 0},
		{"negative capacity", 10, -1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalcUtilization(tt.entries, tt.capacity)
			if got != tt.want {
				t.Errorf("CalcUtilization(%d, %d) = %f, want %f", tt.entries, tt.capacity, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BuildUpgradeInfo
// ---------------------------------------------------------------------------

type fakeUpgradeProvider struct {
	pending    bool
	newVer     string
	detectedAt time.Time
}

func (f *fakeUpgradeProvider) UpgradeInfo() (bool, string, time.Time) {
	return f.pending, f.newVer, f.detectedAt
}

func TestBuildUpgradeInfo_Nil(t *testing.T) {
	if got := BuildUpgradeInfo(nil); got != nil {
		t.Errorf("expected nil for nil provider, got %+v", got)
	}
}

func TestBuildUpgradeInfo_NotPending(t *testing.T) {
	p := &fakeUpgradeProvider{pending: false}
	if got := BuildUpgradeInfo(p); got != nil {
		t.Errorf("expected nil when not pending, got %+v", got)
	}
}

func TestBuildUpgradeInfo_Pending(t *testing.T) {
	now := time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)
	p := &fakeUpgradeProvider{pending: true, newVer: "0.9.0", detectedAt: now}
	got := BuildUpgradeInfo(p)
	if got == nil {
		t.Fatal("expected non-nil UpgradeInfo")
	}
	if !got.Pending {
		t.Error("expected Pending to be true")
	}
	if got.NewVersion != "0.9.0" {
		t.Errorf("NewVersion: want 0.9.0, got %s", got.NewVersion)
	}
	if got.DetectedAt != "2026-03-29T12:00:00Z" {
		t.Errorf("DetectedAt: want 2026-03-29T12:00:00Z, got %s", got.DetectedAt)
	}
}

// ---------------------------------------------------------------------------
// DoctorCheck type construction
// ---------------------------------------------------------------------------

func TestDoctorCheck_Construction(t *testing.T) {
	check := DoctorCheck{
		Name:   "test_check",
		Status: "pass",
		Detail: "Everything is fine",
		Fix:    "",
	}
	if check.Name != "test_check" {
		t.Errorf("Name: want test_check, got %s", check.Name)
	}
	if check.Status != "pass" {
		t.Errorf("Status: want pass, got %s", check.Status)
	}
}

// ---------------------------------------------------------------------------
// EvaluateFastPathFailureThreshold
// ---------------------------------------------------------------------------

func TestEvaluateFastPathFailureThreshold(t *testing.T) {
	tests := []struct {
		name            string
		summary         FastPathTelemetrySummary
		minSamples      int
		maxFailureRatio float64
		wantErr         bool
	}{
		{
			name:            "negative ratio skips check",
			summary:         FastPathTelemetrySummary{Total: 100, Failure: 50},
			minSamples:      10,
			maxFailureRatio: -1,
			wantErr:         false,
		},
		{
			name:            "ratio > 1 is invalid",
			summary:         FastPathTelemetrySummary{Total: 100},
			minSamples:      10,
			maxFailureRatio: 1.5,
			wantErr:         true,
		},
		{
			name:            "minSamples < 1 is invalid",
			summary:         FastPathTelemetrySummary{Total: 100},
			minSamples:      0,
			maxFailureRatio: 0.5,
			wantErr:         true,
		},
		{
			name:            "insufficient samples",
			summary:         FastPathTelemetrySummary{Total: 5},
			minSamples:      10,
			maxFailureRatio: 0.5,
			wantErr:         true,
		},
		{
			name:            "within threshold",
			summary:         FastPathTelemetrySummary{Total: 100, Failure: 5},
			minSamples:      10,
			maxFailureRatio: 0.1,
			wantErr:         false,
		},
		{
			name:            "exceeds threshold",
			summary:         FastPathTelemetrySummary{Total: 100, Failure: 20},
			minSamples:      10,
			maxFailureRatio: 0.1,
			wantErr:         true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EvaluateFastPathFailureThreshold(tt.summary, tt.minSamples, tt.maxFailureRatio)
			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SummarizeFastPathTelemetryLog
// ---------------------------------------------------------------------------

func TestSummarizeFastPathTelemetryLog_ZeroMaxLines(t *testing.T) {
	s := SummarizeFastPathTelemetryLog("/nonexistent", 0)
	if s.Total != 0 {
		t.Errorf("expected 0 total for maxLines=0, got %d", s.Total)
	}
}

func TestSummarizeFastPathTelemetryLog_NonexistentFile(t *testing.T) {
	s := SummarizeFastPathTelemetryLog("/tmp/nonexistent-telemetry-log-12345", 100)
	if s.Total != 0 {
		t.Errorf("expected 0 total for missing file, got %d", s.Total)
	}
}

func TestSummarizeFastPathTelemetryLog_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "telemetry.jsonl")
	content := `{"event":"bridge_fastpath_method","success":true,"method":"GET"}
{"event":"bridge_fastpath_method","success":false,"method":"POST","error_code":500}
{"event":"bridge_fastpath_method","success":true,"method":"GET"}
{"event":"other_event","success":true}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := SummarizeFastPathTelemetryLog(path, 100)
	if s.Total != 3 {
		t.Errorf("Total: want 3, got %d", s.Total)
	}
	if s.Success != 2 {
		t.Errorf("Success: want 2, got %d", s.Success)
	}
	if s.Failure != 1 {
		t.Errorf("Failure: want 1, got %d", s.Failure)
	}
	if s.Methods["GET"] != 2 {
		t.Errorf("Methods[GET]: want 2, got %d", s.Methods["GET"])
	}
	if s.ErrorCodes[500] != 1 {
		t.Errorf("ErrorCodes[500]: want 1, got %d", s.ErrorCodes[500])
	}
}

// ---------------------------------------------------------------------------
// BuildMemoryInfo (nil capture)
// ---------------------------------------------------------------------------

func TestBuildMemoryInfo_NilCapture(t *testing.T) {
	info := BuildMemoryInfo(nil)
	// Should still return valid info from runtime.MemStats
	if info.SysMB <= 0 {
		t.Error("expected positive SysMB from runtime stats")
	}
}

// ---------------------------------------------------------------------------
// Response type construction
// ---------------------------------------------------------------------------

func TestMCPHealthResponse_Construction(t *testing.T) {
	resp := MCPHealthResponse{
		Server: ServerInfo{Version: "0.8.2", PID: 1234},
		Memory: MemoryInfo{CurrentMB: 10.5},
	}
	if resp.Server.Version != "0.8.2" {
		t.Errorf("Version: want 0.8.2, got %s", resp.Server.Version)
	}
	if resp.Upgrade != nil {
		t.Error("expected nil Upgrade by default")
	}
}

func TestBuildResourcePressureChecksDistinguishesDisposableDropsFromActiveSaturation(t *testing.T) {
	cap := capture.NewCapture()
	cap.Telemetry().AddNetworkBodies(make([]types.NetworkBody, cap.Telemetry().Pressure().Network.Capacity+1))
	checks := BuildResourcePressureChecks(cap, nil)
	if len(checks) != 1 || checks[0].Name != "resource_pressure_network" || checks[0].Status != "pass" {
		t.Fatalf("disposable pressure checks = %#v, want one recovered/pass network check", checks)
	}

	for i := 0; i < queries.MaxPendingQueries; i++ {
		if _, err := cap.Queries().CreatePendingQuery(queries.PendingQuery{Type: "pressure"}); err != nil {
			t.Fatal(err)
		}
	}
	checks = BuildResourcePressureChecks(cap, nil)
	foundWarn := false
	for _, check := range checks {
		if check.Name == "resource_pressure_pending_commands" && check.Status == "warn" {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Fatalf("active command saturation missing warning: %#v", checks)
	}
}

func TestCommandExecutionInfo_Defaults(t *testing.T) {
	info := CommandExecutionInfo{Ready: true, Status: "pass"}
	if !info.Ready {
		t.Error("expected Ready=true")
	}
	if info.QueueDepth != 0 {
		t.Error("expected QueueDepth=0 by default")
	}
}
