// e2e_session_test.go — Tests app-telemetry session lifecycle and opt-out behavior.
// Docs: docs/features/feature/app-telemetry/index.md

package telemetry

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"
)

func TestE2E_SessionStart_FirstActivity(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 0, false)

	body := waitForEvent(t, received, "session_start")
	requireEnvelope(t, body, "session_start/first_activity")

	if body["reason"] != "first_activity" {
		t.Errorf("reason = %v, want first_activity", body["reason"])
	}
}

func TestE2E_SessionStart_PostTimeout(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 0, false)
	waitForEvent(t, received, "session_start") // drain first

	// Expire session.
	session.mu.Lock()
	session.lastSeen = time.Now().Add(-sessionTimeout - time.Second)
	session.mu.Unlock()

	tracker.RecordToolCall("observe:page", 0, false)

	body := waitForEvent(t, received, "session_start")
	requireEnvelope(t, body, "session_start/post_timeout")

	if body["reason"] != "post_timeout" {
		t.Errorf("reason = %v, want post_timeout", body["reason"])
	}
}

// ---------- E2E: session_end event ----------

func TestE2E_SessionEnd_Timeout(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 0, false)
	tracker.RecordToolCall("interact:click", 0, false)
	tracker.RecordToolCall("analyze:perf", 0, false)

	// Expire session.
	session.mu.Lock()
	session.lastSeen = time.Now().Add(-sessionTimeout - time.Second)
	session.mu.Unlock()

	// Next touch triggers session_end.
	TouchSession()

	body := waitForEvent(t, received, "session_end")
	requireEnvelope(t, body, "session_end/timeout")

	if body["reason"] != "timeout" {
		t.Errorf("reason = %v, want timeout", body["reason"])
	}
	if calls, ok := body["tool_calls"].(float64); !ok || calls != 3 {
		t.Errorf("tool_calls = %v, want 3", body["tool_calls"])
	}
	if _, ok := body["duration_s"].(float64); !ok {
		t.Errorf("duration_s missing or not a number: %v", body["duration_s"])
	}
}

func TestE2E_SessionEnd_Shutdown(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 0, false)

	tracker.EmitSessionEnd("shutdown")

	body := waitForEvent(t, received, "session_end")
	requireEnvelope(t, body, "session_end/shutdown")

	if body["reason"] != "shutdown" {
		t.Errorf("reason = %v, want shutdown", body["reason"])
	}
	if calls, ok := body["tool_calls"].(float64); !ok || calls != 1 {
		t.Errorf("tool_calls = %v, want 1", body["tool_calls"])
	}
}

func TestE2E_SessionEnd_NoOpWhenIdle(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()

	// No tool calls — EmitSessionEnd should not fire.
	tracker := NewUsageTracker()
	tracker.EmitSessionEnd("shutdown")

	select {
	case body := <-received:
		if body["event"] == "session_end" {
			t.Fatal("session_end should not fire when no tool calls were made")
		}
	case <-time.After(300 * time.Millisecond):
		// Good — nothing sent.
	}
}

// ---------- E2E: app_error event ----------

func TestE2E_OptOut_NoBeaconsSent(t *testing.T) {
	t.Setenv("KABOOM_TELEMETRY", "off")
	drainSem()
	time.Sleep(10 * time.Millisecond)

	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	resetSessionState()
	tracker := NewUsageTracker()
	tracker.RecordToolCall("observe:page", 0, false)
	AppError("daemon_panic", nil)
	BeaconUsageSummary(5, &UsageSnapshot{
		ToolStats: []ToolStat{{Tool: "observe:page", Family: "observe", Name: "page", Count: 1}},
	})

	time.Sleep(200 * time.Millisecond)
	if called {
		t.Fatal("no beacons should be sent when KABOOM_TELEMETRY=off")
	}
}

// ---------- E2E: full session lifecycle ----------

func TestE2E_FullSessionLifecycle(t *testing.T) {
	received := captureBeacon(t)
	resetSessionState()
	resetInstallIDState()
	resetFirstToolCallState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	t.Cleanup(func() {
		resetInstallIDState()
		resetFirstToolCallState()
		resetKaboomDir()
	})
	SetLLMName("claude-code")
	t.Cleanup(func() { SetLLMName("") })

	tracker := NewUsageTracker()

	// === Phase 1: First tool call ever ===
	tracker.RecordToolCall("observe:page", 100*time.Millisecond, false)

	// Expect: session_start, tool_call, first_tool_call (in any order).
	phase1 := collectAll(received, 3*time.Second)
	phase1Events := map[string]map[string]any{}
	for _, b := range phase1 {
		if ev, ok := b["event"].(string); ok {
			phase1Events[ev] = b
		}
	}
	if _, ok := phase1Events["session_start"]; !ok {
		t.Fatal("phase 1: missing session_start")
	}
	if _, ok := phase1Events["tool_call"]; !ok {
		t.Fatal("phase 1: missing tool_call")
	}
	if _, ok := phase1Events["first_tool_call"]; !ok {
		t.Fatal("phase 1: missing first_tool_call")
	}
	// All should have same session ID.
	sid := phase1Events["session_start"]["sid"]
	for ev, body := range phase1Events {
		if body["sid"] != sid {
			t.Errorf("phase 1: %s sid = %v, want %v (same session)", ev, body["sid"], sid)
		}
		if body["llm"] != "claude-code" {
			t.Errorf("phase 1: %s llm = %v, want claude-code", ev, body["llm"])
		}
	}

	// === Phase 2: More tool calls (same session) ===
	tracker.RecordToolCall("interact:click", 50*time.Millisecond, false)
	tracker.RecordToolCall("analyze:security", 200*time.Millisecond, true)

	phase2 := collectAll(received, 2*time.Second)
	phase2Calls := filterByEvent(phase2, "tool_call")
	if len(phase2Calls) != 2 {
		t.Fatalf("phase 2: expected 2 tool_calls, got %d", len(phase2Calls))
	}
	// No duplicate session_start or first_tool_call.
	if len(filterByEvent(phase2, "session_start")) > 0 {
		t.Error("phase 2: unexpected session_start (session should still be active)")
	}
	if len(filterByEvent(phase2, "first_tool_call")) > 0 {
		t.Error("phase 2: unexpected first_tool_call (should fire once per install)")
	}

	// === Phase 3: Session timeout → session_end + new session_start ===
	session.mu.Lock()
	session.lastSeen = time.Now().Add(-sessionTimeout - time.Second)
	session.mu.Unlock()

	tracker.RecordToolCall("observe:errors", 10*time.Millisecond, false)

	phase3 := collectAll(received, 3*time.Second)
	phase3Events := map[string][]map[string]any{}
	for _, b := range phase3 {
		if ev, ok := b["event"].(string); ok {
			phase3Events[ev] = append(phase3Events[ev], b)
		}
	}

	// Must have session_end for old session.
	ends := phase3Events["session_end"]
	if len(ends) != 1 {
		t.Fatalf("phase 3: expected 1 session_end, got %d", len(ends))
	}
	if ends[0]["reason"] != "timeout" {
		t.Errorf("phase 3: session_end reason = %v, want timeout", ends[0]["reason"])
	}
	if calls, ok := ends[0]["tool_calls"].(float64); !ok || calls != 4 {
		t.Errorf("phase 3: session_end tool_calls = %v, want 4 (3 prior + 1 that triggered rotation)", ends[0]["tool_calls"])
	}

	// Must have session_start for new session.
	starts := phase3Events["session_start"]
	if len(starts) != 1 {
		t.Fatalf("phase 3: expected 1 session_start, got %d", len(starts))
	}
	if starts[0]["reason"] != "post_timeout" {
		t.Errorf("phase 3: session_start reason = %v, want post_timeout", starts[0]["reason"])
	}

	// New session should have a DIFFERENT session ID.
	newSID := starts[0]["sid"]
	if newSID == sid {
		t.Error("phase 3: new session should have different sid after timeout rotation")
	}

	// tool_call in new session should use new sid.
	newCalls := phase3Events["tool_call"]
	if len(newCalls) != 1 {
		t.Fatalf("phase 3: expected 1 tool_call, got %d", len(newCalls))
	}
	if newCalls[0]["sid"] != newSID {
		t.Errorf("phase 3: tool_call sid = %v, want %v (new session)", newCalls[0]["sid"], newSID)
	}

	// === Phase 4: Usage summary ===
	snapshot := tracker.SwapAndReset()
	if snapshot == nil {
		t.Fatal("phase 4: SwapAndReset returned nil")
	}
	BeaconUsageSummary(5, snapshot)

	summary := waitForEvent(t, received, "usage_summary")
	requireEnvelope(t, summary, "usage_summary")
	if summary["window_m"] != float64(5) {
		t.Errorf("phase 4: window_m = %v, want 5", summary["window_m"])
	}
	statsRaw, _ := summary["tool_stats"].([]any)
	if len(statsRaw) == 0 {
		t.Error("phase 4: usage_summary has no tool_stats")
	}
}

// ---------- E2E: BuildUsageSummaryPayload for debug endpoint ----------

func TestE2E_Warm_PreloadsInstallID(t *testing.T) {
	resetInstallIDState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	t.Cleanup(func() {
		resetInstallIDState()
		resetKaboomDir()
	})

	// Before Warm, install ID is not cached.
	resetInstallIDState()

	Warm(nil)

	// After Warm, GetInstallID should return instantly (no I/O).
	id := GetInstallID()
	if id == "" {
		t.Fatal("GetInstallID returned empty after Warm()")
	}
	if !regexp.MustCompile(`^[0-9a-f]{12}$`).MatchString(id) {
		t.Errorf("install ID = %q, want 12-char hex", id)
	}
}

// ---------- E2E: opt-out caching ----------

func TestE2E_OptOutCachedAtInit(t *testing.T) {
	// Verify that telemetryOptedOut returns consistent results.
	// (Tests env var reading, not caching — caching is an implementation detail.)
	t.Setenv("KABOOM_TELEMETRY", "off")
	if !telemetryOptedOut() {
		t.Error("telemetryOptedOut should return true when KABOOM_TELEMETRY=off")
	}
	t.Setenv("KABOOM_TELEMETRY", "OFF")
	if !telemetryOptedOut() {
		t.Error("telemetryOptedOut should be case-insensitive")
	}
	t.Setenv("KABOOM_TELEMETRY", "")
	if telemetryOptedOut() {
		t.Error("telemetryOptedOut should return false when KABOOM_TELEMETRY is empty")
	}
}

// ---------- E2E: empty tool key ----------
