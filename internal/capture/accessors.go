// Purpose: Provides the thread-safe read accessors over buffered counters, timestamps, events and performance snapshots.
// Why: One lock-taking read layer over the stores, rather than four files split by which counter they return.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
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
func (c *Capture) AddPerformanceSnapshots(snapshots []performance.PerformanceSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.perf.appendSnapshots(snapshots)
}

// GetPerformanceSnapshots returns all stored performance snapshots (thread-safe)
func (c *Capture) GetPerformanceSnapshots() []performance.PerformanceSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.perf.snapshotsList()
}

// GetPerformanceSnapshotByURL returns a specific snapshot by URL key (thread-safe).
func (c *Capture) GetPerformanceSnapshotByURL(url string) (performance.PerformanceSnapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.perf.snapshotByURL(url)
}

// StoreBeforeSnapshot stores a performance snapshot keyed by correlation_id
// for later perf_diff computation. Max 50 entries with oldest eviction.
func (c *Capture) StoreBeforeSnapshot(correlationID string, snapshot performance.PerformanceSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.perf.storeBeforeSnapshot(correlationID, snapshot)
}

// GetAndDeleteBeforeSnapshot retrieves and removes a before-snapshot by correlation_id.
// Consume-on-read: the snapshot is deleted after retrieval to prevent memory leaks.
func (c *Capture) GetAndDeleteBeforeSnapshot(correlationID string) (performance.PerformanceSnapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.perf.takeBeforeSnapshot(correlationID)
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
	s.snapshots = make(map[string]performance.PerformanceSnapshot)
	s.snapshotOrder = make([]string, 0)
	s.baselines = make(map[string]performance.PerformanceBaseline)
	s.baselineOrder = make([]string, 0)
	s.beforeSnapshots = make(map[string]performance.PerformanceSnapshot)
}

// EventBufferStore exposes high-volume event buffers through read-only snapshots.
type EventBufferStore interface {
	NetworkBodies() []NetworkBody
	WebSocketEvents() []WebSocketEvent
	EnhancedActions() []EnhancedAction
}

// NetworkWaterfallStore exposes network-waterfall snapshots.
type NetworkWaterfallStore interface {
	Entries() []NetworkWaterfallEntry
	Count() int
}

// ExtensionLogStore exposes extension log snapshots.
type ExtensionLogStore interface {
	Entries() []ExtensionLog
}

// PerformanceSnapshotStore exposes performance snapshots keyed by URL.
type PerformanceSnapshotStore interface {
	Snapshots() []performance.PerformanceSnapshot
	SnapshotByURL(url string) (performance.PerformanceSnapshot, bool)
}

type eventBufferView struct {
	capture *Capture
}

func (v eventBufferView) NetworkBodies() []NetworkBody {
	return v.capture.GetNetworkBodies()
}

func (v eventBufferView) WebSocketEvents() []WebSocketEvent {
	return v.capture.GetAllWebSocketEvents()
}

func (v eventBufferView) EnhancedActions() []EnhancedAction {
	return v.capture.GetAllEnhancedActions()
}

type networkWaterfallView struct {
	capture *Capture
}

func (v networkWaterfallView) Entries() []NetworkWaterfallEntry {
	return v.capture.GetNetworkWaterfallEntries()
}

func (v networkWaterfallView) Count() int {
	return v.capture.GetNetworkWaterfallCount()
}

type extensionLogView struct {
	capture *Capture
}

func (v extensionLogView) Entries() []ExtensionLog {
	return v.capture.GetExtensionLogs()
}

type performanceSnapshotView struct {
	capture *Capture
}

func (v performanceSnapshotView) Snapshots() []performance.PerformanceSnapshot {
	return v.capture.GetPerformanceSnapshots()
}

func (v performanceSnapshotView) SnapshotByURL(url string) (performance.PerformanceSnapshot, bool) {
	return v.capture.GetPerformanceSnapshotByURL(url)
}

// EventBuffers returns a read-only sub-store view for network/websocket/action buffers.
func (c *Capture) EventBuffers() EventBufferStore {
	return eventBufferView{capture: c}
}

// NetworkWaterfallStore returns a read-only sub-store view for waterfall entries.
func (c *Capture) NetworkWaterfallStore() NetworkWaterfallStore {
	return networkWaterfallView{capture: c}
}

// ExtensionLogStore returns a read-only sub-store view for extension logs.
func (c *Capture) ExtensionLogStore() ExtensionLogStore {
	return extensionLogView{capture: c}
}

// PerformanceSnapshotStore returns a read-only sub-store view for performance snapshots.
func (c *Capture) PerformanceSnapshotStore() PerformanceSnapshotStore {
	return performanceSnapshotView{capture: c}
}
