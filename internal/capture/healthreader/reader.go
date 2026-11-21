// reader.go — Projects detached operational health across capture owners.
// Docs: docs/features/feature/backend-log-streaming/index.md

package healthreader

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/browsercommand"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/logstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/perfstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/telemetrystore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/circuit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
)

// Owners are the independently synchronized state owners that health and
// doctor reporting read. The composition root builds it (capture.HealthOwners)
// so health consumers never hold the whole Capture. The zero value means no
// capture is composed.
type Owners struct {
	Extension     *syncruntime.Runtime
	Commands      *browsercommand.Owner
	Telemetry     *telemetrystore.Store
	Circuit       *circuit.CircuitBreaker
	ExtensionLogs *logstore.Extension
	Performance   *perfstore.Store
	Recordings    *recording.RecordingManager
	// DiagnosticLogs holds the daemon HTTP debug log.
	DiagnosticLogs *logstore.Diagnostic
}

// Available reports whether the owners were composed (degraded mode otherwise).
func (o Owners) Available() bool { return o.Extension != nil }

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
type Reader struct{ owners Owners }

// New binds health projection to the composed state owners.
func New(owners Owners) *Reader { return &Reader{owners: owners} }

// Snapshot returns a lock-safe aggregate health view.
func (r *Reader) Snapshot() Snapshot {
	circuitOpen, circuitReason, circuitOpenedAt, windowEventCount := r.owners.Circuit.GetState()
	querySnapshot := r.owners.Commands.Snapshot()
	extensionSnapshot := r.owners.Extension.Snapshot()
	telemetrySnapshot := r.owners.Telemetry.Snapshot()
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
