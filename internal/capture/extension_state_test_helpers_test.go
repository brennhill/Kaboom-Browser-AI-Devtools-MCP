// extension_state_test_helpers_test.go — Lock-safe setup for extension-state tests.
package capture

import "time"

func mutateExtensionStateForTest(runtime *ExtensionRuntime, mutate func(*ExtensionState)) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	mutate(&runtime.state)
}

func extensionStateSnapshotForTest(runtime *ExtensionRuntime) ExtensionState {
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	return runtime.state
}

func applySettingsForTest(state *Capture, settings SyncSettings) {
	state.extension.updateSyncConnectionState(SyncRequest{
		ExtSessionID: "capture-package-test",
		Settings:     &settings,
	}, "capture-package-test", time.Now())
}

func currentSettingsForTest(state *Capture) SyncSettings {
	extension := state.Extension()
	tracking, tabID, tabURL := extension.GetTrackingStatus()
	active, activeKnown := extension.IsTrackedTabActive()
	var activeValue *bool
	if activeKnown {
		activeValue = &active
	}
	restricted, level := extension.GetCSPStatus()
	return SyncSettings{
		PilotEnabled:     extension.IsPilotEnabled(),
		TrackingEnabled:  tracking,
		TrackedTabID:     tabID,
		TrackedTabURL:    tabURL,
		TrackedTabTitle:  extension.GetTrackedTabTitle(),
		TabStatus:        extension.GetTabStatus(),
		TrackedTabActive: activeValue,
		CspRestricted:    restricted,
		CspLevel:         level,
	}
}

func connectForTest(state *Capture) {
	state.extension.updateSyncConnectionState(SyncRequest{
		ExtSessionID: "capture-package-test",
	}, "capture-package-test", time.Now())
}

func setPilotForTest(state *Capture, enabled bool) {
	state.Extension().ApplyCachedPilot(enabled, time.Now())
}

func trackForTest(state *Capture, tabID int, tabURL string) {
	state.Extension().UpdateTrackedTab(tabID, tabURL, "")
}
