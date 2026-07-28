// Purpose: Provides the thread-safe read accessors over buffered counters, timestamps, events and performance snapshots.
// Why: One lock-taking read layer over the stores, rather than four files split by which counter they return.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/circuit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// GetNetworkTotalAdded returns the monotonic total of network bodies ever added
func (s *TelemetryStore) GetNetworkTotalAdded() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buffers.networkTotal()
}

// GetNetworkErrorTotalAdded returns the monotonic total of error network bodies ever added.
func (s *TelemetryStore) GetNetworkErrorTotalAdded() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buffers.networkErrorTotal()
}

// GetWebSocketTotalAdded returns the monotonic total of WebSocket events ever added
func (s *TelemetryStore) GetWebSocketTotalAdded() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buffers.webSocketTotal()
}

// GetActionTotalAdded returns the monotonic total of actions ever added
func (s *TelemetryStore) GetActionTotalAdded() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buffers.actionTotal()
}

// TelemetrySnapshot is an immutable point-in-time view of event-store counters.
//
// Invariants:
// - Counts and totals in one snapshot come from the same s.mu critical section.
type TelemetrySnapshot struct {
	NetworkTotalAdded   int64
	WebSocketTotalAdded int64
	ActionTotalAdded    int64
	NetworkCount        int
	WebSocketCount      int
	ActionCount         int
	ConnectionCount     int
}

// GetSnapshot returns a thread-safe capture counter snapshot.
//
// Failure semantics:
// - Snapshot can be stale immediately after return; callers should treat it as diagnostic-only.
func (s *TelemetryStore) GetSnapshot() TelemetrySnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return TelemetrySnapshot{
		NetworkTotalAdded:   s.buffers.networkTotal(),
		WebSocketTotalAdded: s.buffers.webSocketTotal(),
		ActionTotalAdded:    s.buffers.actionTotal(),
		NetworkCount:        s.buffers.networkCount(),
		WebSocketCount:      s.buffers.webSocketCount(),
		ActionCount:         s.buffers.actionCount(),
		ConnectionCount:     s.wsConnections.Count(),
	}
}

// HealthSnapshot aggregates capture + dispatcher + circuit health state.
//
// Invariants:
// - Each subsystem contributes a detached snapshot under its own lock.
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

// HealthReader composes detached health snapshots from independently
// synchronized runtime owners.
type HealthReader struct {
	circuit   *circuit.CircuitBreaker
	queries   *queries.QueryDispatcher
	extension *ExtensionRuntime
	telemetry *TelemetryStore
}

// NewHealthReader binds health aggregation to the canonical runtime owners.
func NewHealthReader(capture *Capture) *HealthReader {
	return &HealthReader{
		circuit:   capture.circuit,
		queries:   capture.queryDispatcher,
		extension: capture.extension,
		telemetry: capture.telemetry,
	}
}

// Snapshot returns a lock-safe aggregate health view.
func (r *HealthReader) Snapshot() HealthSnapshot {
	circuitOpen, circuitReason, circuitOpenedAt, windowEventCount := r.circuit.GetState()
	querySnap := r.queries.GetSnapshot()
	extensionSnap := r.extension.Snapshot()
	telemetrySnap := r.telemetry.GetSnapshot()

	return HealthSnapshot{
		WebSocketCount:        telemetrySnap.WebSocketCount,
		NetworkBodyCount:      telemetrySnap.NetworkCount,
		ActionCount:           telemetrySnap.ActionCount,
		ConnectionCount:       telemetrySnap.ConnectionCount,
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
func (s *TelemetryStore) GetNetworkBodies() []types.NetworkBody {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buffers.networkBodiesCopy()
}

// GetAllWebSocketEvents returns a copy of all WebSocket events slice (thread-safe)
func (s *TelemetryStore) GetAllWebSocketEvents() []types.WebSocketEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buffers.webSocketEventsCopy()
}

// GetAllEnhancedActions returns a copy of all enhanced actions slice (thread-safe)
func (s *TelemetryStore) GetAllEnhancedActions() []types.EnhancedAction {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buffers.enhancedActionsCopy()
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
