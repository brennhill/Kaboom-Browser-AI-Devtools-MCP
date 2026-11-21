// Purpose: Unit tests for extension state logic, including the tracked-tab projection ordering rule.
// Docs: docs/features/feature/backend-log-streaming/index.md

package syncruntime

import (
	"context"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/browsercommand/commandtest"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func mutateExtensionStateForTest(runtime *Runtime, mutate func(*runtimeState)) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	mutate(&runtime.state)
}

func extensionStateSnapshotForTest(runtime *Runtime) runtimeState {
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	return runtime.state
}

func applySettingsForTest(state *testState, settings SyncSettings) {
	state.runtime.updateSyncConnectionState(SyncRequest{ExtSessionID: "capture-package-test", Settings: &settings}, "capture-package-test", time.Now())
}

func currentSettingsForTest(state *testState) SyncSettings {
	extension := state.Extension()
	tracking, tabID, tabURL := extension.GetTrackingStatus()
	active, activeKnown := extension.IsTrackedTabActive()
	var activeValue *bool
	if activeKnown {
		activeValue = &active
	}
	restricted, level := extension.GetCSPStatus()
	return SyncSettings{PilotEnabled: extension.IsPilotEnabled(), TrackingEnabled: tracking, TrackedTabID: tabID,
		TrackedTabURL: tabURL, TrackedTabTitle: extension.GetTrackedTabTitle(), TabStatus: extension.GetTabStatus(),
		TrackedTabActive: activeValue, CspRestricted: restricted, CspLevel: level}
}

func connectForTest(state *testState) {
	state.runtime.updateSyncConnectionState(SyncRequest{ExtSessionID: "capture-package-test"}, "capture-package-test", time.Now())
}

func setPilotForTest(state *testState, enabled bool) {
	state.Extension().ApplyCachedPilot(enabled, time.Now())
}

func trackForTest(state *testState, tabID int, tabURL string) {
	state.Extension().UpdateTrackedTab(tabID, tabURL, "")
}

// ============================================
// WaitForExtensionConnected tests (issue #302)
// ============================================

func TestWaitForExtensionConnected_AlreadyConnected(t *testing.T) {
	t.Parallel()
	c := newTestState()

	// Simulate extension already connected.
	connectForTest(c)

	if !c.Extension().WaitForExtensionConnected(context.Background(), 5*time.Second) {
		t.Fatal("WaitForExtensionConnected returned false when extension already connected")
	}
}

func TestWaitForExtensionConnected_Timeout(t *testing.T) {
	t.Parallel()
	c := newTestState()
	// Extension never connects.

	if c.Extension().WaitForExtensionConnected(context.Background(), 100*time.Millisecond) {
		t.Fatal("WaitForExtensionConnected returned true; expected false after timeout")
	}
}

func TestMarkDisconnectedPreservesAuthoritativeSettings(t *testing.T) {
	t.Parallel()
	c := newTestState()
	c.runtime.updateSyncConnectionState(SyncRequest{Settings: &SyncSettings{
		PilotEnabled:    true,
		TrackingEnabled: true,
		TrackedTabID:    42,
		TrackedTabURL:   "https://example.test",
	}}, "client", time.Now())
	lastSeen := c.Extension().GetExtensionStatus()["last_seen"]

	c.Extension().MarkDisconnected()

	if c.Extension().IsExtensionConnected() {
		t.Fatal("extension remained connected")
	}
	status := c.Extension().GetExtensionStatus()
	if status["last_seen"] != lastSeen || status["last_seen"] == "" {
		t.Fatalf("disconnect discarded last-seen evidence: before=%v after=%v", lastSeen, status["last_seen"])
	}
	if !c.Extension().IsPilotEnabled() {
		t.Fatal("disconnect discarded authoritative pilot setting")
	}
	tracked, tabID, _ := c.Extension().GetTrackingStatus()
	if !tracked || tabID != 42 {
		t.Fatalf("disconnect discarded tracking state: (%v, %d)", tracked, tabID)
	}
}

// TestWaitForExtensionConnected_ContextCancelled lives in readiness_gate_test.go
// with full timing bounds checks (P1-1).

func TestCaptureTestHelpersAndTTL(t *testing.T) {
	t.Parallel()

	c := newTestState()

	c.Telemetry().AddNetworkBodies([]types.NetworkBody{
		{URL: "https://example.test/a", Status: 200},
		{URL: "https://example.test/b", Status: 500},
	})
	c.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{Event: "open", URL: "wss://example.test"},
	})
	c.Telemetry().AddEnhancedActions([]types.EnhancedAction{
		{Type: "click", URL: "https://example.test", Timestamp: 123},
	})
	if got := c.Telemetry().NetworkBodies().Stats().TotalAdded; got != 2 {
		t.Fatalf("GetNetworkTotalAdded() = %d, want 2", got)
	}
	if got := c.Telemetry().WebSockets().Stats().TotalAdded; got != 1 {
		t.Fatalf("GetWebSocketTotalAdded() = %d, want 1", got)
	}
	if got := c.Telemetry().Actions().Stats().TotalAdded; got != 1 {
		t.Fatalf("GetActionTotalAdded() = %d, want 1", got)
	}

	setPilotForTest(c, true)
	if !c.Extension().IsPilotEnabled() {
		t.Fatal("SetPilotEnabled(true) did not update state")
	}
	trackForTest(c, 77, "https://tracked.test")
	enabled, tabID, tabURL := c.Extension().GetTrackingStatus()
	if !enabled || tabID != 77 || tabURL != "https://tracked.test" {
		t.Fatalf("tracking state = (%v,%d,%q), want (true,77,https://tracked.test)", enabled, tabID, tabURL)
	}

	if pending := c.Commands().Pending(); len(pending) != 0 {
		t.Fatalf("GetPendingQueries() = %+v, want none before adding pending query", pending)
	}
	commandtest.Enqueue(c.Commands(), queries.PendingQuery{Type: "query_dom", Params: []byte(`{"selector":".x"}`)})
	commandtest.Enqueue(c.Commands(), queries.PendingQuery{Type: "accessibility", Params: []byte(`{"scope":"page"}`)})
	pending := c.Commands().Pending()
	if len(pending) != 2 || pending[len(pending)-1].Type != "accessibility" {
		t.Fatalf("pending queries = %+v, want accessibility query last", pending)
	}
}

// Tracked-tab projection ordering rule (tracking_projection.go).

func heartbeat(tabID int, url string) SyncRequest {
	settings := &SyncSettings{PilotEnabled: true, TrackingEnabled: tabID > 0, TrackedTabID: tabID, TrackedTabURL: url, TabStatus: "complete"}
	return SyncRequest{ExtSessionID: "ext-1", Settings: settings}
}

func assertTracked(t *testing.T, r *Runtime, wantID int, wantURL string) {
	t.Helper()
	enabled, id, url := r.GetTrackingStatus()
	if !enabled || id != wantID || url != wantURL {
		t.Fatalf("tracking = (%v, %d, %q), want (true, %d, %q)", enabled, id, url, wantID, wantURL)
	}
}

// Synchronous switch_tab feedback is visible immediately, before any heartbeat.
func TestTrackingProjection_SynchronousFeedbackIsImmediate(t *testing.T) {
	r := New()
	r.updateSyncConnectionState(heartbeat(1, "https://a.test/"), "c", time.Unix(100, 0))
	r.UpdateTrackedTab(2, "https://b.test/", "B")
	assertTracked(t, r, 2, "https://b.test/")
}

// A heartbeat snapshotted before the switch (reporting tab A, possibly with a
// late navigation URL) must not overwrite the confirmed tab B.
func TestTrackingProjection_LateTabAHeartbeatCannotOverwriteSwitch(t *testing.T) {
	r := New()
	r.updateSyncConnectionState(heartbeat(1, "https://a.test/"), "c", time.Unix(100, 0))
	r.UpdateTrackedTab(2, "https://b.test/", "B")

	r.updateSyncConnectionState(heartbeat(1, "https://a.test/late-nav"), "c", time.Unix(101, 0))
	assertTracked(t, r, 2, "https://b.test/")
	if !r.IsPilotEnabled() {
		t.Fatal("pilot intent from a held heartbeat must still be applied")
	}
}

// Once the extension reports the confirmed tab, its later details (navigation
// on B) apply normally and the hold is released.
func TestTrackingProjection_BackgroundConvergence(t *testing.T) {
	r := New()
	r.updateSyncConnectionState(heartbeat(1, "https://a.test/"), "c", time.Unix(100, 0))
	r.UpdateTrackedTab(2, "https://b.test/", "B")
	r.updateSyncConnectionState(heartbeat(1, "https://a.test/late"), "c", time.Unix(101, 0))
	r.updateSyncConnectionState(heartbeat(2, "https://b.test/next"), "c", time.Unix(102, 0))
	assertTracked(t, r, 2, "https://b.test/next")

	// Hold released: a genuine later selection (user picks tab 3) applies at once.
	r.updateSyncConnectionState(heartbeat(3, "https://c.test/"), "c", time.Unix(103, 0))
	assertTracked(t, r, 3, "https://c.test/")
}

// Background (async) switch_tab has no synchronous result; the heartbeat alone
// converges the projection under the same rule.
func TestTrackingProjection_BackgroundSwitchConvergesViaHeartbeat(t *testing.T) {
	r := New()
	r.updateSyncConnectionState(heartbeat(1, "https://a.test/"), "c", time.Unix(100, 0))
	r.updateSyncConnectionState(heartbeat(2, "https://b.test/"), "c", time.Unix(101, 0))
	assertTracked(t, r, 2, "https://b.test/")
}

// The extension stays authoritative: if it keeps disagreeing past the bounded
// stale allowance (user re-selected, or B closed), the daemon converges to it.
func TestTrackingProjection_ExtensionWinsAfterStaleAllowance(t *testing.T) {
	r := New()
	r.UpdateTrackedTab(2, "https://b.test/", "B")
	for i := 0; i < staleHeartbeatAllowance; i++ {
		r.updateSyncConnectionState(heartbeat(1, "https://a.test/"), "c", time.Unix(int64(100+i), 0))
		assertTracked(t, r, 2, "https://b.test/")
	}
	r.updateSyncConnectionState(heartbeat(1, "https://a.test/"), "c", time.Unix(200, 0))
	assertTracked(t, r, 1, "https://a.test/")
}

// A held heartbeat does not apply tab-scoped status (it describes tab A).
func TestTrackingProjection_HeldHeartbeatKeepsTabScopedFields(t *testing.T) {
	r := New()
	r.UpdateTrackedTab(2, "https://b.test/", "B")
	req := heartbeat(1, "https://a.test/")
	req.Settings.TabStatus = "loading"
	req.Settings.CspRestricted = true
	req.Settings.CspLevel = "page_blocked"
	r.updateSyncConnectionState(req, "c", time.Unix(100, 0))
	if got := r.GetTabStatus(); got == "loading" {
		t.Fatalf("tab status from stale tab A applied: %q", got)
	}
	if restricted, _ := r.GetCSPStatus(); restricted {
		t.Fatal("CSP posture from stale tab A applied")
	}
}
