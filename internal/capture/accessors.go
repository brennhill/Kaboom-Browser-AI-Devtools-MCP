// Purpose: Provides the thread-safe read accessors over buffered counters, timestamps, events and performance snapshots.
// Why: One lock-taking read layer over the stores, rather than four files split by which counter they return.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
)

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
	extensionSnap := c.extension.Snapshot()

	c.mu.RLock()
	defer c.mu.RUnlock()

	return HealthSnapshot{
		WebSocketCount:        c.buffers.webSocketCount(),
		NetworkBodyCount:      c.buffers.networkCount(),
		ActionCount:           c.buffers.actionCount(),
		ConnectionCount:       c.wsConnections.Count(),
		LastPollTime:          extensionSnap.LastPollAt,
		ExtSessionID:          extensionSnap.ExtSessionID,
		ExtSessionChangedTime: extensionSnap.ExtSessionChangedAt,
		PilotEnabled:          extensionSnap.PilotEnabled,
		CircuitOpen:           circuitOpen,
		WindowEventCount:      windowEventCount,
		CircuitReason:         circuitReason,
		CircuitOpenedTime:     circuitOpenedAt,
		PendingQueryCount:     querySnap.PendingQueryCount,
		QueryResultCount:      querySnap.QueryResultCount,
		ActiveTestIDCount:     extensionSnap.ActiveTestIDCount,
		QueryTimeout:          querySnap.QueryTimeout,
	}
}

// GetNetworkBodies returns a copy of the network bodies slice (thread-safe)
func (c *Capture) GetNetworkBodies() []types.NetworkBody {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.networkBodiesCopy()
}

// GetAllWebSocketEvents returns a copy of all WebSocket events slice (thread-safe)
func (c *Capture) GetAllWebSocketEvents() []types.WebSocketEvent {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.webSocketEventsCopy()
}

// GetAllEnhancedActions returns a copy of all enhanced actions slice (thread-safe)
func (c *Capture) GetAllEnhancedActions() []types.EnhancedAction {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.enhancedActionsCopy()
}

// Performance returns the independently synchronized performance owner.
func (c *Capture) Performance() *PerformanceStore {
	return c.perf
}

// Add stores URL-keyed performance snapshots with bounded oldest-entry eviction.
func (s *PerformanceStore) Add(snapshots []performance.PerformanceSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.appendSnapshots(snapshots)
}

// Entries returns a detached list of stored performance snapshots.
func (s *PerformanceStore) Entries() []performance.PerformanceSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshotsList()
}

// ByURL returns the performance snapshot stored for a URL.
func (s *PerformanceStore) ByURL(url string) (performance.PerformanceSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshotByURL(url)
}

// StoreBefore stores a bounded pre-action snapshot for later performance diffing.
func (s *PerformanceStore) StoreBefore(correlationID string, snapshot performance.PerformanceSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.storeBeforeSnapshot(correlationID, snapshot)
}

// TakeBefore consumes the pre-action snapshot for a correlation ID.
func (s *PerformanceStore) TakeBefore(correlationID string) (performance.PerformanceSnapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.takeBeforeSnapshot(correlationID)
}

const (
	maxPerformanceSnapshots = 100
	maxBeforeSnapshots      = 50
)

// appendSnapshots stores snapshots by URL with oldest-entry eviction.
func (s *PerformanceStore) appendSnapshots(snapshots []performance.PerformanceSnapshot) {
	for _, snapshot := range snapshots {
		key := snapshot.URL
		if key == "" {
			continue
		}

		if _, exists := s.snapshots[key]; !exists {
			s.snapshotOrder = append(s.snapshotOrder, key)
		}
		s.snapshots[key] = snapshot

		for len(s.snapshots) > maxPerformanceSnapshots && len(s.snapshotOrder) > 0 {
			oldestKey := s.snapshotOrder[0]
			s.snapshotOrder = s.snapshotOrder[1:]
			delete(s.snapshots, oldestKey)
		}
	}
}

// snapshotsList returns a detached list copy.
func (s *PerformanceStore) snapshotsList() []performance.PerformanceSnapshot {
	if len(s.snapshots) == 0 {
		return []performance.PerformanceSnapshot{}
	}
	out := make([]performance.PerformanceSnapshot, 0, len(s.snapshots))
	for _, snapshot := range s.snapshots {
		out = append(out, snapshot)
	}
	return out
}

// snapshotByURL returns one snapshot by URL key.
func (s *PerformanceStore) snapshotByURL(url string) (performance.PerformanceSnapshot, bool) {
	snap, ok := s.snapshots[url]
	return snap, ok
}

// storeBeforeSnapshot keeps a pre-action snapshot for perf diff correlation.
func (s *PerformanceStore) storeBeforeSnapshot(correlationID string, snapshot performance.PerformanceSnapshot) {
	s.beforeSnapshots[correlationID] = snapshot
	if len(s.beforeSnapshots) <= maxBeforeSnapshots {
		return
	}

	// Preserve current semantics: remove an arbitrary key when over cap.
	for key := range s.beforeSnapshots {
		delete(s.beforeSnapshots, key)
		break
	}
}

// takeBeforeSnapshot retrieves and deletes a before-snapshot (consume-on-read).
func (s *PerformanceStore) takeBeforeSnapshot(correlationID string) (performance.PerformanceSnapshot, bool) {
	snap, ok := s.beforeSnapshots[correlationID]
	if ok {
		delete(s.beforeSnapshots, correlationID)
	}
	return snap, ok
}

// clear resets performance snapshot/baseline/before-snapshot state.
func (s *PerformanceStore) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots = make(map[string]performance.PerformanceSnapshot)
	s.snapshotOrder = make([]string, 0)
	s.beforeSnapshots = make(map[string]performance.PerformanceSnapshot)
}
