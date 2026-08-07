// sync.go — Test fixtures that exercise capture's canonical extension sync boundary.
package capturefixture

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/commandcontract"
)

// Sync applies an authoritative extension settings snapshot through /sync.
// The canceled request context prevents the canonical handler from entering its
// long-poll wait after it has applied and validated the snapshot.
func Sync(state *capture.Capture, settings syncruntime.SyncSettings) {
	send(state, syncruntime.SyncRequest{
		ExtSessionID: "capture-fixture",
		Settings:     &settings,
	})
}

func send(state *capture.Capture, syncRequest syncruntime.SyncRequest) {
	payload, err := json.Marshal(syncRequest)
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodPost, "/sync", bytes.NewReader(payload)).WithContext(ctx)
	newSyncHandler(state).HandleSync(httptest.NewRecorder(), request)
}

func newSyncHandler(state *capture.Capture) *syncruntime.Handler {
	return syncruntime.NewHandler(syncruntime.Dependencies{
		Runtime:        state.Extension(),
		Queries:        state.Queries(),
		Lifecycle:      state.Lifecycle(),
		FeatureUsage:   state.FeatureUsage(),
		ExtensionLogs:  state.ExtensionLogs(),
		DiagnosticLogs: state.DiagnosticLogs(),
	})
}

func currentSettings(state *capture.Capture) syncruntime.SyncSettings {
	extension := state.Extension()
	tracking, tabID, tabURL := extension.GetTrackingStatus()
	active, activeKnown := extension.IsTrackedTabActive()
	var activeValue *bool
	if activeKnown {
		activeValue = &active
	}
	restricted, level := extension.GetCSPStatus()
	return syncruntime.SyncSettings{
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

// Connect records a current-build heartbeat without changing effective state.
func Connect(state *capture.Capture) { ConnectWithCommandContract(state, commandcontract.ID) }

// ConnectWithCommandContract models an extension build through the canonical
// sync boundary. It exists for deterministic same-version skew tests.
func ConnectWithCommandContract(state *capture.Capture, contractID string) {
	send(state, syncruntime.SyncRequest{ExtSessionID: "capture-fixture", CommandContractID: contractID})
}

// Disconnect records a transport loss without fabricating stale wall time.
func Disconnect(state *capture.Capture) { state.Extension().MarkDisconnected() }

// SetPilot applies an authoritative Pilot setting through extension sync.
func SetPilot(state *capture.Capture, enabled bool) {
	state.Extension().ApplyCachedPilot(enabled, time.Now())
}

// Track applies authoritative tracked-tab state through extension sync.
func Track(state *capture.Capture, tabID int, tabURL string) {
	state.Extension().UpdateTrackedTab(tabID, tabURL, "")
}

// SetTabStatus applies an authoritative Chrome tab lifecycle state.
func SetTabStatus(state *capture.Capture, status string) {
	settings := currentSettings(state)
	settings.TabStatus = status
	Sync(state, settings)
}

// SetCSP applies the extension's latest page CSP probe result.
func SetCSP(state *capture.Capture, restricted bool, level string) {
	settings := currentSettings(state)
	settings.CspRestricted = restricted
	settings.CspLevel = level
	Sync(state, settings)
}

// SetTrackedTabActive applies foreground-tab state through extension sync.
func SetTrackedTabActive(state *capture.Capture, active bool) {
	settings := currentSettings(state)
	settings.TrackedTabActive = &active
	Sync(state, settings)
}
