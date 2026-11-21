// tracking_projection_test.go — Ordering-rule tests for the daemon tracked-tab projection.

package syncruntime

import (
	"testing"
	"time"
)

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
