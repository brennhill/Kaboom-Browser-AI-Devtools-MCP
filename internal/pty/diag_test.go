// diag_test.go — proves the PTY layer's state-mutating failures are no longer
// silent (finding S8 / rule 25): a stuck reap, a failed session close, and a
// failed PTY flush each emit a structured event carrying the session id.

package pty

import (
	"errors"
	"os/exec"
	"sync"
	"testing"
	"time"
)

// capturedEvent is one diagnostic emission.
type capturedEvent struct {
	name   string
	fields map[string]any
}

// captureDiag installs a recording sink for the duration of the test and returns
// a reader for the events seen so far. The sink is mutex-guarded because emissions
// come from background goroutines (the write-buffer drain, a session reaper).
func captureDiag(t *testing.T) func() []capturedEvent {
	t.Helper()
	var mu sync.Mutex
	var events []capturedEvent
	SetDiagnosticHook(func(name string, fields map[string]any) {
		mu.Lock()
		events = append(events, capturedEvent{name: name, fields: fields})
		mu.Unlock()
	})
	t.Cleanup(func() { SetDiagnosticHook(nil) })
	return func() []capturedEvent {
		mu.Lock()
		defer mu.Unlock()
		return append([]capturedEvent(nil), events...)
	}
}

func findEvent(events []capturedEvent, name string) (capturedEvent, bool) {
	for _, e := range events {
		if e.name == name {
			return e, true
		}
	}
	return capturedEvent{}, false
}

// A child that is never reaped (D-state, or a wedged reaper) makes Close fall
// through both bounded waits and give up. That leaves a live process and a leaked
// PTY behind, so it must not be silent.
func TestSession_CloseLogsReapTimeout(t *testing.T) {
	orig := sessionReapWait
	sessionReapWait = 20 * time.Millisecond
	defer func() { sessionReapWait = orig }()

	read := captureDiag(t)

	// reaped stays open -> both waits expire. Process is nil, so no real signal.
	s := &Session{ID: "wedged", cmd: &exec.Cmd{}, done: make(chan struct{}), reaped: make(chan struct{})}
	_ = s.Close()

	ev, ok := findEvent(read(), EventSessionReapTimeout)
	if !ok {
		t.Fatalf("Close on an unreapable child must emit %s, got %v", EventSessionReapTimeout, read())
	}
	if ev.fields["session_id"] != "wedged" {
		t.Fatalf("%s must carry the session id, got %v", EventSessionReapTimeout, ev.fields)
	}
}

// A child that exited on its own is already reaped, so SIGTERM returns
// os.ErrProcessDone. That is the EXPECTED case, not a failure — logging it would
// bury the real signal failures under noise (rule 25: distinguish the two).
func TestSession_CloseDoesNotLogAlreadyExitedChild(t *testing.T) {
	read := captureDiag(t)

	s, err := Spawn(SpawnConfig{ID: "exits", Cmd: "/bin/sh", Args: []string{"-c", "exit 0"}})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	<-s.reaped // the child is gone and reaped before we close
	_ = s.Close()

	if ev, ok := findEvent(read(), EventSessionSignalFailed); ok {
		t.Fatalf("an already-exited child is expected, not a failure; got %s %v", ev.name, ev.fields)
	}
	if ev, ok := findEvent(read(), EventSessionReapTimeout); ok {
		t.Fatalf("a reaped child must not report a reap timeout; got %s %v", ev.name, ev.fields)
	}
}

// StopAll discards every Close error. A PTY fd that fails to close is a leak, so
// the failure has to reach the log.
func TestManager_StopAllLogsCloseFailure(t *testing.T) {
	read := captureDiag(t)

	m := newFakeManager() // fake sessions have a nil ptmx, so Close returns ErrInvalid
	if _, err := m.Start(StartConfig{ID: "s1"}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	m.StopAll()

	ev, ok := findEvent(read(), EventSessionCloseFailed)
	if !ok {
		t.Fatalf("StopAll must not discard a Close error, got %v", read())
	}
	if ev.fields["session_id"] != "s1" {
		t.Fatalf("%s must carry the session id, got %v", EventSessionCloseFailed, ev.fields)
	}
	if ev.fields["error"] == nil {
		t.Fatalf("%s must carry the error, got %v", EventSessionCloseFailed, ev.fields)
	}
}

// A failed PTY write strands the buffered bytes in the queue with no retry. The
// user sees typed input vanish; the daemon log must say why.
func TestWriteBuffer_FlushFailureIsLogged(t *testing.T) {
	read := captureDiag(t)

	boom := errors.New("input/output error")
	wb := NewWriteBuffer(&errWriter{err: boom})
	if _, err := wb.Write([]byte("ls -la\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	_ = wb.Close() // Close's final flush runs synchronously, so the failure is observed here

	ev, ok := findEvent(read(), EventWriteBufferWriteFailed)
	if !ok {
		t.Fatalf("a failed PTY flush must be logged, got %v", read())
	}
	if ev.fields["pending_bytes"] != 7 {
		t.Fatalf("the log must report the stranded byte count (7), got %v", ev.fields["pending_bytes"])
	}
	if ev.fields["error"] == nil {
		t.Fatalf("the log must carry the write error, got %v", ev.fields)
	}
}

// errWriter fails every write with a fixed error.
type errWriter struct{ err error }

func (w *errWriter) Write(p []byte) (int, error) { return 0, w.err }
