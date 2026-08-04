// sync_test.go — Verifies capture fixtures exercise the canonical sync boundary.
package capturefixture

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

func TestSyncAppliesAuthoritativeExtensionState(t *testing.T) {
	t.Parallel()
	state := capture.NewCapture()
	active := true

	Sync(state, capture.SyncSettings{
		PilotEnabled:     true,
		TrackingEnabled:  true,
		TrackedTabID:     42,
		TrackedTabURL:    "https://example.test",
		TabStatus:        "complete",
		TrackedTabActive: &active,
		CspRestricted:    true,
		CspLevel:         "script_exec",
	})

	if !state.Extension().IsPilotEnabled() {
		t.Fatal("pilot state was not applied through sync")
	}
	tracked, tabID, tabURL := state.Extension().GetTrackingStatus()
	if !tracked || tabID != 42 || tabURL != "https://example.test" {
		t.Fatalf("tracking state = (%v, %d, %q)", tracked, tabID, tabURL)
	}
	if got := state.Extension().GetTabStatus(); got != "complete" {
		t.Fatalf("tab status = %q", got)
	}
	if got, known := state.Extension().IsTrackedTabActive(); !known || !got {
		t.Fatalf("active state = (%v, %v)", got, known)
	}
	if restricted, level := state.Extension().GetCSPStatus(); !restricted || level != "script_exec" {
		t.Fatalf("CSP state = (%v, %q)", restricted, level)
	}
	if !state.Extension().IsExtensionConnected() {
		t.Fatal("sync did not mark extension connected")
	}
}

func TestCachedPilotAndTrackedTabFixturesDoNotFabricateConnection(t *testing.T) {
	t.Parallel()
	state := capture.NewCapture()

	SetPilot(state, true)
	Track(state, 42, "https://example.test")

	if state.Extension().IsExtensionConnected() {
		t.Fatal("non-transport fixtures fabricated an extension heartbeat")
	}
	if !state.Extension().IsPilotEnabled() {
		t.Fatal("cached pilot fixture was not applied")
	}
	tracked, tabID, _ := state.Extension().GetTrackingStatus()
	if !tracked || tabID != 42 {
		t.Fatalf("tracked-tab fixture = (%v, %d)", tracked, tabID)
	}
}
