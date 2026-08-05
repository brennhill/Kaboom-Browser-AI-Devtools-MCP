// accessors.go — Aggregates health metadata from canonical capture owners.
// Purpose: Provides thread-safe diagnostic snapshots without exposing storage.
// Why: One lock-taking read layer over the stores, rather than four files split by which counter they return.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/telemetrystore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/circuit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

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
	telemetry *telemetrystore.Store
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
	telemetrySnap := r.telemetry.Snapshot()

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
