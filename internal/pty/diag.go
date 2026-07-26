// diag.go — structured diagnostics sink for the pty package.
// Why: rule 25 (fail loud) applies to state-mutating operations inside the PTY
// layer — signalling a child, closing a session, flushing buffered stdin. Those
// failures were discarded with `_ =`, so a wedged child or a stranded write left
// no trace anywhere. The package has no logger dependency (and must stay usable
// from tests without wiring), so the daemon installs one sink at wiring time and
// every failure site funnels through it.
// Docs: docs/features/feature/terminal/index.md

package pty

import "sync"

// Diagnostic event names emitted by this package. Named constants so the daemon
// log schema is greppable from one place.
const (
	// EventSessionSignalFailed: sending SIGTERM/SIGKILL to the child failed for a
	// reason other than "already exited".
	EventSessionSignalFailed = "pty_session_signal_failed"
	// EventSessionReapTimeout: the child was not reaped within Close's bound even
	// after SIGKILL — the process (and its PTY fd) outlives the daemon's teardown.
	EventSessionReapTimeout = "pty_session_reap_timeout"
	// EventSessionCloseFailed: Session.Close returned an error (PTY fd close).
	EventSessionCloseFailed = "pty_session_close_failed"
	// EventWriteBufferWriteFailed: a buffered chunk could not be written to the
	// PTY; the bytes stay queued and are not retried until the next write.
	EventWriteBufferWriteFailed = "pty_writebuffer_write_failed"
)

// diagMu guards diagFn: the setter (daemon wiring) and the readers (any session
// teardown or drain goroutine) run concurrently.
var (
	diagMu sync.Mutex
	diagFn func(event string, fields map[string]any)
)

// SetDiagnosticHook installs (or clears, with nil) the structured event sink for
// PTY-internal failures. Called at daemon wiring; also used by tests.
func SetDiagnosticHook(fn func(event string, fields map[string]any)) {
	diagMu.Lock()
	diagFn = fn
	diagMu.Unlock()
}

// diag emits a structured event if a sink is installed. Nil-safe, and snapshots
// the hook under the lock so a concurrent SetDiagnosticHook cannot race the call.
func diag(event string, fields map[string]any) {
	diagMu.Lock()
	fn := diagFn
	diagMu.Unlock()
	if fn != nil {
		fn(event, fields)
	}
}
