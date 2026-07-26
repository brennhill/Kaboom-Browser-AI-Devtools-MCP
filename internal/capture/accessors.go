// Purpose: Provides the thread-safe read accessors over buffered counters, timestamps, events and performance snapshots.
// Why: One lock-taking read layer over the stores, rather than four files split by which counter they return.
// Docs: docs/features/feature/backend-log-streaming/index.md

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

// GetNetworkTimestamps returns a copy of the network body timestamps
func (c *Capture) GetNetworkTimestamps() []time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.networkTimestamps()
}

// GetWebSocketTimestamps returns a copy of the WebSocket event timestamps
func (c *Capture) GetWebSocketTimestamps() []time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.webSocketTimestamps()
}

// GetActionTimestamps returns a copy of the action timestamps
func (c *Capture) GetActionTimestamps() []time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.actionTimestamps()
}

// GetNetworkBodies returns a copy of the network bodies slice (thread-safe)
func (c *Capture) GetNetworkBodies() []NetworkBody {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.networkBodiesCopy()
}

// GetAllWebSocketEvents returns a copy of all WebSocket events slice (thread-safe)
func (c *Capture) GetAllWebSocketEvents() []WebSocketEvent {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.webSocketEventsCopy()
}

// GetAllEnhancedActions returns a copy of all enhanced actions slice (thread-safe)
func (c *Capture) GetAllEnhancedActions() []EnhancedAction {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.enhancedActionsCopy()
}

// AddPerformanceSnapshots stores performance snapshots from the extension.
// Snapshots are keyed by URL with LRU eviction (max 100 entries).
func (c *Capture) AddPerformanceSnapshots(snapshots []PerformanceSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.perf.appendSnapshots(snapshots)
}

// GetPerformanceSnapshots returns all stored performance snapshots (thread-safe)
func (c *Capture) GetPerformanceSnapshots() []PerformanceSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.perf.snapshotsList()
}

// GetPerformanceSnapshotByURL returns a specific snapshot by URL key (thread-safe).
func (c *Capture) GetPerformanceSnapshotByURL(url string) (PerformanceSnapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.perf.snapshotByURL(url)
}

// StoreBeforeSnapshot stores a performance snapshot keyed by correlation_id
// for later perf_diff computation. Max 50 entries with oldest eviction.
func (c *Capture) StoreBeforeSnapshot(correlationID string, snapshot PerformanceSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.perf.storeBeforeSnapshot(correlationID, snapshot)
}

// GetAndDeleteBeforeSnapshot retrieves and removes a before-snapshot by correlation_id.
// Consume-on-read: the snapshot is deleted after retrieval to prevent memory leaks.
func (c *Capture) GetAndDeleteBeforeSnapshot(correlationID string) (PerformanceSnapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.perf.takeBeforeSnapshot(correlationID)
}
