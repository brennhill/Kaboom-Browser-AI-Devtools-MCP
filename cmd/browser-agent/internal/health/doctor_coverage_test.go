// doctor_coverage_test.go — Behavioural tests for the doctor/setup diagnostic surfaces.
//
// Focus is the UNHEALTHY reporting paths: port already in use, extension
// disconnected, pilot disabled, no tracked tab, backed-up command queue,
// failed/stalled commands, missing or unreadable telemetry logs.
//
// Determinism notes:
//   - No network: the only sockets opened bind 127.0.0.1:0 (an ephemeral local
//     port), never a hostname, so nothing resolves DNS.
//   - No sleeps: command-age behaviour is driven through
//     BuildCommandExecutionInfoAt(cap, now) with a synthetic clock.
//   - Tests that capture os.Stdout or call t.Setenv must not use t.Parallel().

package health

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// captureStdout redirects os.Stdout for the duration of fn and returns what was
// written. Reads concurrently so output larger than the pipe buffer cannot
// deadlock. Callers must not use t.Parallel().
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	func() {
		defer func() {
			os.Stdout = orig
			_ = w.Close()
		}()
		fn()
	}()

	out := <-done
	_ = r.Close()
	return out
}

// occupiedPort binds an ephemeral loopback port and keeps it held for the whole
// test, so the returned port is guaranteed unavailable. Loopback only — no DNS.
func occupiedPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	return ln.Addr().(*net.TCPAddr).Port
}

// freePort binds then immediately releases an ephemeral loopback port. There is
// an inherent race (another process could grab it), so it is only used where a
// false "in use" result would still keep the assertion meaningful.
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

// mustContain asserts an exact literal appears in the captured output. Doctor
// output is a user-facing contract; the literals are the thing under test.
func mustContain(t *testing.T, out, want string) {
	t.Helper()
	if !strings.Contains(out, want) {
		t.Errorf("output missing %q\n--- full output ---\n%s", want, out)
	}
}

func mustNotContain(t *testing.T, out, notWant string) {
	t.Helper()
	if strings.Contains(out, notWant) {
		t.Errorf("output unexpectedly contains %q\n--- full output ---\n%s", notWant, out)
	}
}

// findCheck returns the named DoctorCheck, failing the test if absent.
func findCheck(t *testing.T, checks []DoctorCheck, name string) DoctorCheck {
	t.Helper()
	for _, c := range checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("check %q not found in %+v", name, checks)
	return DoctorCheck{}
}

// ---------------------------------------------------------------------------
// IsLocalPortAvailable / SuggestAvailablePort
// ---------------------------------------------------------------------------

func TestIsLocalPortAvailable_ReportsFalseWhilePortIsHeld(t *testing.T) {
	port := occupiedPort(t)
	if IsLocalPortAvailable(port) {
		t.Errorf("port %d is held by this test but was reported available", port)
	}
}

func TestIsLocalPortAvailable_ReportsTrueForReleasedPort(t *testing.T) {
	port := freePort(t)
	if !IsLocalPortAvailable(port) {
		t.Errorf("port %d was released but was reported unavailable", port)
	}
}

func TestSuggestAvailablePort_SkipsHeldPortAndReturnsNext(t *testing.T) {
	held := occupiedPort(t)
	// Searching from the held port must not suggest the held port itself.
	got, ok := SuggestAvailablePort(held, 25)
	if !ok {
		t.Fatalf("expected a free port within 25 of %d", held)
	}
	if got == held {
		t.Errorf("suggested the held port %d", held)
	}
	if got < held || got > held+25 {
		t.Errorf("suggestion %d outside search window [%d,%d]", got, held, held+25)
	}
}

func TestSuggestAvailablePort_ZeroOffsetOnHeldPortFails(t *testing.T) {
	held := occupiedPort(t)
	// maxOffset=0 means "only try startPort" — held, so no suggestion exists.
	got, ok := SuggestAvailablePort(held, 0)
	if ok {
		t.Errorf("expected no suggestion, got %d", got)
	}
	if got != 0 {
		t.Errorf("failed suggestion must return port 0, got %d", got)
	}
}

func TestSuggestAvailablePort_SkipsNonPositiveCandidates(t *testing.T) {
	// Candidates -5..-2 are all <= 0 and must be skipped rather than passed to
	// net.Listen (which would produce a confusing "invalid port" error).
	got, ok := SuggestAvailablePort(-5, 3)
	if ok {
		t.Errorf("expected no suggestion from an all-negative range, got %d", got)
	}
	if got != 0 {
		t.Errorf("want port 0, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// CheckPortAvailability
// ---------------------------------------------------------------------------

func TestCheckPortAvailability_ReportsConflictWithRemediation(t *testing.T) {
	port := occupiedPort(t)
	hintCalledWith := -1
	hint := func(p int) string {
		hintCalledWith = p
		return "lsof -ti:" + fmt.Sprint(p) + " | xargs kill"
	}

	out := captureStdout(t, func() { CheckPortAvailability(port, hint) })

	if hintCalledWith != port {
		t.Errorf("PortKillHint called with %d, want %d", hintCalledWith, port)
	}
	mustContain(t, out, "Checking port availability... FAILED")
	mustContain(t, out, fmt.Sprintf("  Port %d is already in use.", port))
	mustContain(t, out, fmt.Sprintf("  Fix: lsof -ti:%d | xargs kill", port))
	mustContain(t, out, fmt.Sprintf("  Quick stop (Kaboom): kaboom --stop --port %d", port))
	// A conflict must offer an alternative port, not just complain.
	mustContain(t, out, "  Suggested free port: --port ")
	mustNotContain(t, out, "is available.")
}

func TestCheckPortAvailability_FallsBackWhenNoFreePortCanBeSuggested(t *testing.T) {
	// 65535 is the last valid TCP port, so every candidate the suggester tries
	// (65536..65560) is out of range and it must give up. The check then has
	// to fall back to the naive "+1" hint rather than printing nothing.
	const port = 65535
	if ln, err := net.Listen("tcp", "127.0.0.1:65535"); err == nil {
		defer func() { _ = ln.Close() }()
	}
	// Either this test holds 65535 or something else does; either way the
	// availability probe below must report a conflict.

	out := captureStdout(t, func() {
		CheckPortAvailability(port, func(int) string { return "kill-it" })
	})

	mustContain(t, out, "Checking port availability... FAILED")
	mustContain(t, out, "  Or use a different port: --port 65536")
	mustNotContain(t, out, "Suggested free port")
}

func TestCheckPortAvailability_ReportsOKForFreePort(t *testing.T) {
	port := freePort(t)
	out := captureStdout(t, func() {
		CheckPortAvailability(port, func(int) string {
			t.Error("PortKillHint must not be consulted when the port is free")
			return ""
		})
	})
	mustContain(t, out, "Checking port availability... OK")
	mustContain(t, out, fmt.Sprintf("  Port %d is available.", port))
	mustNotContain(t, out, "already in use")
}

// ---------------------------------------------------------------------------
// CheckStateDirectory
// ---------------------------------------------------------------------------

func TestCheckStateDirectory_ReportsResolvedRootAndLogPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(state.StateDirEnv, dir)

	out := captureStdout(t, CheckStateDirectory)

	mustContain(t, out, "Checking runtime state directory... OK")
	mustContain(t, out, "  State dir: "+dir)
	// Log path is derived from the root; a regression that stopped honouring
	// KABOOM_STATE_DIR for logs would show up here.
	mustContain(t, out, "  Log file: "+filepath.Join(dir, "logs", "kaboom.jsonl"))
}

func TestCheckStateDirectory_ReportsFailureWhenRootCannotBeResolved(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("home resolution on Windows uses USERPROFILE, not HOME")
	}
	t.Setenv(state.StateDirEnv, "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "")

	out := captureStdout(t, CheckStateDirectory)

	mustContain(t, out, "Checking runtime state directory... FAILED")
	mustContain(t, out, "  Cannot determine runtime state directory: ")
	mustNotContain(t, out, "State dir:")
}

// ---------------------------------------------------------------------------
// RunSetupCheckWithOptions
// ---------------------------------------------------------------------------

func writeTelemetryLog(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fastpath.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("write telemetry log: %v", err)
	}
	return path
}

func fastPathLine(success bool, method string, errorCode int) string {
	return fmt.Sprintf(`{"event":"bridge_fastpath_method","success":%t,"method":%q,"error_code":%d}`,
		success, method, errorCode)
}

func TestRunSetupCheck_DefaultOptionsSkipThresholdAndPass(t *testing.T) {
	t.Setenv(state.StateDirEnv, t.TempDir())
	missing := filepath.Join(t.TempDir(), "never-written.jsonl")

	var ok bool
	out := captureStdout(t, func() {
		ok = RunSetupCheckWithOptions(freePort(t), SetupCheckOptions{}, SetupDeps{
			Version:                  "9.9.9-test",
			PortKillHint:             func(int) string { return "kill-hint" },
			FastPathTelemetryLogPath: func() (string, error) { return missing, nil },
		})
	})

	if !ok {
		t.Error("zero-value options must disable the threshold gate and return true")
	}
	mustContain(t, out, "KABOOM SETUP CHECK")
	mustContain(t, out, "Version: 9.9.9-test")
	// MaxFailureRatio defaults to -1 when both knobs are zero, so the
	// threshold section must be skipped entirely.
	mustNotContain(t, out, "Checking fast-path failure threshold...")
	mustContain(t, out, "  1. Start server:    npx kaboom-agentic-browser")
}

func TestRunSetupCheck_ReportsFailureRatioBreach(t *testing.T) {
	t.Setenv(state.StateDirEnv, t.TempDir())
	lines := []string{
		fastPathLine(true, "attach", 0),
		fastPathLine(false, "attach", 503),
		fastPathLine(false, "attach", 503),
		fastPathLine(false, "attach", 500),
	}
	path := writeTelemetryLog(t, lines...)

	var ok bool
	out := captureStdout(t, func() {
		ok = RunSetupCheckWithOptions(freePort(t), SetupCheckOptions{
			MinSamples:      4,
			MaxFailureRatio: 0.1,
		}, SetupDeps{
			Version:                  "0.0.1",
			PortKillHint:             func(int) string { return "kill-hint" },
			FastPathTelemetryLogPath: func() (string, error) { return path, nil },
		})
	})

	if ok {
		t.Error("3/4 failures against a 0.1 threshold must fail the setup check")
	}
	mustContain(t, out, "Checking fast-path failure threshold... FAILED")
	mustContain(t, out, "  failure ratio 0.7500 exceeds threshold 0.1000 (3/4 failures)")
	// The telemetry section must still have reported the breakdown.
	mustContain(t, out, "  Last 200 events: total=4 success=1 failure=3")
	mustContain(t, out, "  Error codes: 500=1, 503=2")
}

func TestRunSetupCheck_ReportsInsufficientSamples(t *testing.T) {
	t.Setenv(state.StateDirEnv, t.TempDir())
	path := writeTelemetryLog(t, fastPathLine(true, "attach", 0))

	var ok bool
	out := captureStdout(t, func() {
		ok = RunSetupCheckWithOptions(freePort(t), SetupCheckOptions{MaxFailureRatio: 0.5}, SetupDeps{
			Version:                  "0.0.1",
			PortKillHint:             func(int) string { return "kill-hint" },
			FastPathTelemetryLogPath: func() (string, error) { return path, nil },
		})
	})

	if ok {
		t.Error("1 sample against the default MinSamples=50 must fail")
	}
	// MinSamples defaults to 50 when left at zero.
	mustContain(t, out, "  insufficient samples: got 1, need 50")
}

func TestRunSetupCheck_PassesWhenRatioWithinThreshold(t *testing.T) {
	t.Setenv(state.StateDirEnv, t.TempDir())
	lines := make([]string, 0, 10)
	for i := 0; i < 9; i++ {
		lines = append(lines, fastPathLine(true, "attach", 0))
	}
	lines = append(lines, fastPathLine(false, "attach", 500))
	path := writeTelemetryLog(t, lines...)

	var ok bool
	out := captureStdout(t, func() {
		ok = RunSetupCheckWithOptions(freePort(t), SetupCheckOptions{
			MinSamples:      10,
			MaxFailureRatio: 0.2,
		}, SetupDeps{
			Version:                  "0.0.1",
			PortKillHint:             func(int) string { return "kill-hint" },
			FastPathTelemetryLogPath: func() (string, error) { return path, nil },
		})
	})

	if !ok {
		t.Error("1/10 failures against a 0.2 threshold must pass")
	}
	mustContain(t, out, "Checking fast-path failure threshold... OK")
	mustContain(t, out, "  Ratio 0.1000 within threshold 0.2000 (samples=10)")
}

// ---------------------------------------------------------------------------
// PrintFastPathTelemetryDiagnostics
// ---------------------------------------------------------------------------

func TestPrintFastPathTelemetry_ReportsUnresolvablePath(t *testing.T) {
	var summary FastPathTelemetrySummary
	var ok bool
	out := captureStdout(t, func() {
		summary, ok = PrintFastPathTelemetryDiagnostics(200, func() (string, error) {
			return "", fmt.Errorf("state dir unavailable")
		})
	})

	if ok {
		t.Error("unresolvable path must report ok=false")
	}
	// Maps must be non-nil so callers can index them without a nil check.
	if summary.ErrorCodes == nil || summary.Methods == nil {
		t.Errorf("summary maps must be initialised even on failure: %+v", summary)
	}
	mustContain(t, out, "Checking bridge fast-path telemetry... FAILED")
	mustContain(t, out, "  Cannot resolve telemetry log path: state dir unavailable")
}

func TestPrintFastPathTelemetry_ReportsNoTelemetryRecordedYet(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.jsonl")

	var summary FastPathTelemetrySummary
	var ok bool
	out := captureStdout(t, func() {
		summary, ok = PrintFastPathTelemetryDiagnostics(200, func() (string, error) { return missing, nil })
	})

	// A missing log is normal on a fresh install: OK status but ok=false so the
	// caller knows there is nothing to threshold against.
	if ok {
		t.Error("missing log must report ok=false")
	}
	if summary.Total != 0 {
		t.Errorf("Total: want 0, got %d", summary.Total)
	}
	mustContain(t, out, "Checking bridge fast-path telemetry... OK")
	mustContain(t, out, "  Status: no fast-path telemetry recorded yet")
	mustNotContain(t, out, "FAILED")
}

func TestPrintFastPathTelemetry_ReportsUnreadableLogPath(t *testing.T) {
	// A regular file used as a directory component yields ENOTDIR from Stat —
	// deterministic on every unix, and unlike chmod 0000 it also fails as root.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	bad := filepath.Join(blocker, "fastpath.jsonl")

	var ok bool
	out := captureStdout(t, func() {
		_, ok = PrintFastPathTelemetryDiagnostics(200, func() (string, error) { return bad, nil })
	})

	if ok {
		t.Error("unreadable log must report ok=false")
	}
	mustContain(t, out, "Checking bridge fast-path telemetry... FAILED")
	mustContain(t, out, "  Telemetry log read error: ")
}

func TestPrintFastPathTelemetry_PrintsSortedMethodsAndErrorCodes(t *testing.T) {
	path := writeTelemetryLog(t,
		fastPathLine(false, "zeta", 503),
		fastPathLine(false, "alpha", 500),
		fastPathLine(true, "alpha", 0),
	)

	var summary FastPathTelemetrySummary
	var ok bool
	out := captureStdout(t, func() {
		summary, ok = PrintFastPathTelemetryDiagnostics(50, func() (string, error) { return path, nil })
	})

	if !ok {
		t.Fatal("readable log must report ok=true")
	}
	if summary.Total != 3 || summary.Success != 1 || summary.Failure != 2 {
		t.Errorf("summary: want total=3 success=1 failure=2, got %+v", summary)
	}
	mustContain(t, out, "  Last 50 events: total=3 success=1 failure=2")
	// Sorted output keeps doctor runs diffable; map iteration order would not be.
	mustContain(t, out, "  Methods: alpha=2, zeta=1")
	mustContain(t, out, "  Error codes: 500=1, 503=1")
}

func TestPrintFastPathTelemetry_PrintsNoneWhenNoErrorCodes(t *testing.T) {
	path := writeTelemetryLog(t, fastPathLine(true, "attach", 0))

	out := captureStdout(t, func() {
		_, _ = PrintFastPathTelemetryDiagnostics(50, func() (string, error) { return path, nil })
	})

	// error_code 0 is explicitly not counted, so a clean log prints "none".
	mustContain(t, out, "  Error codes: none")
}

// ---------------------------------------------------------------------------
// SummarizeFastPathTelemetryLog — parsing edge cases
// ---------------------------------------------------------------------------

func TestSummarizeFastPathTelemetryLog_KeepsOnlyLastMaxLines(t *testing.T) {
	path := writeTelemetryLog(t,
		fastPathLine(true, "old", 0),
		fastPathLine(true, "old", 0),
		fastPathLine(true, "old", 0),
		fastPathLine(false, "recent", 500),
		fastPathLine(false, "recent", 500),
	)

	// maxLines is a tail window: the three older successes must be dropped.
	s := SummarizeFastPathTelemetryLog(path, 2)
	if s.Total != 2 {
		t.Errorf("Total: want 2, got %d", s.Total)
	}
	if s.Success != 0 || s.Failure != 2 {
		t.Errorf("want success=0 failure=2, got success=%d failure=%d", s.Success, s.Failure)
	}
	if s.Methods["old"] != 0 {
		t.Errorf("older lines must be evicted, got Methods[old]=%d", s.Methods["old"])
	}
	if s.Methods["recent"] != 2 {
		t.Errorf("Methods[recent]: want 2, got %d", s.Methods["recent"])
	}
}

func TestSummarizeFastPathTelemetryLog_IgnoresGarbageAndForeignEvents(t *testing.T) {
	path := writeTelemetryLog(t,
		"",
		"   ",
		"not json at all",
		`{"event":"something_else","success":false}`,
		`{"event":"bridge_fastpath_method","success":true,"method":"attach"}`,
		`{"event":"bridge_fastpath_method","success":"yes","method":"attach"}`, // non-bool success => failure
		`{"event":"bridge_fastpath_method","success":false,"error_code":0}`,    // code 0 not recorded
	)

	s := SummarizeFastPathTelemetryLog(path, 200)
	if s.Total != 3 {
		t.Errorf("Total: want 3 (only bridge_fastpath_method rows), got %d", s.Total)
	}
	if s.Success != 1 {
		t.Errorf("Success: want 1, got %d", s.Success)
	}
	// A non-boolean "success" field counts as a failure, not a parse skip.
	if s.Failure != 2 {
		t.Errorf("Failure: want 2, got %d", s.Failure)
	}
	if len(s.ErrorCodes) != 0 {
		t.Errorf("error_code 0 must not be recorded, got %+v", s.ErrorCodes)
	}
	if s.Methods["attach"] != 2 {
		t.Errorf("Methods[attach]: want 2, got %d", s.Methods["attach"])
	}
}

func TestSummarizeFastPathTelemetryLog_NegativeMaxLinesReturnsInitialisedMaps(t *testing.T) {
	s := SummarizeFastPathTelemetryLog(writeTelemetryLog(t, fastPathLine(true, "attach", 0)), -1)
	if s.Total != 0 {
		t.Errorf("Total: want 0, got %d", s.Total)
	}
	if s.ErrorCodes == nil || s.Methods == nil {
		t.Error("maps must be initialised even for a rejected maxLines")
	}
}

// ---------------------------------------------------------------------------
// RunDoctorChecks — the broken-system reports
// ---------------------------------------------------------------------------

func TestRunDoctorChecks_FreshCaptureReportsExtensionDisconnectedWithFix(t *testing.T) {
	cap := capture.NewCapture()
	checks := RunDoctorChecks(cap)

	if len(checks) != 6 {
		t.Fatalf("want 6 checks, got %d: %+v", len(checks), checks)
	}
	want := []string{
		"extension_connected", "pilot_enabled", "tracked_tab",
		"circuit_breaker", "command_queue", "command_execution",
	}
	for i, name := range want {
		if checks[i].Name != name {
			t.Errorf("checks[%d].Name = %q, want %q", i, checks[i].Name, name)
		}
	}

	ext := findCheck(t, checks, "extension_connected")
	if ext.Status != "fail" {
		t.Errorf("extension_connected status: want fail, got %q", ext.Status)
	}
	if ext.Detail != "Extension is not connected" {
		t.Errorf("extension_connected detail: got %q", ext.Detail)
	}
	// Remediation text is the whole point of doctor — pin it verbatim.
	if ext.Fix != "Open the Kaboom extension popup and verify it shows 'Connected'. If not, click the extension icon or reload the page." {
		t.Errorf("extension_connected fix: got %q", ext.Fix)
	}
}

func TestRunDoctorChecks_UnsyncedPilotWarnsAsAssumedEnabled(t *testing.T) {
	cap := capture.NewCapture()
	cap.SetPilotUnknownForTest()

	pilot := findCheck(t, RunDoctorChecks(cap), "pilot_enabled")
	if pilot.Status != "warn" {
		t.Errorf("status: want warn, got %q", pilot.Status)
	}
	if pilot.Detail != "AI Web Pilot status not yet confirmed; assuming enabled until first sync" {
		t.Errorf("detail: got %q", pilot.Detail)
	}
	if pilot.Fix != "Open the extension once to confirm pilot settings, then rerun doctor" {
		t.Errorf("fix: got %q", pilot.Fix)
	}
}

func TestRunDoctorChecks_ExplicitlyDisabledPilotWarnsWithDistinctDetail(t *testing.T) {
	cap := capture.NewCapture()
	cap.SetPilotEnabled(false)

	pilot := findCheck(t, RunDoctorChecks(cap), "pilot_enabled")
	if pilot.Status != "warn" {
		t.Errorf("status: want warn, got %q", pilot.Status)
	}
	// Distinct from the "not yet confirmed" wording above: an explicit disable
	// is actionable, an unconfirmed one is not.
	if pilot.Detail != "AI Web Pilot is explicitly disabled — interact actions will fail" {
		t.Errorf("detail: got %q", pilot.Detail)
	}
	if pilot.Fix != "Enable AI Web Pilot in the extension popup" {
		t.Errorf("fix: got %q", pilot.Fix)
	}
}

// Every pilot state the capture layer can report must map to exactly one
// doctor check, and each must say something different. There used to be a
// second, unreachable "AI Web Pilot is disabled" arm duplicating the
// explicitly-disabled wording; a duplicate string in a diagnostics surface is a
// maintenance hazard, since only one copy ever gets updated.
func TestRunDoctorChecks_EveryPilotStateHasItsOwnDistinctWording(t *testing.T) {
	details := map[string]string{}
	for name, setup := range map[string]func(*capture.Capture){
		capture.PilotStateEnabled:            func(c *capture.Capture) { c.SetPilotEnabled(true) },
		capture.PilotStateExplicitlyDisabled: func(c *capture.Capture) { c.SetPilotEnabled(false) },
		capture.PilotStateAssumedEnabled:     func(c *capture.Capture) { c.SetPilotUnknownForTest() },
	} {
		cap := capture.NewCapture()
		setup(cap)
		pilot := findCheck(t, RunDoctorChecks(cap), "pilot_enabled")
		if pilot.Detail == "" {
			t.Errorf("%s: check has no detail", name)
		}
		if prev, dup := details[pilot.Detail]; dup {
			t.Errorf("states %q and %q report identical wording %q", prev, name, pilot.Detail)
		}
		details[pilot.Detail] = name
	}
	if len(details) != 3 {
		t.Errorf("got %d distinct details across 3 pilot states: %v", len(details), details)
	}
}

func TestRunDoctorChecks_EnabledPilotPassesWithNoFix(t *testing.T) {
	cap := capture.NewCapture()
	cap.SetPilotEnabled(true)

	pilot := findCheck(t, RunDoctorChecks(cap), "pilot_enabled")
	if pilot.Status != "pass" {
		t.Errorf("status: want pass, got %q", pilot.Status)
	}
	if pilot.Detail != "AI Web Pilot is enabled" {
		t.Errorf("detail: got %q", pilot.Detail)
	}
	if pilot.Fix != "" {
		t.Errorf("a passing check must not carry a fix, got %q", pilot.Fix)
	}
}

func TestRunDoctorChecks_NoTrackedTabWarnsWithNavigationHint(t *testing.T) {
	tab := findCheck(t, RunDoctorChecks(capture.NewCapture()), "tracked_tab")
	if tab.Status != "warn" {
		t.Errorf("status: want warn, got %q", tab.Status)
	}
	if tab.Detail != "No tab is being tracked — observe and interact may return empty results" {
		t.Errorf("detail: got %q", tab.Detail)
	}
	if tab.Fix != "Navigate to a page in Chrome. The extension auto-tracks the active tab." {
		t.Errorf("fix: got %q", tab.Fix)
	}
}

func TestRunDoctorChecks_TrackedTabPassesWithIDAndURL(t *testing.T) {
	cap := capture.NewCapture()
	cap.SetTrackingStatusForTest(42, "https://example.test/page")

	tab := findCheck(t, RunDoctorChecks(cap), "tracked_tab")
	if tab.Status != "pass" {
		t.Errorf("status: want pass, got %q", tab.Status)
	}
	if tab.Detail != "Tracking tab 42: https://example.test/page" {
		t.Errorf("detail: got %q", tab.Detail)
	}
}

func TestRunDoctorChecks_ConnectedExtensionWithoutPollReportsLastSeenUnknown(t *testing.T) {
	cap := capture.NewCapture()
	cap.SimulateExtensionConnectForTest()

	ext := findCheck(t, RunDoctorChecks(cap), "extension_connected")
	if ext.Status != "pass" {
		t.Errorf("status: want pass, got %q", ext.Status)
	}
	// lastSyncSeen (connectivity) and lastPollAt (freshness) are separate
	// fields; a sync without a poll must degrade to "unknown", not crash or
	// print a bogus 1970 duration.
	if ext.Detail != "Extension connected (last seen: unknown)" {
		t.Errorf("detail: got %q", ext.Detail)
	}
}

func TestRunDoctorChecks_PolledExtensionReportsLastSeenAge(t *testing.T) {
	cap := capture.NewCapture()
	cap.SimulateSyncForTest("ext-session-1", "client-1")

	ext := findCheck(t, RunDoctorChecks(cap), "extension_connected")
	if ext.Status != "pass" {
		t.Errorf("status: want pass, got %q", ext.Status)
	}
	// A real /sync stamps lastPollAt, so the freshness age must be rendered
	// instead of the "unknown" fallback.
	if !strings.HasPrefix(ext.Detail, "Extension connected (last seen: ") ||
		!strings.HasSuffix(ext.Detail, "s ago)") {
		t.Errorf("detail: got %q", ext.Detail)
	}
	if strings.Contains(ext.Detail, "unknown") {
		t.Errorf("detail must report an age once the extension has polled: %q", ext.Detail)
	}
}

func TestRunDoctorChecks_StaleSyncReportsDisconnected(t *testing.T) {
	cap := capture.NewCapture()
	cap.SimulateExtensionConnectForTest()
	cap.SimulateExtensionDisconnectForTest() // pushes lastSyncSeen 1h into the past

	ext := findCheck(t, RunDoctorChecks(cap), "extension_connected")
	if ext.Status != "fail" {
		t.Errorf("a stale sync must read as disconnected, got status %q", ext.Status)
	}
}

func TestRunDoctorChecks_EmptyQueuePassesWithEmptyWording(t *testing.T) {
	q := findCheck(t, RunDoctorChecks(capture.NewCapture()), "command_queue")
	if q.Status != "pass" {
		t.Errorf("status: want pass, got %q", q.Status)
	}
	if q.Detail != "Command queue empty" {
		t.Errorf("detail: got %q", q.Detail)
	}
}

func TestRunDoctorChecks_ShallowQueuePassesWithCount(t *testing.T) {
	cap := capture.NewCapture()
	enqueue(t, cap, 3)

	q := findCheck(t, RunDoctorChecks(cap), "command_queue")
	if q.Status != "pass" {
		t.Errorf("3 pending commands is below the warn threshold, got status %q", q.Status)
	}
	if q.Detail != "Command queue: 3 pending" {
		t.Errorf("detail: got %q", q.Detail)
	}
}

func TestRunDoctorChecks_BackedUpQueueWarnsAtFive(t *testing.T) {
	cap := capture.NewCapture()
	enqueue(t, cap, 5)

	q := findCheck(t, RunDoctorChecks(cap), "command_queue")
	// 5 is the exact boundary: < 5 passes, >= 5 warns.
	if q.Status != "warn" {
		t.Errorf("status: want warn at depth 5, got %q", q.Status)
	}
	if q.Detail != "Command queue has 5 pending commands — extension may be falling behind" {
		t.Errorf("detail: got %q", q.Detail)
	}
	if q.Fix != "Wait for commands to complete, or check extension connectivity." {
		t.Errorf("fix: got %q", q.Fix)
	}
}

func TestRunDoctorChecks_CommandExecutionFailureAttachesFix(t *testing.T) {
	cap := capture.NewCapture()
	failCommands(t, cap, "timeout", 3)

	exec := findCheck(t, RunDoctorChecks(cap), "command_execution")
	if exec.Status != "fail" {
		t.Errorf("3 recent failures must fail the check, got %q", exec.Status)
	}
	if exec.Fix != `Inspect observe(what:"failed_commands") for recent expiry/timeout/error events and verify extension polling (/sync). If degradation persists, reload the extension or run configure(action:"restart").` {
		t.Errorf("fix: got %q", exec.Fix)
	}
}

func TestRunDoctorChecks_HealthyCommandExecutionHasNoFix(t *testing.T) {
	exec := findCheck(t, RunDoctorChecks(capture.NewCapture()), "command_execution")
	if exec.Status != "pass" {
		t.Errorf("status: want pass, got %q", exec.Status)
	}
	if exec.Fix != "" {
		t.Errorf("passing command_execution must carry no fix, got %q", exec.Fix)
	}
}

// enqueue registers n pending queries so QueueDepth reports n.
func enqueue(t *testing.T, cap *capture.Store, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if _, err := cap.CreatePendingQuery(newPendingQuery(fmt.Sprintf("q%d", i))); err != nil {
			t.Fatalf("CreatePendingQuery: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// HandleDoctorHTTP
// ---------------------------------------------------------------------------

type doctorHTTPBody struct {
	Status              string        `json:"status"`
	ReadyForInteraction bool          `json:"ready_for_interaction"`
	Version             string        `json:"version"`
	Checks              []DoctorCheck `json:"checks"`
}

func decodeDoctorHTTP(t *testing.T, cap *capture.Store, ver string) (*httptest.ResponseRecorder, doctorHTTPBody) {
	t.Helper()
	rec := httptest.NewRecorder()
	HandleDoctorHTTP(rec, cap, ver)
	var body doctorHTTPBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode doctor body %q: %v", rec.Body.String(), err)
	}
	return rec, body
}

func TestHandleDoctorHTTP_UnhealthyWhenExtensionDisconnected(t *testing.T) {
	rec, body := decodeDoctorHTTP(t, capture.NewCapture(), "1.2.3")

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type: want application/json, got %q", got)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status code: want 200 even when unhealthy, got %d", rec.Code)
	}
	if body.Status != "unhealthy" {
		t.Errorf("status: want unhealthy, got %q", body.Status)
	}
	if body.ReadyForInteraction {
		t.Error("ready_for_interaction must be false when a check fails")
	}
	if body.Version != "1.2.3" {
		t.Errorf("version: want 1.2.3, got %q", body.Version)
	}
	if len(body.Checks) != 6 {
		t.Errorf("want 6 checks in the payload, got %d", len(body.Checks))
	}
}

func TestHandleDoctorHTTP_DegradedWhenOnlyWarnings(t *testing.T) {
	cap := capture.NewCapture()
	cap.SimulateExtensionConnectForTest()
	cap.SetPilotEnabled(true)
	// tracked_tab is left unset => warn, no fails.

	_, body := decodeDoctorHTTP(t, cap, "1.2.3")

	if body.Status != "degraded" {
		t.Errorf("warn-only must be degraded, got %q", body.Status)
	}
	// A degraded server is still not ready — this is deliberately stricter
	// than "not unhealthy".
	if body.ReadyForInteraction {
		t.Error("ready_for_interaction must be false while degraded")
	}
}

func TestHandleDoctorHTTP_HealthyWhenEveryCheckPasses(t *testing.T) {
	cap := capture.NewCapture()
	cap.SimulateExtensionConnectForTest()
	cap.SetPilotEnabled(true)
	cap.SetTrackingStatusForTest(7, "https://example.test/")

	_, body := decodeDoctorHTTP(t, cap, "1.2.3")

	if body.Status != "healthy" {
		t.Fatalf("want healthy, got %q (checks=%+v)", body.Status, body.Checks)
	}
	if !body.ReadyForInteraction {
		t.Error("ready_for_interaction must be true when all checks pass")
	}
}

func TestHandleDoctorHTTP_FailOutranksWarn(t *testing.T) {
	cap := capture.NewCapture()
	// Extension disconnected (fail) plus no tracked tab (warn): the fail must
	// win regardless of check ordering.
	_, body := decodeDoctorHTTP(t, cap, "v")

	if body.Status != "unhealthy" {
		t.Errorf("want unhealthy, got %q", body.Status)
	}
}

// ---------------------------------------------------------------------------
// BuildCommandExecutionInfo / BuildCommandExecutionInfoAt
// ---------------------------------------------------------------------------

// newPendingQuery builds a queue entry with no correlation ID, so it moves
// QueueDepth without also creating a command lifecycle record.
func newPendingQuery(label string) queries.PendingQuery {
	return queries.PendingQuery{Type: "dom_query", Params: []byte(`{"label":"` + label + `"}`)}
}

// failCommands registers n commands and drives them to the given terminal
// failure status.
func failCommands(t *testing.T, cap *capture.Store, status string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("%s-%d", status, i)
		cap.RegisterCommand(id, "q-"+id, 10*time.Minute)
		cap.ApplyCommandResult(id, status, nil, "synthetic "+status)
	}
}

// completeCommands registers n commands and drives them to "complete".
func completeCommands(t *testing.T, cap *capture.Store, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("done-%d", i)
		cap.RegisterCommand(id, "q-"+id, 10*time.Minute)
		cap.ApplyCommandResult(id, "complete", nil, "")
	}
}

func TestBuildCommandExecutionInfo_NilCaptureIsNotReady(t *testing.T) {
	info := BuildCommandExecutionInfo(nil)
	if info.Ready {
		t.Error("nil capture must not be ready")
	}
	if info.Status != "fail" {
		t.Errorf("status: want fail, got %q", info.Status)
	}
	if info.Detail != "Capture not initialized" {
		t.Errorf("detail: got %q", info.Detail)
	}
	if info.WindowSeconds != 300 {
		t.Errorf("WindowSeconds must still be populated, got %d", info.WindowSeconds)
	}
}

func TestBuildCommandExecutionInfo_IdleCaptureIsReady(t *testing.T) {
	info := BuildCommandExecutionInfo(capture.NewCapture())
	if !info.Ready || info.Status != "pass" {
		t.Errorf("idle capture: want ready/pass, got ready=%v status=%q", info.Ready, info.Status)
	}
	if info.Detail != "window=300s; no recent command failures" {
		t.Errorf("detail: got %q", info.Detail)
	}
	if info.RecentFailureRatePct != 0 {
		t.Errorf("failure rate with no attempts must be 0, got %f", info.RecentFailureRatePct)
	}
	if info.LastSuccessAt != "" {
		t.Errorf("LastSuccessAt must be empty with no completions, got %q", info.LastSuccessAt)
	}
}

func TestBuildCommandExecutionInfo_SingleFailureWarns(t *testing.T) {
	cap := capture.NewCapture()
	failCommands(t, cap, "error", 1)

	info := BuildCommandExecutionInfoAt(cap, time.Now())
	if info.Status != "warn" {
		t.Errorf("1 failure must warn, got %q", info.Status)
	}
	if info.Ready {
		t.Error("Ready must be false for any non-pass status")
	}
	if info.RecentFailedCount != 1 || info.RecentErrorCount != 1 {
		t.Errorf("want failed=1 error=1, got %+v", info)
	}
	if info.Detail != "window=300s; recent failures=1/1 (100.0%): expired=0 timeout=0 error=1 cancelled=0" {
		t.Errorf("detail: got %q", info.Detail)
	}
}

func TestBuildCommandExecutionInfo_ThreeFailuresFail(t *testing.T) {
	cap := capture.NewCapture()
	failCommands(t, cap, "expired", 3)

	info := BuildCommandExecutionInfoAt(cap, time.Now())
	// 3 is the exact fail threshold; 2 only warns (pinned below).
	if info.Status != "fail" {
		t.Errorf("3 failures must fail, got %q", info.Status)
	}
	if info.RecentExpiredCount != 3 {
		t.Errorf("RecentExpiredCount: want 3, got %d", info.RecentExpiredCount)
	}
	if info.Detail != "window=300s; recent failures=3/3 (100.0%): expired=3 timeout=0 error=0 cancelled=0" {
		t.Errorf("detail: got %q", info.Detail)
	}
}

func TestBuildCommandExecutionInfo_TwoFailuresStillOnlyWarn(t *testing.T) {
	cap := capture.NewCapture()
	failCommands(t, cap, "cancelled", 2)

	info := BuildCommandExecutionInfoAt(cap, time.Now())
	if info.Status != "warn" {
		t.Errorf("2 failures is below the fail threshold, got %q", info.Status)
	}
	if info.RecentCancelledCount != 2 {
		t.Errorf("RecentCancelledCount: want 2, got %d", info.RecentCancelledCount)
	}
}

func TestBuildCommandExecutionInfo_ClassifiesEachFailureKind(t *testing.T) {
	cap := capture.NewCapture()
	failCommands(t, cap, "expired", 1)
	failCommands(t, cap, "timeout", 1)
	failCommands(t, cap, "error", 1)
	failCommands(t, cap, "cancelled", 1)
	completeCommands(t, cap, 4)

	info := BuildCommandExecutionInfoAt(cap, time.Now())
	if info.RecentExpiredCount != 1 || info.RecentTimeoutCount != 1 ||
		info.RecentErrorCount != 1 || info.RecentCancelledCount != 1 {
		t.Errorf("per-kind counters wrong: %+v", info)
	}
	if info.RecentFailedCount != 4 {
		t.Errorf("RecentFailedCount: want 4, got %d", info.RecentFailedCount)
	}
	if info.RecentSuccessCount != 4 {
		t.Errorf("RecentSuccessCount: want 4, got %d", info.RecentSuccessCount)
	}
	// 4 failed of 8 attempts.
	if info.RecentFailureRatePct != 50 {
		t.Errorf("RecentFailureRatePct: want 50, got %f", info.RecentFailureRatePct)
	}
	if info.Detail != "window=300s; recent failures=4/8 (50.0%): expired=1 timeout=1 error=1 cancelled=1" {
		t.Errorf("detail: got %q", info.Detail)
	}
}

func TestBuildCommandExecutionInfo_IgnoresFailuresOlderThanWindow(t *testing.T) {
	cap := capture.NewCapture()
	failCommands(t, cap, "error", 5)

	// Evaluate 6 minutes after the failures: past the 5-minute window, so the
	// server must report healthy again rather than latching on old errors.
	info := BuildCommandExecutionInfoAt(cap, time.Now().Add(6*time.Minute))
	if info.RecentFailedCount != 0 {
		t.Errorf("out-of-window failures must not count, got %d", info.RecentFailedCount)
	}
	if info.Status != "pass" {
		t.Errorf("status after the window expires: want pass, got %q", info.Status)
	}
	if info.Detail != "window=300s; no recent command failures" {
		t.Errorf("detail: got %q", info.Detail)
	}
}

func TestBuildCommandExecutionInfo_IgnoresFuturedatedEvents(t *testing.T) {
	cap := capture.NewCapture()
	failCommands(t, cap, "error", 3)

	// Clock skew: evaluating "before" the events must not count them.
	info := BuildCommandExecutionInfoAt(cap, time.Now().Add(-time.Minute))
	if info.RecentFailedCount != 0 {
		t.Errorf("future-dated failures must be skipped, got %d", info.RecentFailedCount)
	}
	if info.Status != "pass" {
		t.Errorf("status: want pass, got %q", info.Status)
	}
}

func TestBuildCommandExecutionInfo_ClampsNegativeAgesToZero(t *testing.T) {
	cap := capture.NewCapture()
	cap.RegisterCommand("future", "q-future", 30*time.Minute)
	completeCommands(t, cap, 1)

	// Backwards clock: ages computed against an earlier "now" would be
	// negative, which would render as "-60.0s ago" and as a negative
	// oldest_pending_age_ms on the wire.
	info := BuildCommandExecutionInfoAt(cap, time.Now().Add(-time.Minute))

	if info.PendingCount != 1 {
		t.Fatalf("PendingCount: want 1, got %d", info.PendingCount)
	}
	if info.OldestPendingAgeMs != 0 {
		t.Errorf("OldestPendingAgeMs must clamp to 0, got %d", info.OldestPendingAgeMs)
	}
	if info.LastSuccessAt == "" {
		t.Fatal("LastSuccessAt must still be recorded for a future-dated completion")
	}
	if info.LastSuccessAgeMs != 0 {
		t.Errorf("LastSuccessAgeMs must clamp to 0, got %d", info.LastSuccessAgeMs)
	}
	// The success is outside the window (negative age), so it is not counted
	// as a recent success even though the timestamp is retained.
	if info.RecentSuccessCount != 0 {
		t.Errorf("RecentSuccessCount: want 0, got %d", info.RecentSuccessCount)
	}
}

func TestBuildCommandExecutionInfo_RetainsLastSuccessOutsideWindow(t *testing.T) {
	cap := capture.NewCapture()
	completeCommands(t, cap, 1)

	// 6 minutes later: outside the 5-minute counting window, but the "when did
	// anything last work" timestamp must survive — it is what distinguishes a
	// quiet daemon from a wedged one.
	info := BuildCommandExecutionInfoAt(cap, time.Now().Add(6*time.Minute))

	if info.RecentSuccessCount != 0 {
		t.Errorf("RecentSuccessCount: want 0 outside the window, got %d", info.RecentSuccessCount)
	}
	if info.LastSuccessAt == "" {
		t.Error("LastSuccessAt must be retained regardless of the window")
	}
	if info.LastSuccessAgeMs < 359_000 || info.LastSuccessAgeMs > 361_000 {
		t.Errorf("LastSuccessAgeMs: want ~360000, got %d", info.LastSuccessAgeMs)
	}
}

func TestBuildCommandExecutionInfo_RecordsLastSuccessTimestamp(t *testing.T) {
	cap := capture.NewCapture()
	completeCommands(t, cap, 1)

	now := time.Now().Add(30 * time.Second)
	info := BuildCommandExecutionInfoAt(cap, now)

	if info.RecentSuccessCount != 1 {
		t.Fatalf("RecentSuccessCount: want 1, got %d", info.RecentSuccessCount)
	}
	if info.LastSuccessAt == "" {
		t.Fatal("LastSuccessAt must be set after a completion")
	}
	parsed, err := time.Parse(time.RFC3339Nano, info.LastSuccessAt)
	if err != nil {
		t.Fatalf("LastSuccessAt %q is not RFC3339Nano: %v", info.LastSuccessAt, err)
	}
	if parsed.Location() != time.UTC {
		t.Errorf("LastSuccessAt must be UTC, got %v", parsed.Location())
	}
	// ~30s of synthetic elapsed time, allowing for real execution jitter.
	if info.LastSuccessAgeMs < 29_000 || info.LastSuccessAgeMs > 31_000 {
		t.Errorf("LastSuccessAgeMs: want ~30000, got %d", info.LastSuccessAgeMs)
	}
	if info.Status != "pass" {
		t.Errorf("a recent success with no failures must pass, got %q", info.Status)
	}
}

func TestBuildCommandExecutionInfo_PendingBacklogWarnsAfter45s(t *testing.T) {
	cap := capture.NewCapture()
	cap.RegisterCommand("stuck", "q-stuck", 30*time.Minute)

	info := BuildCommandExecutionInfoAt(cap, time.Now().Add(50*time.Second))

	if info.PendingCount != 1 {
		t.Fatalf("PendingCount: want 1, got %d", info.PendingCount)
	}
	if info.Status != "warn" {
		t.Errorf("a 50s-old pending command with no success must warn, got %q", info.Status)
	}
	if info.Ready {
		t.Error("Ready must be false while a stalled backlog is reported")
	}
	// Wording pinned; the elapsed float is checked separately so clock jitter
	// cannot flake the string compare.
	const wantPrefix = "window=300s; no recent command failures; pending backlog: 1 command(s), oldest="
	if !strings.HasPrefix(info.Detail, wantPrefix) {
		t.Errorf("detail prefix: got %q", info.Detail)
	}
	if !strings.HasSuffix(info.Detail, "s, last_success=none") {
		t.Errorf("detail must report last_success=none when nothing ever succeeded: %q", info.Detail)
	}
	if info.OldestPendingAgeMs < 49_000 || info.OldestPendingAgeMs > 51_000 {
		t.Errorf("OldestPendingAgeMs: want ~50000, got %d", info.OldestPendingAgeMs)
	}
}

func TestBuildCommandExecutionInfo_PendingBacklogFailsAfter2Minutes(t *testing.T) {
	cap := capture.NewCapture()
	cap.RegisterCommand("stuck", "q-stuck", 30*time.Minute)

	info := BuildCommandExecutionInfoAt(cap, time.Now().Add(3*time.Minute))
	if info.Status != "fail" {
		t.Errorf("a 3-minute-old pending command must fail, got %q", info.Status)
	}
}

func TestBuildCommandExecutionInfo_YoungBacklogDoesNotWarn(t *testing.T) {
	cap := capture.NewCapture()
	cap.RegisterCommand("fresh", "q-fresh", 30*time.Minute)

	info := BuildCommandExecutionInfoAt(cap, time.Now().Add(5*time.Second))
	if info.Status != "pass" {
		t.Errorf("a 5s-old pending command is normal, got %q", info.Status)
	}
	if strings.Contains(info.Detail, "pending backlog") {
		t.Errorf("detail must not mention a backlog: %q", info.Detail)
	}
}

func TestBuildCommandExecutionInfo_StalledBacklogReportsLastSuccessAge(t *testing.T) {
	cap := capture.NewCapture()
	cap.RegisterCommand("stuck", "q-stuck", 30*time.Minute)
	completeCommands(t, cap, 1)

	// Both the pending command and the completion are ~50s old under the
	// synthetic clock, so the stall warning fires AND a last success exists —
	// this is the branch that must print an age instead of the "none" literal.
	info := BuildCommandExecutionInfoAt(cap, time.Now().Add(50*time.Second))

	if info.Status != "warn" {
		t.Errorf("status: want warn, got %q", info.Status)
	}
	if info.LastSuccessAt == "" {
		t.Fatal("expected a recorded success")
	}
	if strings.Contains(info.Detail, "last_success=none") {
		t.Errorf("last_success must not be 'none' when a completion exists: %q", info.Detail)
	}
	if !strings.Contains(info.Detail, "last_success=50.0s ago") {
		t.Errorf("detail must report the last success age: %q", info.Detail)
	}
}

func TestBuildCommandExecutionInfo_ReportsQueueDepthSeparatelyFromPending(t *testing.T) {
	cap := capture.NewCapture()
	enqueue(t, cap, 2)                                // dispatcher queue depth
	cap.RegisterCommand("p1", "q-p1", 30*time.Minute) // lifecycle pending
	cap.RegisterCommand("p2", "q-p2", 30*time.Minute)
	cap.ApplyCommandResult("p2", "complete", nil, "") // p2 leaves the pending set

	info := BuildCommandExecutionInfoAt(cap, time.Now())
	if info.QueueDepth != 2 {
		t.Errorf("QueueDepth: want 2, got %d", info.QueueDepth)
	}
	if info.PendingCount != 1 {
		t.Errorf("PendingCount: want 1 (p2 completed), got %d", info.PendingCount)
	}
}
