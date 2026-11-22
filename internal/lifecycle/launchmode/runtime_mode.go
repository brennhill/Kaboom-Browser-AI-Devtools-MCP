// runtime_mode.go — Selects the daemon process runtime from explicit CLI modes.
// Docs: docs/features/feature/lazy-server-start/index.md

package launchmode

// RuntimeMode identifies the process-level transport role.
type RuntimeMode string

const (
	RuntimeBridge RuntimeMode = "bridge"
	RuntimeDaemon RuntimeMode = "daemon"
)

// SelectRuntimeMode applies the canonical CLI precedence rules.
func SelectRuntimeMode(bridge, daemon bool) RuntimeMode {
	if bridge {
		return RuntimeBridge
	}
	if daemon {
		return RuntimeDaemon
	}
	return RuntimeBridge
}
