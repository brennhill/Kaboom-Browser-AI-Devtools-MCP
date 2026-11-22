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
	CodeDaemonPanic                 Code = "daemon_panic"
	CodeDaemonStartFailed           Code = "daemon_start_failed"
	CodeToolRateLimited             Code = "tool_rate_limited"
	CodeBridgeConnectionError       Code = "bridge_connection_error"
	CodeBridgePortBlocked           Code = "bridge_port_blocked"
	CodeBridgeSpawnBuildError       Code = "bridge_spawn_build_error"
	CodeBridgeSpawnStartError       Code = "bridge_spawn_start_error"
	CodeBridgeSpawnTimeout          Code = "bridge_spawn_timeout"
	CodeBridgeExitError             Code = "bridge_exit_error"
	CodeExtensionDisconnect         Code = "extension_disconnect"
	CodeInstallConfigError          Code = "install_config_error"
)

type Subsystem string

const (
	SubsystemDaemon    Subsystem = "daemon"
	SubsystemBridge    Subsystem = "bridge"
	SubsystemTracking  Subsystem = "tracking"
	SubsystemCommand   Subsystem = "command"
	SubsystemQueue     Subsystem = "queue"
	SubsystemState     Subsystem = "state"
	SubsystemStartup   Subsystem = "startup"
	SubsystemExtension Subsystem = "extension"
	SubsystemInstaller Subsystem = "installer"
)

type ErrorKind string

const (
	ErrorKindInternal    ErrorKind = "internal"
	ErrorKindIntegration ErrorKind = "integration"
)

type PrivacyClass string

const PrivacyBoundedProductMetadata PrivacyClass = "bounded_product_metadata"

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
	ErrorKind    ErrorKind
	Privacy      PrivacyClass
	DoctorDetail string
	DoctorFix    string
}

var definitions = map[Code]Definition{
	CodeUncleanDaemonExit:           definition(SubsystemDaemon, StageLifecycle, SeverityFatal, true, ErrorKindInternal, doctorProse{detail: "The previous daemon run did not record a clean shutdown.", fix: "Restart Kaboom and inspect the correlated local incident timeline if this repeats."}),
	CodeDaemonRestartLoop:           definition(SubsystemDaemon, StageLifecycle, SeverityFatal, false, ErrorKindInternal, doctorProse{detail: "Kaboom repeatedly restarted inside the protected startup window.", fix: "Inspect System Doctor before starting another daemon."}),
	CodeExtensionReconnectExhausted: definition(SubsystemBridge, StageReconnect, SeverityError, true, ErrorKindIntegration, doctorProse{detail: "The extension exhausted its bounded reconnect attempts.", fix: "Reload the extension and tracked page, then retry."}),
	CodeTrackedTabRecoveryFailed:    definition(SubsystemTracking, StageTracking, SeverityError, true, ErrorKindIntegration, doctorProse{detail: "Kaboom could not restore the previously tracked tab.", fix: "Select the intended tab in the extension and retry."}),
	CodeContentReadinessTimeout:     definition(SubsystemBridge, StageReadiness, SeverityError, true, ErrorKindIntegration, doctorProse{detail: "The content script did not acknowledge readiness before the deadline.", fix: "Reload the tracked page and retry the command."}),
	CodeStaleCommandResultRejected:  definition(SubsystemCommand, StageResolution, SeverityWarning, true, ErrorKindInternal, doctorProse{detail: "A result from an obsolete connection generation was rejected.", fix: "Retry if the current command did not complete."}),
	CodeQueueSaturated:              definition(SubsystemQueue, StageCapacity, SeverityWarning, true, ErrorKindInternal, doctorProse{detail: "A bounded operational queue reached capacity.", fix: "Wait for active work to drain, then retry."}),
	CodeStateRecoveryFailed:         definition(SubsystemState, StageRecovery, SeverityError, true, ErrorKindInternal, doctorProse{detail: "Persisted state could not be recovered safely.", fix: "Reset the affected state through System Doctor and retry."}),
	CodeDaemonPanic:                 definition(SubsystemDaemon, StageLifecycle, SeverityFatal, false, ErrorKindInternal, doctorProse{detail: "The daemon recovered a panic.", fix: "Inspect local crash diagnostics and restart Kaboom."}),
	CodeDaemonStartFailed:           definition(SubsystemStartup, StageLifecycle, SeverityFatal, false, ErrorKindInternal, doctorProse{detail: "The daemon could not start.", fix: "Inspect local startup diagnostics and retry."}),
	CodeToolRateLimited:             definition(SubsystemDaemon, StageCapacity, SeverityWarning, true, ErrorKindIntegration, doctorProse{detail: "A tool call exceeded the bounded request rate.", fix: "Retry after the reported backoff."}),
	CodeBridgeConnectionError:       definition(SubsystemBridge, StageReconnect, SeverityError, true, ErrorKindIntegration, doctorProse{detail: "The MCP bridge could not connect.", fix: "Retry the connection or inspect System Doctor."}),
	CodeBridgePortBlocked:           definition(SubsystemBridge, StageLifecycle, SeverityError, false, ErrorKindIntegration, doctorProse{detail: "The bridge port is owned by another process.", fix: "Free the configured port and retry."}),
	CodeBridgeSpawnBuildError:       definition(SubsystemBridge, StageLifecycle, SeverityFatal, false, ErrorKindInternal, doctorProse{detail: "The bridge daemon binary could not be built.", fix: "Inspect local build diagnostics."}),
	CodeBridgeSpawnStartError:       definition(SubsystemBridge, StageLifecycle, SeverityFatal, false, ErrorKindInternal, doctorProse{detail: "The bridge daemon process could not start.", fix: "Inspect local process diagnostics."}),
	CodeBridgeSpawnTimeout:          definition(SubsystemBridge, StageReadiness, SeverityError, true, ErrorKindInternal, doctorProse{detail: "The spawned daemon did not become ready before its deadline.", fix: "Retry after inspecting System Doctor."}),
	CodeBridgeExitError:             definition(SubsystemBridge, StageLifecycle, SeverityError, false, ErrorKindInternal, doctorProse{detail: "The bridge exited unexpectedly.", fix: "Inspect local bridge diagnostics."}),
	CodeExtensionDisconnect:         definition(SubsystemExtension, StageReconnect, SeverityWarning, false, ErrorKindIntegration, doctorProse{detail: "The extension disconnected.", fix: "Reload the extension if it does not reconnect."}),
	CodeInstallConfigError:          definition(SubsystemInstaller, StageRecovery, SeverityError, false, ErrorKindInternal, doctorProse{detail: "Installation configuration could not be updated.", fix: "Inspect local installer diagnostics and retry."}),
}

// doctorProse pairs the Doctor detail and fix strings every definition carries.
type doctorProse struct {
	detail string
	fix    string
}

func definition(subsystem Subsystem, stage Stage, severity Severity, retryable bool, kind ErrorKind, prose doctorProse) Definition {
	return Definition{Subsystem: subsystem, Stage: stage, Severity: severity, Retryable: retryable, ErrorKind: kind, Privacy: PrivacyBoundedProductMetadata, DoctorDetail: prose.detail, DoctorFix: prose.fix}
}

func Lookup(code Code) (Definition, bool) {
	definition, ok := definitions[code]
	return definition, ok
}
