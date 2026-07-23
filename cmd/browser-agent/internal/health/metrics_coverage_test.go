// metrics_coverage_test.go — Behavioural tests for Metrics and the get_health
// response builders (response_builders.go, response_types.go, deps.go).
//
// The emphasis is on the degraded surfaces: missing capture, missing server
// dependency, disconnected pilot, open buffers, and the exact JSON wire shape
// that /api/status and the get_health MCP tool depend on.
//
// No network, no sleeps, no filesystem writes outside t.TempDir().

package health

import (
	"encoding/json"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// ---------------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------------

type fakeServerDeps struct {
	terminalPort   int
	consoleEntries int
	consoleCap     int
	consoleDropped int64
	statsCalls     int
}

func (f *fakeServerDeps) GetTerminalPort() int { return f.terminalPort }
func (f *fakeServerDeps) GetConsoleStats() (int, int, int64) {
	f.statsCalls++
	return f.consoleEntries, f.consoleCap, f.consoleDropped
}

func staticLaunchMode(info LaunchModeInfo) func() LaunchModeInfo {
	return func() LaunchModeInfo { return info }
}

// ---------------------------------------------------------------------------
// Metrics — counters that must not bleed between tools
// ---------------------------------------------------------------------------

func TestMetrics_ErrorsDoNotInflateRequestCounts(t *testing.T) {
	m := NewMetrics()
	m.IncrementError("observe")
	m.IncrementError("observe")

	// A tool that only ever errored must still report 0 requests: the audit
	// error_rate_pct divides by requests, so conflating the two would produce
	// a >100% rate.
	if got := m.GetRequestCount("observe"); got != 0 {
		t.Errorf("GetRequestCount after errors only: want 0, got %d", got)
	}
	if got := m.GetTotalRequests(); got != 0 {
		t.Errorf("GetTotalRequests: want 0, got %d", got)
	}
	if got := m.GetErrorCount("observe"); got != 2 {
		t.Errorf("GetErrorCount: want 2, got %d", got)
	}
}

func TestMetrics_GetErrorCountUnknownToolIsZero(t *testing.T) {
	m := NewMetrics()
	m.IncrementError("observe")
	if got := m.GetErrorCount("never_called"); got != 0 {
		t.Errorf("unknown tool error count: want 0, got %d", got)
	}
}

func TestMetrics_BuildAuditInfo_ErrorRateSaturatesAt100(t *testing.T) {
	m := NewMetrics()
	m.IncrementRequest("interact")
	m.IncrementError("interact")
	m.IncrementError("interact")

	info := m.BuildAuditInfo()
	// error_rate_pct is a dashboard field; a rate above 100% is not a number
	// anyone can read. The raw counts stay unclamped so the discrepancy is
	// still visible to whoever looks.
	if info.ErrorRatePct != 100 {
		t.Errorf("ErrorRatePct: want 100, got %f", info.ErrorRatePct)
	}
	if info.TotalCalls != 1 || info.TotalErrors != 2 {
		t.Errorf("want calls=1 errors=2, got calls=%d errors=%d", info.TotalCalls, info.TotalErrors)
	}
}

func TestMetrics_BuildAuditInfo_ErrorRateIsExactBelowTheClamp(t *testing.T) {
	// The clamp must not round or distort an ordinary rate.
	m := NewMetrics()
	for i := 0; i < 4; i++ {
		m.IncrementRequest("observe")
	}
	m.IncrementError("observe")

	if got := m.BuildAuditInfo().ErrorRatePct; got != 25 {
		t.Errorf("ErrorRatePct: want 25, got %f", got)
	}
}

func TestMetrics_BuildAuditInfo_CallsPerToolIsADetachedCopy(t *testing.T) {
	m := NewMetrics()
	m.IncrementRequest("observe")

	info := m.BuildAuditInfo()
	info.CallsPerTool["observe"] = 999
	info.CallsPerTool["injected"] = 1

	// Mutating the returned snapshot must not corrupt the live counters —
	// BuildAuditInfo is handed straight to JSON encoders and report_issue.
	if got := m.GetRequestCount("observe"); got != 1 {
		t.Errorf("live counter mutated through the snapshot: got %d", got)
	}
	again := m.BuildAuditInfo()
	if _, ok := again.CallsPerTool["injected"]; ok {
		t.Error("snapshot mutation leaked back into the metrics map")
	}
}

func TestMetrics_BuildAuditInfo_TracksErrorsForToolsWithNoRequests(t *testing.T) {
	m := NewMetrics()
	m.IncrementRequest("observe")
	m.IncrementError("generate") // errored tool never recorded a request

	info := m.BuildAuditInfo()
	if info.TotalErrors != 1 {
		t.Errorf("TotalErrors: want 1, got %d", info.TotalErrors)
	}
	// calls_per_tool is keyed off requests only; "generate" must be absent.
	if _, ok := info.CallsPerTool["generate"]; ok {
		t.Errorf("calls_per_tool must only list tools with requests: %+v", info.CallsPerTool)
	}
	if len(info.CallsPerTool) != 1 {
		t.Errorf("calls_per_tool: want 1 entry, got %+v", info.CallsPerTool)
	}
}

func TestMetrics_GetUptimeIsMonotonicallyNonDecreasing(t *testing.T) {
	m := NewMetrics()
	first := m.GetUptime()
	second := m.GetUptime()
	if second < first {
		t.Errorf("uptime went backwards: %v then %v", first, second)
	}
	if first < 0 {
		t.Errorf("uptime must never be negative, got %v", first)
	}
}

// ---------------------------------------------------------------------------
// BuildBuffersInfo
// ---------------------------------------------------------------------------

func TestBuildBuffersInfo_NilServerFallsBackToDefaultConsoleCapacity(t *testing.T) {
	info := BuildBuffersInfo(nil, nil)

	// Without a server the console stats are unknowable; the builder must
	// still emit a sane capacity so utilization is not a divide-by-zero.
	if info.Console.Capacity != 1000 {
		t.Errorf("Console.Capacity: want 1000, got %d", info.Console.Capacity)
	}
	if info.Console.Entries != 0 || info.Console.DroppedCount != 0 {
		t.Errorf("Console stats: want zeroes, got %+v", info.Console)
	}
	if info.Console.UtilizationPct != 0 {
		t.Errorf("Console.UtilizationPct: want 0, got %f", info.Console.UtilizationPct)
	}
	// Capture capacities are compile-time constants and must appear even with
	// a nil capture, so dashboards can render an empty-but-correct gauge.
	if info.Network.Capacity != 100 {
		t.Errorf("Network.Capacity: want 100, got %d", info.Network.Capacity)
	}
	if info.WebSocket.Capacity != 500 {
		t.Errorf("WebSocket.Capacity: want 500, got %d", info.WebSocket.Capacity)
	}
	if info.Actions.Capacity != 1000 {
		t.Errorf("Actions.Capacity: want 1000, got %d", info.Actions.Capacity)
	}
}

func TestBuildBuffersInfo_ReportsDroppedConsoleEntries(t *testing.T) {
	srv := &fakeServerDeps{consoleEntries: 750, consoleCap: 1000, consoleDropped: 4242}

	info := BuildBuffersInfo(nil, srv)

	if srv.statsCalls != 1 {
		t.Errorf("GetConsoleStats should be consulted exactly once, got %d", srv.statsCalls)
	}
	if info.Console.Entries != 750 {
		t.Errorf("Console.Entries: want 750, got %d", info.Console.Entries)
	}
	// Dropped console lines are the signal that the buffer overflowed; losing
	// this in the response would hide data loss from operators.
	if info.Console.DroppedCount != 4242 {
		t.Errorf("Console.DroppedCount: want 4242, got %d", info.Console.DroppedCount)
	}
	if info.Console.UtilizationPct != 75 {
		t.Errorf("Console.UtilizationPct: want 75, got %f", info.Console.UtilizationPct)
	}
}

func TestBuildBuffersInfo_ZeroConsoleCapacityDoesNotDivideByZero(t *testing.T) {
	info := BuildBuffersInfo(nil, &fakeServerDeps{consoleEntries: 10, consoleCap: 0})
	if info.Console.UtilizationPct != 0 {
		t.Errorf("utilization with zero capacity must be 0, got %f", info.Console.UtilizationPct)
	}
}

func TestBuildBuffersInfo_ReflectsCaptureBufferCounts(t *testing.T) {
	cap := capture.NewCapture()
	cap.AddNetworkBodiesForTest([]capture.NetworkBody{{URL: "https://a.test/1", Status: 500}})
	cap.AddWebSocketEventsForTest([]capture.WebSocketEvent{{Event: "message", Data: "x"}})
	cap.AddEnhancedActionsForTest([]capture.EnhancedAction{{Type: "click"}, {Type: "click"}})

	info := BuildBuffersInfo(cap, nil)

	if info.Network.Entries != 1 {
		t.Errorf("Network.Entries: want 1, got %d", info.Network.Entries)
	}
	if info.WebSocket.Entries != 1 {
		t.Errorf("WebSocket.Entries: want 1, got %d", info.WebSocket.Entries)
	}
	if info.Actions.Entries != 2 {
		t.Errorf("Actions.Entries: want 2, got %d", info.Actions.Entries)
	}
	// 1/100 network bodies.
	if info.Network.UtilizationPct != 1 {
		t.Errorf("Network.UtilizationPct: want 1, got %f", info.Network.UtilizationPct)
	}
	// 2/1000 actions.
	if info.Actions.UtilizationPct != 0.2 {
		t.Errorf("Actions.UtilizationPct: want 0.2, got %f", info.Actions.UtilizationPct)
	}
}

// ---------------------------------------------------------------------------
// BuildRateLimitInfo
// ---------------------------------------------------------------------------

func TestBuildRateLimitInfo_NilCaptureStillReportsThreshold(t *testing.T) {
	info := BuildRateLimitInfo(nil)
	if info.Threshold != 1000 {
		t.Errorf("Threshold: want 1000, got %d", info.Threshold)
	}
	if info.CircuitOpen {
		t.Error("nil capture must not report an open circuit")
	}
	if info.CircuitReason != "" {
		t.Errorf("CircuitReason: want empty, got %q", info.CircuitReason)
	}
	if info.CurrentRate != 0 {
		t.Errorf("CurrentRate: want 0, got %d", info.CurrentRate)
	}
}

func TestBuildRateLimitInfo_ReportsCurrentWindowRate(t *testing.T) {
	cap := capture.NewCapture()
	cap.RecordEvents(37)

	info := BuildRateLimitInfo(cap)
	if info.CurrentRate != 37 {
		t.Errorf("CurrentRate: want 37, got %d", info.CurrentRate)
	}
	// Recording below the threshold must never trip the breaker.
	if info.CircuitOpen {
		t.Error("37 events/s is far below the 1000 threshold; circuit must stay closed")
	}
}

// ---------------------------------------------------------------------------
// BuildPilotInfo
// ---------------------------------------------------------------------------

func TestBuildPilotInfo_NilCaptureReportsNeverConnected(t *testing.T) {
	info := BuildPilotInfo(nil)
	if info.Source != "never_connected" {
		t.Errorf("Source: want never_connected, got %q", info.Source)
	}
	if info.Enabled {
		t.Error("Enabled must be false without a capture")
	}
	if info.ExtensionConnected {
		t.Error("ExtensionConnected must be false without a capture")
	}
}

func TestBuildPilotInfo_UnsyncedCaptureReportsAssumedStartup(t *testing.T) {
	cap := capture.NewCapture()
	cap.SetPilotUnknownForTest()

	info := BuildPilotInfo(cap)
	// Startup uncertainty is reported as enabled-but-assumed so interact is
	// not blocked before the first sync; the source names the assumption.
	if !info.Enabled {
		t.Error("Enabled must be true while pilot state is assumed")
	}
	if info.Source != "assumed_startup" {
		t.Errorf("Source: want assumed_startup, got %q", info.Source)
	}
	if info.ExtensionConnected {
		t.Error("ExtensionConnected must be false before any sync")
	}
}

func TestBuildPilotInfo_ExplicitDisableIsReported(t *testing.T) {
	cap := capture.NewCapture()
	cap.SetPilotEnabled(false)
	cap.SimulateExtensionConnectForTest()

	info := BuildPilotInfo(cap)
	if info.Enabled {
		t.Error("Enabled must be false after an explicit disable")
	}
	if info.Source != "test_helper" {
		t.Errorf("Source: want test_helper (the recorded provenance), got %q", info.Source)
	}
	if !info.ExtensionConnected {
		t.Error("ExtensionConnected must be true right after a sync")
	}
}

func TestBuildPilotInfo_StaleSyncReportsDisconnected(t *testing.T) {
	cap := capture.NewCapture()
	cap.SetPilotEnabled(true)
	cap.SimulateExtensionConnectForTest()
	cap.SimulateExtensionDisconnectForTest()

	info := BuildPilotInfo(cap)
	if !info.Enabled {
		t.Error("a stale sync must not clear the last known pilot setting")
	}
	// Connectivity is derived from sync recency, so a 1h-old sync is down.
	if info.ExtensionConnected {
		t.Error("ExtensionConnected must be false once the sync goes stale")
	}
}

// ---------------------------------------------------------------------------
// BuildMemoryInfo
// ---------------------------------------------------------------------------

func TestBuildMemoryInfo_CurrentAndAllocAreTheSameReading(t *testing.T) {
	info := BuildMemoryInfo(capture.NewCapture())

	// current_mb and alloc_mb are both fed from MemStats.Alloc; a divergence
	// would mean one of them silently changed meaning.
	if info.CurrentMB != info.AllocMB {
		t.Errorf("CurrentMB (%f) and AllocMB (%f) must agree", info.CurrentMB, info.AllocMB)
	}
	if info.AllocMB <= 0 {
		t.Errorf("AllocMB must be positive, got %f", info.AllocMB)
	}
	if info.SysMB < info.AllocMB {
		t.Errorf("SysMB (%f) must be >= AllocMB (%f)", info.SysMB, info.AllocMB)
	}
	// The per-buffer breakdown is not implemented yet and is pinned at zero;
	// this test fails the day someone wires it up without updating docs.
	if info.BufferBreakdown != (BufferMemoryBreakdown{}) {
		t.Errorf("BufferBreakdown is expected to be unpopulated, got %+v", info.BufferBreakdown)
	}
}

// ---------------------------------------------------------------------------
// GetHealth — full assembly
// ---------------------------------------------------------------------------

func TestGetHealth_AssemblesEveryStanzaForADegradedServer(t *testing.T) {
	m := NewMetrics()
	m.IncrementRequest("observe")
	m.IncrementRequest("observe")
	m.IncrementError("observe")

	cap := capture.NewCapture() // extension never connected
	srv := &fakeServerDeps{terminalPort: 7411, consoleEntries: 12, consoleCap: 100, consoleDropped: 3}
	upg := &fakeUpgradeProvider{
		pending:    true,
		newVer:     "1.4.0",
		detectedAt: time.Date(2026, 7, 1, 8, 30, 0, 0, time.UTC),
	}

	resp := m.GetHealth(cap, srv, upg, staticLaunchMode(LaunchModeInfo{
		Mode:          "stdio",
		Reason:        "parent_is_mcp_client",
		ParentProcess: "claude",
	}), "2.0.0-rc1")

	if resp.Server.Version != "2.0.0-rc1" {
		t.Errorf("Server.Version: want 2.0.0-rc1, got %q", resp.Server.Version)
	}
	if resp.Server.PID != os.Getpid() {
		t.Errorf("Server.PID: want %d, got %d", os.Getpid(), resp.Server.PID)
	}
	if want := runtime.GOOS + "/" + runtime.GOARCH; resp.Server.Platform != want {
		t.Errorf("Server.Platform: want %q, got %q", want, resp.Server.Platform)
	}
	if resp.Server.GoVersion != runtime.Version() {
		t.Errorf("Server.GoVersion: want %q, got %q", runtime.Version(), resp.Server.GoVersion)
	}
	if resp.Server.LaunchMode != "stdio" || resp.Server.LaunchModeReason != "parent_is_mcp_client" ||
		resp.Server.ParentProcess != "claude" {
		t.Errorf("launch mode not threaded through: %+v", resp.Server)
	}
	// TerminalPort comes from the server dep, not from launch mode.
	if resp.Server.TerminalPort != 7411 {
		t.Errorf("Server.TerminalPort: want 7411, got %d", resp.Server.TerminalPort)
	}
	if resp.Server.UptimeSeconds < 0 {
		t.Errorf("UptimeSeconds must not be negative, got %f", resp.Server.UptimeSeconds)
	}

	if resp.Audit.TotalCalls != 2 || resp.Audit.TotalErrors != 1 {
		t.Errorf("Audit: want calls=2 errors=1, got %+v", resp.Audit)
	}
	if resp.Audit.ErrorRatePct != 50 {
		t.Errorf("Audit.ErrorRatePct: want 50, got %f", resp.Audit.ErrorRatePct)
	}
	if resp.Buffers.Console.Entries != 12 || resp.Buffers.Console.DroppedCount != 3 {
		t.Errorf("Buffers.Console: got %+v", resp.Buffers.Console)
	}
	if resp.RateLimiting.Threshold != 1000 {
		t.Errorf("RateLimiting.Threshold: want 1000, got %d", resp.RateLimiting.Threshold)
	}
	if resp.Pilot.ExtensionConnected {
		t.Error("Pilot.ExtensionConnected must be false for a never-synced capture")
	}
	if resp.CommandExecution.Status != "pass" || !resp.CommandExecution.Ready {
		t.Errorf("CommandExecution on an idle capture: got %+v", resp.CommandExecution)
	}
	if resp.Upgrade == nil {
		t.Fatal("Upgrade must be populated when one is pending")
	}
	if resp.Upgrade.NewVersion != "1.4.0" || resp.Upgrade.DetectedAt != "2026-07-01T08:30:00Z" {
		t.Errorf("Upgrade: got %+v", resp.Upgrade)
	}
}

func TestGetHealth_NilServerOmitsTerminalPortAndUsesDefaultConsoleCapacity(t *testing.T) {
	m := NewMetrics()
	resp := m.GetHealth(capture.NewCapture(), nil, nil, staticLaunchMode(LaunchModeInfo{}), "0.1.0")

	if resp.Server.TerminalPort != 0 {
		t.Errorf("TerminalPort must stay 0 without a server dep, got %d", resp.Server.TerminalPort)
	}
	if resp.Buffers.Console.Capacity != 1000 {
		t.Errorf("Console.Capacity: want the 1000 default, got %d", resp.Buffers.Console.Capacity)
	}
	if resp.Upgrade != nil {
		t.Errorf("Upgrade must be nil without a provider, got %+v", resp.Upgrade)
	}
}

func TestGetHealth_NilCaptureReportsCommandExecutionFailure(t *testing.T) {
	m := NewMetrics()
	resp := m.GetHealth(nil, nil, nil, staticLaunchMode(LaunchModeInfo{}), "0.1.0")

	// A daemon with no capture wired up is unusable; the health payload must
	// say so rather than reporting a cheerful all-zero response.
	if resp.CommandExecution.Ready {
		t.Error("CommandExecution.Ready must be false with a nil capture")
	}
	if resp.CommandExecution.Status != "fail" {
		t.Errorf("CommandExecution.Status: want fail, got %q", resp.CommandExecution.Status)
	}
	if resp.CommandExecution.Detail != "Capture not initialized" {
		t.Errorf("CommandExecution.Detail: got %q", resp.CommandExecution.Detail)
	}
	if resp.Pilot.Source != "never_connected" {
		t.Errorf("Pilot.Source: want never_connected, got %q", resp.Pilot.Source)
	}
}

func TestGetHealth_UpgradeOmittedWhenNotPending(t *testing.T) {
	m := NewMetrics()
	resp := m.GetHealth(capture.NewCapture(), nil,
		&fakeUpgradeProvider{pending: false, newVer: "9.9.9"},
		staticLaunchMode(LaunchModeInfo{}), "0.1.0")

	if resp.Upgrade != nil {
		t.Errorf("a non-pending upgrade must not be surfaced, got %+v", resp.Upgrade)
	}
}

// ---------------------------------------------------------------------------
// Wire shape — these keys are consumed by GET /api/status and openapi.json
// ---------------------------------------------------------------------------

func TestMCPHealthResponse_JSONKeysAreSnakeCaseAndStable(t *testing.T) {
	m := NewMetrics()
	m.IncrementRequest("observe")
	resp := m.GetHealth(capture.NewCapture(), &fakeServerDeps{terminalPort: 1},
		nil, staticLaunchMode(LaunchModeInfo{Mode: "http"}), "1.0.0")

	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{
		"server", "memory", "buffers", "rate_limiting", "audit", "pilot", "command_execution",
	} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("top-level key %q missing from get_health payload: %s", key, raw)
		}
	}
	// upgrade is omitempty and must be absent when nothing is pending.
	if _, ok := decoded["upgrade"]; ok {
		t.Error("upgrade key must be omitted when no upgrade is pending")
	}

	var server map[string]json.RawMessage
	if err := json.Unmarshal(decoded["server"], &server); err != nil {
		t.Fatalf("unmarshal server: %v", err)
	}
	for _, key := range []string{"version", "uptime_seconds", "pid", "platform", "go_version", "terminal_port"} {
		if _, ok := server[key]; !ok {
			t.Errorf("server.%s missing: %s", key, decoded["server"])
		}
	}
	// launch_mode_reason and parent_process are omitempty and were left blank.
	if _, ok := server["launch_mode_reason"]; ok {
		t.Error("empty launch_mode_reason must be omitted")
	}
}

func TestCommandExecutionInfo_OmitsZeroValuedOptionalFields(t *testing.T) {
	raw, err := json.Marshal(BuildCommandExecutionInfo(capture.NewCapture()))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Required keys are always present, even at zero.
	for _, key := range []string{
		"ready", "status", "detail", "window_seconds", "queue_depth", "pending_count",
		"recent_success_count", "recent_failed_count", "recent_failure_rate_pct",
	} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("required key %q missing: %s", key, raw)
		}
	}
	// omitempty keys must not appear for an idle daemon, keeping the payload
	// small on the hot get_health path.
	for _, key := range []string{"oldest_pending_age_ms", "last_success_at", "last_success_age_ms"} {
		if _, ok := decoded[key]; ok {
			t.Errorf("optional key %q must be omitted when zero: %s", key, raw)
		}
	}
}

func TestDoctorCheck_OmitsFixWhenEmpty(t *testing.T) {
	raw, err := json.Marshal(DoctorCheck{Name: "n", Status: "pass", Detail: "d"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got := string(raw); got != `{"name":"n","status":"pass","detail":"d"}` {
		t.Errorf("passing checks must omit fix, got %s", got)
	}
}
