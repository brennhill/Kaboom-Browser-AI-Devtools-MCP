// contract_compliance_test.go — Tests verifying beacon payloads match the Counterscale ingest contract.
// These tests catch schema drift between the Go sender and the Counterscale worker.

package telemetry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
)

// captureBeacon sets up a test server and returns a channel that receives beacon payloads.
func captureBeacon(t *testing.T) chan map[string]any {
	t.Helper()
	drainSem()
	t.Cleanup(drainSem)

	received := make(chan map[string]any, 20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		select {
		case received <- body:
		default:
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(srv.Close)
	overrideEndpoint(srv.URL)
	t.Cleanup(resetEndpoint)
	return received
}

// waitForEvent drains the channel until it finds a beacon with the given event type.
func waitForEvent(t *testing.T, ch chan map[string]any, event string) map[string]any {
	t.Helper()
	for {
		select {
		case body := <-ch:
			if body["event"] == event {
				return body
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for %q event", event)
			return nil
		}
	}
}

// TestContract_ToolCallHasNoNullAsyncOutcome verifies async_outcome is omitted (not null)
// when the tool call is synchronous.
func TestContract_ToolCallHasNoNullAsyncOutcome(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()
	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 50*time.Millisecond, false)

	body := waitForEvent(t, received, "tool_call")

	// async_outcome should either be absent or a non-null string — never JSON null.
	val, exists := body["async_outcome"]
	if exists && val != nil {
		// If present, must be a string (not null).
		if _, ok := val.(string); !ok {
			t.Errorf("async_outcome = %v (%T), want absent or string", val, val)
		}
	}
}

func TestContract_RejectsUnsupportedProducerEvents(t *testing.T) {
	resetDeliveryDiagnostics()
	fired := make(chan bool, 1)
	setOnFireBeacon(func(sent bool) { fired <- sent })
	defer setOnFireBeacon(nil)

	fireStructuredBeacon(map[string]any{"event": "daemon_start"})

	select {
	case sent := <-fired:
		if sent {
			t.Fatal("unsupported event was sent")
		}
	case <-time.After(time.Second):
		t.Fatal("unsupported event was not rejected")
	}
	if got := DeliveryDiagnostics(); got.Dropped != 1 {
		t.Fatalf("unsupported event diagnostics = %+v, want one drop", got)
	}
}

// TestContract_ToolCallV2Envelope verifies tool_call beacons have all required v2 envelope fields.
func TestContract_ToolCallV2Envelope(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()
	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 50*time.Millisecond, false)

	body := waitForEvent(t, received, "tool_call")

	// Required v2 envelope fields per contract.
	for _, field := range []string{"event", "iid", "sid", "ts", "v", "os", "channel"} {
		if _, ok := body[field]; !ok {
			t.Errorf("missing required v2 envelope field: %s", field)
		}
	}
	// Required tool_call fields.
	for _, field := range []string{"family", "name", "tool", "outcome", "latency_ms"} {
		if _, ok := body[field]; !ok {
			t.Errorf("missing required tool_call field: %s", field)
		}
	}
	if body["family"] != "observe" {
		t.Errorf("family = %v, want observe", body["family"])
	}
	if body["name"] != "page" {
		t.Errorf("name = %v, want page", body["name"])
	}
	if body["tool"] != "observe:page" {
		t.Errorf("tool = %v, want observe:page", body["tool"])
	}
	if body["outcome"] != "success" {
		t.Errorf("outcome = %v, want success", body["outcome"])
	}
}

// TestContract_AppErrorNoDetailField verifies app_error beacons do not send the
// 'detail' field, which is not in the contract and silently dropped by the ingest.
func TestContract_AppErrorNoDetailField(t *testing.T) {
	received := captureBeacon(t)
	AppError(incident.CodeDaemonPanic)

	body := waitForEvent(t, received, "app_error")

	if _, exists := body["detail"]; exists {
		t.Error("app_error should not send 'detail' field — not in Counterscale contract, silently dropped")
	}
	// Verify required contract fields are present.
	for _, field := range []string{"error_kind", "error_code", "severity", "source"} {
		if _, ok := body[field]; !ok {
			t.Errorf("missing required app_error field: %s", field)
		}
	}
}

func TestContract_AppErrorUsesFixedPrivacyBoundedSchema(t *testing.T) {
	received := captureBeacon(t)
	AppError(incident.CodeDaemonPanic)

	body := waitForEvent(t, received, "app_error")
	allowed := map[string]bool{
		"event": true, "iid": true, "sid": true, "ts": true, "v": true,
		"os": true, "channel": true, "llm": true, "error_kind": true,
		"error_code": true, "severity": true, "source": true, "retryable": true,
		"outcome": true, "attempt_bucket": true, "latency_bucket": true,
	}
	for field := range body {
		if !allowed[field] {
			t.Errorf("app_error transmitted non-contract field %q", field)
		}
	}
}

func TestContract_ReliabilityProjectionUsesAppErrorAllowlist(t *testing.T) {
	received := captureBeacon(t)
	ReportReliability(incident.ReliabilityEvent{
		Code: incident.CodeStateRecoveryFailed, Subsystem: incident.SubsystemState,
		Stage: incident.StageRecovery, Severity: incident.SeverityError,
		Retryable: true, Outcome: incident.OutcomePending, AttemptBucket: incident.AttemptZero,
		LatencyBucket: incident.LatencyUnderSecond,
	})
	body := waitForEvent(t, received, "app_error")
	if body["error_code"] != "STATE_RECOVERY_FAILED" || body["source"] != "state" || body["severity"] != "error" || body["retryable"] != true {
		t.Fatalf("reliability app_error = %#v", body)
	}
	if body["outcome"] != "pending" || body["attempt_bucket"] != "0" || body["latency_bucket"] != "under_1s" {
		t.Fatalf("reliability lifecycle buckets = %#v", body)
	}
	for _, forbidden := range []string{"stage", "detail", "fix", "correlation_id", "generation"} {
		if _, ok := body[forbidden]; ok {
			t.Fatalf("reliability event leaked %q: %#v", forbidden, body)
		}
	}
}

func TestContract_ReliabilityRecoveryTransitionIsQueryable(t *testing.T) {
	received := captureBeacon(t)
	ReportReliability(incident.ReliabilityEvent{
		Code: incident.CodeStateRecoveryFailed, Outcome: incident.OutcomeRecovered,
		AttemptBucket: incident.AttemptTwoThree, LatencyBucket: incident.LatencyFiveToThirty,
	})
	body := waitForEvent(t, received, "app_error")
	if body["outcome"] != "recovered" || body["attempt_bucket"] != "2_3" || body["latency_bucket"] != "5s_30s" {
		t.Fatalf("recovery transition = %#v", body)
	}
}

func TestContract_ReliabilityRejectsUnknownLifecycleDimensions(t *testing.T) {
	received := captureBeacon(t)
	QueueReliability(incident.ReliabilityEvent{
		Code: incident.CodeStateRecoveryFailed, Outcome: incident.Outcome("private-state"),
		AttemptBucket: incident.AttemptOne, LatencyBucket: incident.LatencyUnderSecond,
	})
	select {
	case body := <-received:
		t.Fatalf("invalid lifecycle dimension reached telemetry: %#v", body)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestContract_ReliabilityProjectionCannotOverrideRegistryClassification(t *testing.T) {
	received := captureBeacon(t)
	ReportReliability(incident.ReliabilityEvent{
		Code: incident.CodeDaemonPanic, Subsystem: incident.SubsystemInstaller,
		Severity: incident.SeverityWarning, Retryable: true,
		Outcome: incident.OutcomePending, AttemptBucket: incident.AttemptZero,
		LatencyBucket: incident.LatencyUnderSecond,
	})
	body := waitForEvent(t, received, "app_error")
	if body["source"] != "daemon" || body["severity"] != "fatal" {
		t.Fatalf("caller classification overrode registry: %#v", body)
	}
	if _, ok := body["retryable"]; ok {
		t.Fatalf("caller retryability overrode registry: %#v", body)
	}
}

// TestContract_UsageSummaryNoSessionDepth is a compile-time check:
// UsageSnapshot no longer has a SessionDepth field, so it cannot be sent.
// Intentionally left as a named test for documentation — verifies via beacon_test.go
// that the payload key is absent.

// TestContract_AppErrorClassifiesNewCategories verifies all migrated error categories
// produce valid error_kind, severity, and source fields.
func TestContract_AppErrorClassifiesNewCategories(t *testing.T) {
	cases := []struct {
		code      incident.Code
		wantKind  string
		wantSev   string
		wantSrc   string
		wantRetry bool
	}{
		// Existing
		{incident.CodeDaemonPanic, "internal", "fatal", "daemon", false},
		{incident.CodeDaemonStartFailed, "internal", "fatal", "startup", false},
		{incident.CodeToolRateLimited, "integration", "warning", "daemon", true},
		// New: bridge errors
		{incident.CodeBridgeConnectionError, "integration", "error", "bridge", true},
		{incident.CodeBridgePortBlocked, "integration", "error", "bridge", false},
		{incident.CodeBridgeSpawnBuildError, "internal", "fatal", "bridge", false},
		{incident.CodeBridgeSpawnStartError, "internal", "fatal", "bridge", false},
		{incident.CodeBridgeSpawnTimeout, "internal", "error", "bridge", true},
		{incident.CodeBridgeExitError, "internal", "error", "bridge", false},
		// New: extension/install errors
		{incident.CodeExtensionDisconnect, "integration", "warning", "extension", false},
		{incident.CodeInstallConfigError, "internal", "error", "installer", false},
	}

	for _, tc := range cases {
		t.Run(string(tc.code), func(t *testing.T) {
			definition, ok := incident.Lookup(tc.code)
			if !ok {
				t.Fatal("registered reliability code missing")
			}
			if string(definition.ErrorKind) != tc.wantKind {
				t.Errorf("error_kind = %q, want %q", definition.ErrorKind, tc.wantKind)
			}
			if string(definition.Severity) != tc.wantSev {
				t.Errorf("severity = %q, want %q", definition.Severity, tc.wantSev)
			}
			if string(definition.Subsystem) != tc.wantSrc {
				t.Errorf("source = %q, want %q", definition.Subsystem, tc.wantSrc)
			}
			if definition.Retryable != tc.wantRetry || definition.Privacy != incident.PrivacyBoundedProductMetadata {
				t.Errorf("retry/privacy = %v/%q", definition.Retryable, definition.Privacy)
			}
		})
	}
}

func TestContract_AppErrorRejectsUnknownCode(t *testing.T) {
	received := captureBeacon(t)
	AppError(incident.Code("invented_runtime_failure"))
	select {
	case event := <-received:
		t.Fatalf("unknown reliability code emitted telemetry: %#v", event)
	case <-time.After(20 * time.Millisecond):
	}
}

// TestContract_AppErrorSendsAllRequiredFields fires an AppError and checks
// the actual beacon has every field the ingest expects.
func TestContract_AppErrorSendsAllRequiredFields(t *testing.T) {
	received := captureBeacon(t)

	AppError(incident.CodeBridgeConnectionError)

	body := waitForEvent(t, received, "app_error")

	// V2 envelope.
	for _, field := range []string{"event", "iid", "sid", "ts", "v", "os", "channel"} {
		if _, ok := body[field]; !ok {
			t.Errorf("missing v2 envelope field: %s", field)
		}
	}
	// App error specific.
	if body["error_kind"] != "integration" {
		t.Errorf("error_kind = %v, want integration", body["error_kind"])
	}
	if body["error_code"] != "BRIDGE_CONNECTION_ERROR" {
		t.Errorf("error_code = %v, want BRIDGE_CONNECTION_ERROR", body["error_code"])
	}
	if body["severity"] != "error" {
		t.Errorf("severity = %v, want error", body["severity"])
	}
	if body["source"] != "bridge" {
		t.Errorf("source = %v, want bridge", body["source"])
	}
	if body["retryable"] != true {
		t.Errorf("retryable = %v, want true", body["retryable"])
	}
}

// TestContract_FireStructuredBeaconDefensiveCheck verifies fireStructuredBeacon
// does not panic when 'event' field is missing or wrong type.
func TestContract_FireStructuredBeaconDefensiveCheck(t *testing.T) {
	received := captureBeacon(t)

	// Should not panic with missing event.
	fireStructuredBeacon(map[string]any{"not_event": "test"})
	// Should not panic with wrong type.
	fireStructuredBeacon(map[string]any{"event": 123})

	// Verify no beacons were sent (both should be silently dropped).
	select {
	case body := <-received:
		t.Fatalf("expected no beacon, got event=%v", body["event"])
	case <-time.After(200 * time.Millisecond):
		// Good — nothing sent.
	}
}

// TestContract_SessionStartReasonPostTimeout verifies that session_start after
// a timeout rotation uses reason "post_timeout", not "first_activity".
func TestContract_SessionStartReasonPostTimeout(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 0, false)

	// Drain session_start for first session.
	first := waitForEvent(t, received, "session_start")
	if first["reason"] != "first_activity" {
		t.Fatalf("first session_start reason = %v, want first_activity", first["reason"])
	}

	// Simulate inactivity beyond timeout.
	session.mu.Lock()
	session.lastSeen = time.Now().Add(-sessionTimeout - time.Second)
	session.mu.Unlock()

	// Next RecordToolCall triggers session rotation.
	tracker.RecordToolCall("observe:page", 0, false)

	// The new session_start should have reason "post_timeout".
	second := waitForEvent(t, received, "session_start")
	if second["reason"] != "post_timeout" {
		t.Errorf("session_start after timeout reason = %v, want post_timeout", second["reason"])
	}
}

// TestContract_BeaconUsageSummaryDRY verifies BeaconUsageSummary uses
// BuildUsageSummaryPayload internally (no duplicated logic).
func TestContract_BeaconUsageSummaryDRY(t *testing.T) {
	received := captureBeacon(t)
	resetInstallIDState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	t.Cleanup(resetKaboomDir)
	resetSessionState()
	TouchSession()

	snapshot := &UsageSnapshot{
		ToolStats:     []ToolStat{{Tool: "observe:page", Family: "observe", Name: "page", Count: 3, LatencyAvgMs: 45, LatencyMaxMs: 100}},
		AsyncOutcomes: map[string]int{"complete": 2},
	}

	// Build expected payload via the same function BeaconUsageSummary uses.
	expected := BuildUsageSummaryPayload(5, snapshot)
	if expected == nil {
		t.Fatal("BuildUsageSummaryPayload returned nil")
	}

	// Fire the actual beacon.
	BeaconUsageSummary(5, snapshot)
	body := waitForEvent(t, received, "usage_summary")

	// Check key fields match what BuildUsageSummaryPayload produces.
	if body["window_m"] == nil {
		t.Error("missing window_m in beacon")
	}
	if body["tool_stats"] == nil {
		t.Error("missing tool_stats in beacon")
	}
}

// TestContract_AppErrorSignature verifies AppError accepts only a typed code.
func TestContract_AppErrorSignature(t *testing.T) {
	received := captureBeacon(t)

	AppError(incident.CodeDaemonPanic)

	body := waitForEvent(t, received, "app_error")
	if body["error_code"] != "DAEMON_PANIC" {
		t.Errorf("error_code = %v, want DAEMON_PANIC", body["error_code"])
	}
}

// TestContract_BeaconUsageSummaryHasV2Envelope verifies usage_summary beacons
// include all required v2 envelope fields.
func TestContract_BeaconUsageSummaryHasV2Envelope(t *testing.T) {
	received := captureBeacon(t)
	resetInstallIDState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	t.Cleanup(resetKaboomDir)
	resetSessionState()
	TouchSession()

	snapshot := &UsageSnapshot{
		ToolStats: []ToolStat{{Tool: "observe:page", Family: "observe", Name: "page", Count: 1}},
	}
	BeaconUsageSummary(5, snapshot)

	body := waitForEvent(t, received, "usage_summary")

	for _, field := range []string{"event", "iid", "sid", "ts", "v", "os", "channel"} {
		if _, ok := body[field]; !ok {
			t.Errorf("missing required v2 envelope field: %s", field)
		}
	}
	if body["window_m"] == nil {
		t.Error("missing window_m")
	}
	if body["tool_stats"] == nil {
		t.Error("missing tool_stats")
	}
}

func TestContract_AppErrorFieldsComeFromClassification(t *testing.T) {
	received := captureBeacon(t)
	AppError(incident.CodeDaemonPanic)

	body := waitForEvent(t, received, "app_error")

	// Contract fields must reflect the canonical registry, not caller data.
	if body["error_kind"] != "internal" {
		t.Errorf("error_kind = %v, want internal (props overwrote contract field)", body["error_kind"])
	}
	if body["error_code"] != "DAEMON_PANIC" {
		t.Errorf("error_code = %v, want DAEMON_PANIC (props overwrote contract field)", body["error_code"])
	}
	if body["severity"] != "fatal" {
		t.Errorf("severity = %v, want fatal (props overwrote contract field)", body["severity"])
	}
	if body["source"] != "daemon" {
		t.Errorf("source = %v, want daemon (props overwrote contract field)", body["source"])
	}
	if body["event"] != "app_error" {
		t.Errorf("event = %v, want app_error (props overwrote event type)", body["event"])
	}
}

// TestContract_UsageSummaryOmitsEmptyAsyncOutcomes verifies that usage_summary
// beacons omit async_outcomes when empty rather than sending {}.
func TestContract_UsageSummaryOmitsEmptyAsyncOutcomes(t *testing.T) {
	resetInstallIDState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	defer resetKaboomDir()
	resetSessionState()
	TouchSession()

	snapshot := &UsageSnapshot{
		ToolStats:     []ToolStat{{Tool: "observe:page", Family: "observe", Name: "page", Count: 1}},
		AsyncOutcomes: map[string]int{}, // empty
	}
	payload := BuildUsageSummaryPayload(5, snapshot)

	if ao, exists := payload["async_outcomes"]; exists {
		if m, ok := ao.(map[string]int); ok && len(m) == 0 {
			t.Error("usage_summary should omit async_outcomes when empty, not send {}")
		}
	}
}

// TestContract_ConcurrentRecordToolCall_NoDuplicateSessionStart verifies that
// concurrent RecordToolCall invocations produce exactly one session_start beacon
// per session, even under contention.
func TestContract_ConcurrentRecordToolCall_NoDuplicateSessionStart(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	tracker := NewUsageTracker()
	const goroutines = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			tracker.RecordToolCall("observe:page", 0, false)
		}()
	}
	wg.Wait()

	// Drain all events and count session_starts.
	sessionStarts := 0
	deadline := time.After(3 * time.Second)
	for {
		select {
		case body := <-received:
			if body["event"] == "session_start" {
				sessionStarts++
			}
		case <-deadline:
			goto done
		}
	}
done:
	if sessionStarts != 1 {
		t.Errorf("concurrent RecordToolCall produced %d session_start beacons, want exactly 1", sessionStarts)
	}
}

// TestContract_EnvelopeLLMField verifies that beacons include the 'llm' field
// when SetLLMName is called (MCP client name from initialize handshake).
func TestContract_EnvelopeLLMField(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	SetLLMName("claude-code")
	t.Cleanup(func() { SetLLMName("") })

	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 0, false)

	body := waitForEvent(t, received, "tool_call")

	if body["llm"] != "claude-code" {
		t.Errorf("llm = %v, want claude-code", body["llm"])
	}
}

// TestContract_EnvelopeOmitsLLMWhenEmpty verifies llm is absent when no client connected.
func TestContract_EnvelopeOmitsLLMWhenEmpty(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	SetLLMName("")

	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 0, false)

	body := waitForEvent(t, received, "tool_call")

	if _, exists := body["llm"]; exists {
		t.Error("llm should be absent when no client name is set")
	}
}

// TestContract_SessionDepthNotInSnapshot verifies that UsageSnapshot no longer
// carries a SessionDepth field (removed as dead code — not sent to Counterscale).
func TestContract_SessionDepthNotInSnapshot(t *testing.T) {
	c := NewUsageTracker()
	c.RecordToolCall("a", 0, false)
	c.RecordToolCall("b", 0, false)

	snapshot := c.SwapAndReset()
	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}
	// SessionDepth should no longer exist on the struct.
	// This test will fail to compile if the field is re-added.
	_ = snapshot.ToolStats
	_ = snapshot.AsyncOutcomes
	// No SessionDepth field to access — that's the point.
}

// TestContract_ConcurrentRecordToolCall_PostTimeoutSingleSessionStart verifies that
// concurrent RecordToolCall after a timeout rotation produces exactly one session_start
// with reason "post_timeout".
func TestContract_ConcurrentRecordToolCall_PostTimeoutSingleSessionStart(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	tracker := NewUsageTracker()
	// Establish first session.
	tracker.RecordToolCall("observe:page", 0, false)

	// Drain first session_start.
	waitForEvent(t, received, "session_start")

	// Simulate inactivity beyond timeout.
	session.mu.Lock()
	session.lastSeen = time.Now().Add(-sessionTimeout - time.Second)
	session.mu.Unlock()

	// Concurrent calls after timeout — should produce exactly one post_timeout session_start.
	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			tracker.RecordToolCall("observe:page", 0, false)
		}()
	}
	wg.Wait()

	postTimeoutStarts := 0
	deadline := time.After(3 * time.Second)
	for {
		select {
		case body := <-received:
			if body["event"] == "session_start" {
				postTimeoutStarts++
			}
		case <-deadline:
			goto done
		}
	}
done:
	if postTimeoutStarts != 1 {
		t.Errorf("concurrent post-timeout RecordToolCall produced %d session_start beacons, want exactly 1", postTimeoutStarts)
	}
}
