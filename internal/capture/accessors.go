// accessors.go — Aggregates health metadata from canonical capture owners.
// Purpose: Provides thread-safe diagnostic snapshots without exposing storage.
// Why: One lock-taking read layer over the stores, rather than four files split by which counter they return.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/circuit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

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
	NetworkCapacity     int
	WebSocketCapacity   int
	ActionCapacity      int
	ConnectionCount     int
}

// GetSnapshot returns a thread-safe capture counter snapshot.
//
// Failure semantics:
// - Snapshot can be stale immediately after return; callers should treat it as diagnostic-only.
func (s *TelemetryStore) GetSnapshot() TelemetrySnapshot {
	network := s.networkBodies.Stats()
	actions := s.actions.Stats()
	webSockets := s.webSockets.Stats()

	return TelemetrySnapshot{
		NetworkTotalAdded:   network.TotalAdded,
		WebSocketTotalAdded: webSockets.TotalAdded,
		ActionTotalAdded:    actions.TotalAdded,
		NetworkCount:        network.Count,
		WebSocketCount:      webSockets.Count,
		ActionCount:         actions.Count,
		NetworkCapacity:     network.Pressure.Capacity,
		WebSocketCapacity:   webSockets.Capacity,
		ActionCapacity:      actions.Capacity,
		ConnectionCount:     webSockets.ConnectionCount,
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
	NetworkCapacity       int
	WebSocketCapacity     int
	ActionCapacity        int
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
		NetworkCapacity:       telemetrySnap.NetworkCapacity,
		WebSocketCapacity:     telemetrySnap.WebSocketCapacity,
		ActionCapacity:        telemetrySnap.ActionCapacity,
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
