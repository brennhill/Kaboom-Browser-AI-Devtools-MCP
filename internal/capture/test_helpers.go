// Purpose: Provides test-only capture mutation helpers and threshold overrides for deterministic setup.
// Why: Enables focused tests without exposing unsafe mutation primitives in production APIs.
// Docs: docs/features/feature/self-testing/index.md

package capture

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle"
)

// AddNetworkBodiesForTest adds network bodies directly to the buffer (TEST ONLY)
// Normal production code should use HTTP handlers
func (s *TelemetryStore) AddNetworkBodiesForTest(bodies []types.NetworkBody) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, body := range bodies {
		s.buffers.networkBodies = append(s.buffers.networkBodies, networkBodyEntry{
			Body:    body,
			AddedAt: now,
		})
		s.buffers.networkTotalAdded++
		if body.Status >= 400 {
			s.buffers.networkErrorTotalAdded++
		}
	}
}

// AddWebSocketEventsForTest adds WebSocket events directly to the buffer (TEST ONLY)
func (s *TelemetryStore) AddWebSocketEventsForTest(events []types.WebSocketEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, event := range events {
		s.buffers.wsEvents = append(s.buffers.wsEvents, wsEventEntry{
			Event:   event,
			AddedAt: now,
		})
		s.buffers.wsTotalAdded++
	}
}

// AddEnhancedActionsForTest adds enhanced actions directly to the buffer (TEST ONLY)
func (s *TelemetryStore) AddEnhancedActionsForTest(actions []types.EnhancedAction) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, action := range actions {
		s.buffers.enhancedActions = append(s.buffers.enhancedActions, enhancedActionEntry{
			Action:  action,
			AddedAt: now,
		})
		s.buffers.actionTotalAdded++
	}
}

// SetPilotEnabled sets the pilot enabled state (TEST ONLY)
func (r *ExtensionRuntime) SetPilotEnabled(enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.pilotEnabled = enabled
	r.state.pilotStatusKnown = true
	r.state.pilotUpdatedAt = time.Now()
	r.state.pilotSource = PilotSourceTestHelper
}

// SetPilotUnknownForTest resets pilot to startup-uncertain state (TEST ONLY).
func (r *ExtensionRuntime) SetPilotUnknownForTest() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.pilotEnabled = false
	r.state.pilotStatusKnown = false
	r.state.pilotUpdatedAt = time.Time{}
	r.state.pilotSource = PilotSourceAssumedStartup
}

// SetTrackingStatusForTest sets the tracked tab URL and ID (TEST ONLY)
func (r *ExtensionRuntime) SetTrackingStatusForTest(tabID int, tabURL string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.trackingEnabled = true
	r.state.trackedTabID = tabID
	r.state.trackedTabURL = tabURL
	r.state.trackingUpdated = time.Now()
}

// SetClientRegistryForTest sets the client registry (TEST ONLY)
func (c *Capture) SetClientRegistryForTest(reg ClientRegistry) {
	c.SetClientRegistry(reg)
}

// AddExtraWSEventsForTest adds extra WebSocket event entries to the buffer (TEST ONLY).
// This replaces SetWSParallelMismatchForTest since parallel arrays no longer exist.
func (s *TelemetryStore) AddExtraWSEventsForTest(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for i := 0; i < count; i++ {
		s.buffers.wsEvents = append(s.buffers.wsEvents, wsEventEntry{
			Event: types.WebSocketEvent{
				Event: "message",
				Data:  "extra-event",
				ID:    "ws-extra",
			},
			AddedAt: now,
		})
	}
}

// GetWSLengthsForTest returns wsEvents count and memory total (TEST ONLY).
// The addedAt return value always equals events since timestamps are embedded in entries.
func (s *TelemetryStore) GetWSLengthsForTest() (events int, addedAt int, memoryTotal int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := len(s.buffers.wsEvents)
	return n, n, s.buffers.wsMemoryTotal
}

// SimulateExtensionConnectForTest marks the extension as connected by
// setting lastSyncSeen to now. Thread-safe (operates on the instance, not a global).
func (r *ExtensionRuntime) SimulateExtensionConnectForTest() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.lastSyncSeen = time.Now()
	r.state.lastExtensionConnected = true
}

// SimulateExtensionDisconnectForTest marks the extension as disconnected by
// setting lastSyncSeen far in the past. Thread-safe (operates on the instance, not a global).
func (r *ExtensionRuntime) SimulateExtensionDisconnectForTest() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.lastSyncSeen = time.Now().Add(-1 * time.Hour)
}

// SetTabStatusForTest sets the tracked tab status (TEST ONLY).
// Valid values: "loading", "complete".
func (r *ExtensionRuntime) SetTabStatusForTest(status string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.tabStatus = status
}

// SetCSPStatusForTest sets the CSP restriction state (TEST ONLY)
func (r *ExtensionRuntime) SetCSPStatusForTest(restricted bool, level string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.cspRestricted = restricted
	r.state.cspLevel = level
}

// SimulateSyncForTest simulates a /sync connection from the extension,
// triggering lifecycle callbacks (extension_connected) like a real sync would.
// This is faster than calling HandleSync because it avoids the 5-second long-poll.
// Thread-safe (TEST ONLY).
func (c *Capture) SimulateSyncForTest(extSessionID string, clientID string) {
	now := time.Now()
	req := SyncRequest{
		ExtSessionID: extSessionID,
		Settings: &SyncSettings{
			PilotEnabled:    false,
			TrackingEnabled: true,
			TrackedTabID:    1,
		},
	}
	state := c.extension.updateSyncConnectionState(req, clientID, now)

	if !state.wasConnected || state.isReconnect {
		c.Lifecycle().Emit(lifecycle.EventExtensionConnected, map[string]any{
			"ext_session_id":     state.extSessionID,
			"is_reconnect":       state.isReconnect,
			"disconnect_seconds": state.timeSinceLastPoll.Seconds(),
		})
	}
}

// SetExtensionDisconnectThresholdForTesting overrides the disconnect threshold and
// returns a restore function for test cleanup.
// NOTE: Tests that mutate this var must NOT use t.Parallel() since it is a
// package-level variable shared across all tests in the package.
func SetExtensionDisconnectThresholdForTesting(d time.Duration) func() {
	prev := extensionDisconnectThreshold
	extensionDisconnectThreshold = d
	return func() {
		extensionDisconnectThreshold = prev
	}
}
