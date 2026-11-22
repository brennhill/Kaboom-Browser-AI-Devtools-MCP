// diagnostics.go — Owns platform-neutral structured PTY failure reporting.
// Why: Manager and write-buffer failures exist on every target, while Unix
// session lifecycle code is build-tagged and cannot own their shared contract.
// Docs: docs/features/feature/terminal/index.md

package diagnostics

import "sync"

// Diagnostic event names emitted by the PTY runtime.
const (
	EventSessionSignalFailed    = "pty_session_signal_failed"
	EventSessionReapTimeout     = "pty_session_reap_timeout"
	EventSessionCloseFailed     = "pty_session_close_failed"
	EventWriteBufferWriteFailed = "pty_writebuffer_write_failed"
)

var sink struct {
	sync.RWMutex
	hook func(event string, fields map[string]any)
}

// SetHook installs or clears the process-local structured diagnostic sink.
func SetHook(hook func(event string, fields map[string]any)) {
	sink.Lock()
	sink.hook = hook
	sink.Unlock()
}

// Emit forwards a structured PTY failure when a sink is installed.
func Emit(event string, fields map[string]any) {
	sink.RLock()
	hook := sink.hook
	sink.RUnlock()
	if hook != nil {
		hook(event, fields)
	}
}
