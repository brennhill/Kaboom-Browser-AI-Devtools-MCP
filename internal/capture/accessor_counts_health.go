// Purpose: Provides thread-safe accessors for monotonic network, WebSocket, and action counters.
// Why: Separates health/count accessors from event and performance accessors to limit file scope.
package capture

import "time"

// GetNetworkTotalAdded returns the monotonic total of network bodies ever added
func (c *Capture) GetNetworkTotalAdded() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.networkTotal()
}

// GetNetworkErrorTotalAdded returns the monotonic total of error network bodies ever added.
func (c *Capture) GetNetworkErrorTotalAdded() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.networkErrorTotal()
}

// GetWebSocketTotalAdded returns the monotonic total of WebSocket events ever added
func (c *Capture) GetWebSocketTotalAdded() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.webSocketTotal()
}

// GetActionTotalAdded returns the monotonic total of actions ever added
func (c *Capture) GetActionTotalAdded() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.actionTotal()
}

// CaptureSnapshot is an immutable point-in-time view of core ring-buffer counters.
//
// Invariants:
// - Counts and totals in one snapshot come from the same c.mu critical section.
type CaptureSnapshot struct {
	NetworkTotalAdded   int64
	WebSocketTotalAdded int64
	ActionTotalAdded    int64
	NetworkCount        int
	WebSocketCount      int
	ActionCount         int
}

// GetSnapshot returns a thread-safe capture counter snapshot.
//
// Failure semantics:
// - Snapshot can be stale immediately after return; callers should treat it as diagnostic-only.
func (c *Capture) GetSnapshot() CaptureSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CaptureSnapshot{
		NetworkTotalAdded:   c.buffers.networkTotal(),
		WebSocketTotalAdded: c.buffers.webSocketTotal(),
		ActionTotalAdded:    c.buffers.actionTotal(),
		NetworkCount:        c.buffers.networkCount(),
		WebSocketCount:      c.buffers.webSocketCount(),
		ActionCount:         c.buffers.actionCount(),
	}
}

// GetClientRegistry returns the client registry (thread-safe)
func (c *Capture) GetClientRegistry() ClientRegistry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.clientRegistry
}

// HealthSnapshot aggregates capture + dispatcher + circuit health state.
//
// Invariants:
// - Subsystem snapshots (circuit/queries) are sampled before c.mu to avoid lock inversion.
type HealthSnapshot struct {
	WebSocketCount        int
	NetworkBodyCount      int
	ActionCount           int
	ConnectionCount       int
	LastPollTime          time.Time
	ExtSessionID          string
	ExtSessionChangedTime time.Time
	PilotEnabled          bool
	CircuitOpen           bool
	WindowEventCount      int
	CircuitReason         string
	CircuitOpenedTime     time.Time
	PendingQueryCount     int
	QueryResultCount      int
	ActiveTestIDCount     int
	QueryTimeout          time.Duration
}

// GetHealthSnapshot returns a lock-safe aggregate health view.
//
// Invariants:
// - Reads c.circuit/c.queryDispatcher first, then c.mu, preserving declared lock hierarchy.
func (c *Capture) GetHealthSnapshot() HealthSnapshot {
	// Get sub-struct state (own locks) before acquiring c.mu
	circuitOpen, circuitReason, circuitOpenedAt, windowEventCount := c.circuit.GetState()
	querySnap := c.queryDispatcher.GetSnapshot()

	c.mu.RLock()
	defer c.mu.RUnlock()

	return HealthSnapshot{
		WebSocketCount:        c.buffers.webSocketCount(),
		NetworkBodyCount:      c.buffers.networkCount(),
		ActionCount:           c.buffers.actionCount(),
		ConnectionCount:       c.wsConnections.Count(),
		LastPollTime:          c.extensionState.lastPollAt,
		ExtSessionID:          c.extensionState.extSessionID,
		ExtSessionChangedTime: c.extensionState.extSessionChangedAt,
		PilotEnabled:          c.extensionState.pilotEnabled,
		CircuitOpen:           circuitOpen,
		WindowEventCount:      windowEventCount,
		CircuitReason:         circuitReason,
		CircuitOpenedTime:     circuitOpenedAt,
		PendingQueryCount:     querySnap.PendingQueryCount,
		QueryResultCount:      querySnap.QueryResultCount,
		ActiveTestIDCount:     len(c.extensionState.activeTestIDs),
		QueryTimeout:          querySnap.QueryTimeout,
	}
}
