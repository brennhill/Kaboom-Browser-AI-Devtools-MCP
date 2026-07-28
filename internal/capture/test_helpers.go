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
func (c *Capture) AddNetworkBodiesForTest(bodies []types.NetworkBody) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for _, body := range bodies {
		c.buffers.networkBodies = append(c.buffers.networkBodies, networkBodyEntry{
			Body:    body,
			AddedAt: now,
		})
		c.buffers.networkTotalAdded++
		if body.Status >= 400 {
			c.buffers.networkErrorTotalAdded++
		}
	}
}

// AddWebSocketEventsForTest adds WebSocket events directly to the buffer (TEST ONLY)
func (c *Capture) AddWebSocketEventsForTest(events []types.WebSocketEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for _, event := range events {
		c.buffers.wsEvents = append(c.buffers.wsEvents, wsEventEntry{
			Event:   event,
			AddedAt: now,
		})
		c.buffers.wsTotalAdded++
	}
}

// AddEnhancedActionsForTest adds enhanced actions directly to the buffer (TEST ONLY)
func (c *Capture) AddEnhancedActionsForTest(actions []types.EnhancedAction) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for _, action := range actions {
		c.buffers.enhancedActions = append(c.buffers.enhancedActions, enhancedActionEntry{
			Action:  action,
			AddedAt: now,
		})
		c.buffers.actionTotalAdded++
	}
}

// SetPilotEnabled sets the pilot enabled state (TEST ONLY)
func (c *Capture) SetPilotEnabled(enabled bool) {
	c.extension.mu.Lock()
	defer c.extension.mu.Unlock()
	c.extension.state.pilotEnabled = enabled
	c.extension.state.pilotStatusKnown = true
	c.extension.state.pilotUpdatedAt = time.Now()
	c.extension.state.pilotSource = PilotSourceTestHelper
}

// SetPilotUnknownForTest resets pilot to startup-uncertain state (TEST ONLY).
func (c *Capture) SetPilotUnknownForTest() {
	c.extension.mu.Lock()
	defer c.extension.mu.Unlock()
	c.extension.state.pilotEnabled = false
	c.extension.state.pilotStatusKnown = false
	c.extension.state.pilotUpdatedAt = time.Time{}
	c.extension.state.pilotSource = PilotSourceAssumedStartup
}

// SetTrackingStatusForTest sets the tracked tab URL and ID (TEST ONLY)
func (c *Capture) SetTrackingStatusForTest(tabID int, tabURL string) {
	c.extension.mu.Lock()
	defer c.extension.mu.Unlock()
	c.extension.state.trackingEnabled = true
	c.extension.state.trackedTabID = tabID
	c.extension.state.trackedTabURL = tabURL
	c.extension.state.trackingUpdated = time.Now()
}

// SetClientRegistryForTest sets the client registry (TEST ONLY)
func (c *Capture) SetClientRegistryForTest(reg ClientRegistry) {
	c.SetClientRegistry(reg)
}

// AddExtraWSEventsForTest adds extra WebSocket event entries to the buffer (TEST ONLY).
// This replaces SetWSParallelMismatchForTest since parallel arrays no longer exist.
func (c *Capture) AddExtraWSEventsForTest(count int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for i := 0; i < count; i++ {
		c.buffers.wsEvents = append(c.buffers.wsEvents, wsEventEntry{
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
func (c *Capture) GetWSLengthsForTest() (events int, addedAt int, memoryTotal int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	n := len(c.buffers.wsEvents)
	return n, n, c.buffers.wsMemoryTotal
}

// SimulateExtensionConnectForTest marks the extension as connected by
// setting lastSyncSeen to now. Thread-safe (operates on the instance, not a global).
func (c *Capture) SimulateExtensionConnectForTest() {
	c.extension.mu.Lock()
	defer c.extension.mu.Unlock()
	c.extension.state.lastSyncSeen = time.Now()
	c.extension.state.lastExtensionConnected = true
}

// SimulateExtensionDisconnectForTest marks the extension as disconnected by
// setting lastSyncSeen far in the past. Thread-safe (operates on the instance, not a global).
func (c *Capture) SimulateExtensionDisconnectForTest() {
	c.extension.mu.Lock()
	defer c.extension.mu.Unlock()
	c.extension.state.lastSyncSeen = time.Now().Add(-1 * time.Hour)
}

// SetTabStatusForTest sets the tracked tab status (TEST ONLY).
// Valid values: "loading", "complete".
func (c *Capture) SetTabStatusForTest(status string) {
	c.extension.mu.Lock()
	defer c.extension.mu.Unlock()
	c.extension.state.tabStatus = status
}

// SetCSPStatusForTest sets the CSP restriction state (TEST ONLY)
func (c *Capture) SetCSPStatusForTest(restricted bool, level string) {
	c.extension.mu.Lock()
	defer c.extension.mu.Unlock()
	c.extension.state.cspRestricted = restricted
	c.extension.state.cspLevel = level
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
		c.emitLifecycleEvent(lifecycle.EventExtensionConnected, map[string]any{
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
