// reader.go — Projects detached operational health across capture owners.
// Docs: docs/features/feature/backend-log-streaming/index.md

package healthreader

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// Snapshot aggregates capture, dispatcher, and circuit health state.
type Snapshot struct {
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

// Reader owns the diagnostic projection over canonical state owners.
type Reader struct{ capture *capture.Capture }

// New binds health projection to a capture composition root.
func New(captured *capture.Capture) *Reader { return &Reader{capture: captured} }

// Snapshot returns a lock-safe aggregate health view.
func (r *Reader) Snapshot() Snapshot {
	circuitOpen, circuitReason, circuitOpenedAt, windowEventCount := r.capture.Circuit().GetState()
	querySnapshot := r.capture.Queries().GetSnapshot()
	extensionSnapshot := r.capture.Extension().Snapshot()
	telemetrySnapshot := r.capture.Telemetry().Snapshot()
	return Snapshot{
		WebSocketCount: telemetrySnapshot.WebSocketCount, NetworkBodyCount: telemetrySnapshot.NetworkCount,
		ActionCount: telemetrySnapshot.ActionCount, NetworkCapacity: telemetrySnapshot.NetworkCapacity,
		WebSocketCapacity: telemetrySnapshot.WebSocketCapacity, ActionCapacity: telemetrySnapshot.ActionCapacity,
		ConnectionCount: telemetrySnapshot.ConnectionCount, LastPollTime: extensionSnapshot.LastPollAt,
		ExtSessionID: extensionSnapshot.ExtSessionID, ExtSessionChangedTime: extensionSnapshot.ExtSessionChangedAt,
		PilotEnabled: extensionSnapshot.PilotEnabled, CircuitOpen: circuitOpen, WindowEventCount: windowEventCount,
		CircuitReason: circuitReason, CircuitOpenedTime: circuitOpenedAt, PendingQueryCount: querySnapshot.PendingQueryCount,
		QueryResultCount: querySnapshot.QueryResultCount, ActiveTestIDCount: extensionSnapshot.ActiveTestIDCount,
		QueryTimeout: querySnapshot.QueryTimeout,
	}
}
