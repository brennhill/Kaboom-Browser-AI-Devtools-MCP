// registry.go — Canonical operational incident codes and owned presentation metadata.
package incident

// Code is a stable operational failure identifier. Lookup rejects values outside
// the closed registry before they enter local state or outbound telemetry.
type Code string

const (
	CodeUncleanDaemonExit           Code = "unclean_daemon_exit"
	CodeDaemonRestartLoop           Code = "daemon_restart_loop"
	CodeExtensionReconnectExhausted Code = "extension_reconnect_exhausted"
	CodeTrackedTabRecoveryFailed    Code = "tracked_tab_recovery_failed"
	CodeContentReadinessTimeout     Code = "content_readiness_timeout"
	CodeStaleCommandResultRejected  Code = "stale_command_result_rejected"
	CodeQueueSaturated              Code = "queue_saturated"
	CodeStateRecoveryFailed         Code = "state_recovery_failed"
)

type Subsystem string

const (
	SubsystemDaemon   Subsystem = "daemon"
	SubsystemBridge   Subsystem = "bridge"
	SubsystemTracking Subsystem = "tracking"
	SubsystemCommand  Subsystem = "command"
	SubsystemQueue    Subsystem = "queue"
	SubsystemState    Subsystem = "state"
)

type Stage string

const (
	StageLifecycle  Stage = "lifecycle"
	StageReconnect  Stage = "reconnect"
	StageTracking   Stage = "tracking"
	StageReadiness  Stage = "readiness"
	StageResolution Stage = "resolution"
	StageCapacity   Stage = "capacity"
	StageRecovery   Stage = "recovery"
)

type Severity string

const (
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
	SeverityFatal   Severity = "fatal"
)

// Definition owns fixed Doctor presentation and analytics classification.
// Callers report facts; they never author outbound dimensions or Doctor prose.
type Definition struct {
	Subsystem    Subsystem
	Stage        Stage
	Severity     Severity
	Retryable    bool
	DoctorDetail string
	DoctorFix    string
}

var definitions = map[Code]Definition{
	CodeUncleanDaemonExit:           {SubsystemDaemon, StageLifecycle, SeverityFatal, true, "The previous daemon run did not record a clean shutdown.", "Restart Kaboom and inspect the correlated local incident timeline if this repeats."},
	CodeDaemonRestartLoop:           {SubsystemDaemon, StageLifecycle, SeverityFatal, false, "Kaboom repeatedly restarted inside the protected startup window.", "Inspect System Doctor before starting another daemon."},
	CodeExtensionReconnectExhausted: {SubsystemBridge, StageReconnect, SeverityError, true, "The extension exhausted its bounded reconnect attempts.", "Reload the extension and tracked page, then retry."},
	CodeTrackedTabRecoveryFailed:    {SubsystemTracking, StageTracking, SeverityError, true, "Kaboom could not restore the previously tracked tab.", "Select the intended tab in the extension and retry."},
	CodeContentReadinessTimeout:     {SubsystemBridge, StageReadiness, SeverityError, true, "The content script did not acknowledge readiness before the deadline.", "Reload the tracked page and retry the command."},
	CodeStaleCommandResultRejected:  {SubsystemCommand, StageResolution, SeverityWarning, true, "A result from an obsolete connection generation was rejected.", "Retry if the current command did not complete."},
	CodeQueueSaturated:              {SubsystemQueue, StageCapacity, SeverityWarning, true, "A bounded operational queue reached capacity.", "Wait for active work to drain, then retry."},
	CodeStateRecoveryFailed:         {SubsystemState, StageRecovery, SeverityError, true, "Persisted state could not be recovered safely.", "Reset the affected state through System Doctor and retry."},
}

func Lookup(code Code) (Definition, bool) {
	definition, ok := definitions[code]
	return definition, ok
}
