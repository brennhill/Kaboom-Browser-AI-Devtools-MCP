// Purpose: Tests runtime clocks, diagnostics, buffered writes, and reaping lifecycle.
// Docs: docs/features/feature/terminal/index.md

package pty

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"sync"
	"testing"
	"time"

	ptydiag "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty/diagnostics"
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
	ptydiag.SetHook(func(name string, fields map[string]any) {
		mu.Lock()
		events = append(events, capturedEvent{name: name, fields: fields})
		mu.Unlock()
	})
	t.Cleanup(func() { ptydiag.SetHook(nil) })
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

	ev, ok := findEvent(read(), ptydiag.EventSessionReapTimeout)
	if !ok {
		t.Fatalf("Close on an unreapable child must emit %s, got %v", ptydiag.EventSessionReapTimeout, read())
	}
	if ev.fields["session_id"] != "wedged" {
		t.Fatalf("%s must carry the session id, got %v", ptydiag.EventSessionReapTimeout, ev.fields)
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

	if ev, ok := findEvent(read(), ptydiag.EventSessionSignalFailed); ok {
		t.Fatalf("an already-exited child is expected, not a failure; got %s %v", ev.name, ev.fields)
	}
	if ev, ok := findEvent(read(), ptydiag.EventSessionReapTimeout); ok {
		t.Fatalf("a reaped child must not report a reap timeout; got %s %v", ev.name, ev.fields)
	}
}

// Session.Wait had no bound: it blocked on `reaped` forever. Its one caller is
// Relay.reapExitCode, which runs after the PTY read fails and BEFORE the deferred
// fanout.Close() — so an unreapable child parked readLoop permanently, the fanout
// never closed, and every WebSocket pump hung waiting for a channel that would
// never close (finding S6). Close already bounds its reap wait; Wait must too.
func TestSession_WaitIsBounded(t *testing.T) {
	read := captureDiag(t)

	s := &Session{ID: "unreapable", cmd: &exec.Cmd{}, done: make(chan struct{}), reaped: make(chan struct{})}

	returned := make(chan error, 1)
	go func() { returned <- s.Wait(30 * time.Millisecond) }()

	select {
	case err := <-returned:
		if !errors.Is(err, ErrReapTimeout) {
			t.Fatalf("Wait on an unreapable child should report ErrReapTimeout, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Session.Wait is unbounded — an unreapable child parks the relay's readLoop forever")
	}

	if _, ok := findEvent(read(), ptydiag.EventSessionReapTimeout); !ok {
		t.Fatalf("a give-up in Wait must be logged, got %v", read())
	}
}

// A child that has already exited must return from Wait immediately with no error
// and no timeout log — the overwhelmingly common case.
func TestSession_WaitReturnsImmediatelyForReapedChild(t *testing.T) {
	read := captureDiag(t)

	reaped := make(chan struct{})
	close(reaped)
	s := &Session{ID: "done", cmd: &exec.Cmd{}, done: make(chan struct{}), reaped: reaped}

	if err := s.Wait(time.Hour); err != nil {
		t.Fatalf("Wait on a reaped child: %v", err)
	}
	if ev, ok := findEvent(read(), ptydiag.EventSessionReapTimeout); ok {
		t.Fatalf("a reaped child must not log a timeout, got %v", ev.fields)
	}
}

// StopAll discards every Close error. A PTY fd that fails to close is a leak, so
// the failure has to reach the log.
func TestManager_StopAllLogsCloseFailure(t *testing.T) {
	read := captureDiag(t)

	m := newFakeManager()
	started, err := m.Start(StartConfig{ID: "s1"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	started.Session.ptmx = closeErrorPTY{err: errors.New("close fixture failed")}
	m.StopAll()

	ev, ok := findEvent(read(), ptydiag.EventSessionCloseFailed)
	if !ok {
		t.Fatalf("StopAll must not discard a Close error, got %v", read())
	}
	if ev.fields["session_id"] != "s1" {
		t.Fatalf("%s must carry the session id, got %v", ptydiag.EventSessionCloseFailed, ev.fields)
	}
	if ev.fields["error"] == nil {
		t.Fatalf("%s must carry the error, got %v", ptydiag.EventSessionCloseFailed, ev.fields)
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

	ev, ok := findEvent(read(), ptydiag.EventWriteBufferWriteFailed)
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

type closeErrorPTY struct{ err error }

func (pty closeErrorPTY) Read([]byte) (int, error)    { return 0, nil }
func (pty closeErrorPTY) Write(p []byte) (int, error) { return len(p), nil }
func (pty closeErrorPTY) Close() error                { return pty.err }
func (pty closeErrorPTY) Fd() uintptr                 { return 0 }

type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
}

func newFakeClock(t time.Time) *fakeClock { return &fakeClock{now: t} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) AfterFunc(d time.Duration, f func()) stoppableTimer {
	c.mu.Lock()
	defer c.mu.Unlock()
	ft := &fakeTimer{c: c, deadline: c.now.Add(d), f: f, active: true}
	c.timers = append(c.timers, ft)
	return ft
}

// Advance moves the clock forward by d and fires due timers (outside the lock so
// callbacks may re-enter the clock, e.g. via Reset).
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	now := c.now
	var toFire []func()
	for _, ft := range c.timers {
		if ft.active && !ft.deadline.After(now) {
			ft.active = false
			toFire = append(toFire, ft.f)
		}
	}
	c.mu.Unlock()
	for _, f := range toFire {
		f()
	}
}

type fakeTimer struct {
	c        *fakeClock
	deadline time.Time
	f        func()
	active   bool // false once stopped or fired
}

func (ft *fakeTimer) Stop() bool {
	ft.c.mu.Lock()
	defer ft.c.mu.Unlock()
	was := ft.active
	ft.active = false
	return was
}

func (ft *fakeTimer) Reset(d time.Duration) bool {
	ft.c.mu.Lock()
	defer ft.c.mu.Unlock()
	was := ft.active
	ft.active = true
	ft.deadline = ft.c.now.Add(d)
	return was
}

func TestSession_IdleFiresDeterministically(t *testing.T) {
	clk := newFakeClock(time.Unix(1000, 0))
	s := &Session{clk: clk}

	var mu sync.Mutex
	fired := 0
	s.SetIdleConfig(IdleConfig{Timeout: 50 * time.Millisecond, Callback: func(string) {
		mu.Lock()
		fired++
		mu.Unlock()
	}})

	s.AppendScrollback([]byte("some output")) // arms the idle timer at now+50ms

	clk.Advance(49 * time.Millisecond)
	mu.Lock()
	if fired != 0 {
		t.Fatalf("idle fired early: %d", fired)
	}
	mu.Unlock()

	clk.Advance(1 * time.Millisecond) // reaches the deadline
	mu.Lock()
	if fired != 1 {
		t.Fatalf("idle should have fired exactly once, got %d", fired)
	}
	mu.Unlock()
}

func TestSession_IdleTimerResetsOnNewOutput(t *testing.T) {
	clk := newFakeClock(time.Unix(1000, 0))
	s := &Session{clk: clk}

	var mu sync.Mutex
	fired := 0
	s.SetIdleConfig(IdleConfig{Timeout: 50 * time.Millisecond, Callback: func(string) {
		mu.Lock()
		fired++
		mu.Unlock()
	}})

	s.AppendScrollback([]byte("out 1")) // deadline = t+50ms
	clk.Advance(40 * time.Millisecond)  // t+40ms, not yet idle
	s.AppendScrollback([]byte("out 2")) // resets deadline to t+90ms
	clk.Advance(40 * time.Millisecond)  // t+80ms — would have fired at t+50ms without the reset

	mu.Lock()
	if fired != 0 {
		t.Fatalf("idle must reset on new output; fired too early: %d", fired)
	}
	mu.Unlock()

	clk.Advance(10 * time.Millisecond) // t+90ms — reaches the reset deadline
	mu.Lock()
	if fired != 1 {
		t.Fatalf("idle should fire once, 50ms after the LAST output, got %d", fired)
	}
	mu.Unlock()
}

// AppendScrollback never checked whether the session was closed, so a chunk that
// lands after Close (readLoop's last read, or a racing broadcast) re-armed a fresh
// 30s idle timer that nothing would ever stop — the callback then fired against a
// dead session, logging "session X is idle" for a shell that exited half a minute
// earlier (finding S10).
func TestSession_ClosedSessionDoesNotRearmIdleTimer(t *testing.T) {
	clk := newFakeClock(time.Unix(1000, 0))
	s := &Session{ID: "gone", cmd: &exec.Cmd{}, done: make(chan struct{}), reaped: closedChan(), clk: clk}

	var mu sync.Mutex
	fired := 0
	s.SetIdleConfig(IdleConfig{Timeout: 50 * time.Millisecond, Callback: func(string) {
		mu.Lock()
		fired++
		mu.Unlock()
	}})

	_ = s.Close()
	s.AppendScrollback([]byte("trailing output after close"))
	clk.Advance(time.Hour)

	mu.Lock()
	defer mu.Unlock()
	if fired != 0 {
		t.Fatalf("a closed session must not arm an idle timer; callback fired %d time(s)", fired)
	}
}

// Close stops the idle timer, but Stop loses the race if the timer has already
// fired: the callback is already queued (or running) and will report a dead
// session as idle. The callback itself must re-check the closed flag.
func TestSession_IdleCallbackSuppressedWhenTimerFiresDuringClose(t *testing.T) {
	clk := newFakeClock(time.Unix(1000, 0))
	s := &Session{ID: "racy", cmd: &exec.Cmd{}, done: make(chan struct{}), reaped: closedChan(), clk: clk}

	var mu sync.Mutex
	fired := 0
	s.SetIdleConfig(IdleConfig{Timeout: 50 * time.Millisecond, Callback: func(string) {
		mu.Lock()
		fired++
		mu.Unlock()
	}})
	s.AppendScrollback([]byte("output")) // arms the timer

	// Simulate "the deadline already passed, so Stop() can no longer un-fire it" by
	// detaching the armed timer from the session before Close can stop it. The
	// timer stays live in the clock and fires after Close returns.
	s.scrollMu.Lock()
	s.idleTimer = nil
	s.scrollMu.Unlock()

	_ = s.Close()
	clk.Advance(time.Hour) // the un-stopped timer fires now, after Close

	mu.Lock()
	defer mu.Unlock()
	if fired != 0 {
		t.Fatalf("an idle callback that fires after Close must be suppressed; fired %d time(s)", fired)
	}
}

// closedChan returns an already-closed channel so a hand-built Session can Close
// without waiting on a reaper that does not exist.
func closedChan() chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func TestSession_LastOutputAtUsesInjectedClock(t *testing.T) {
	base := time.Unix(1000, 0)
	clk := newFakeClock(base)
	s := &Session{clk: clk}

	s.AppendScrollback([]byte("a"))
	if got := s.LastOutputAt(); !got.Equal(base) {
		t.Fatalf("LastOutputAt = %v, want %v", got, base)
	}

	clk.Advance(10 * time.Second)
	s.AppendScrollback([]byte("b"))
	if got := s.LastOutputAt(); !got.Equal(base.Add(10 * time.Second)) {
		t.Fatalf("LastOutputAt = %v, want %v", got, base.Add(10*time.Second))
	}
}

func TestSession_ReaperRecoversPanic(t *testing.T) {
	reapHook = func() { panic("reaper boom") }
	defer func() { reapHook = nil }()

	s := &Session{ID: "reaper-panic", reaped: make(chan struct{}), cmd: &exec.Cmd{}}

	// Call reap synchronously. An escaped panic would crash the test binary;
	// reaching the assertions below proves the recover held.
	s.reap()

	select {
	case <-s.reaped:
		// Required: closed despite the panic.
	default:
		t.Fatal("reaper must close reaped even when it panics")
	}
}

func TestWriteBuffer_CloseDoesNotHangOnStuckWriter(t *testing.T) {
	gw := &gatedWriter{gate: make(chan struct{})} // gate never closed -> Write blocks forever
	wb := NewWriteBuffer(gw)
	if _, err := wb.Write([]byte("stuck")); err != nil {
		t.Fatalf("write: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- wb.Close() }()

	select {
	case err := <-done:
		if !errors.Is(err, ErrWriteBufferCloseTimeout) {
			t.Fatalf("Close on a stuck writer should time out, got %v", err)
		}
	case <-time.After(writeBufferCloseTimeout + 3*time.Second):
		t.Fatal("Close hung on a stuck writer — the bound did not fire")
	}
	close(gw.gate) // let the blocked drain goroutine unwind so it does not leak
}

// The stuck-writer close timeout leaks a drain goroutine + fd that cannot be safely
// interrupted; that leak must not be SILENT. Close must both return
// ErrWriteBufferCloseTimeout AND fire the diagnostics hook so it is diagnosable (M).
func TestWriteBuffer_CloseTimeoutSurfacesViaHookAndError(t *testing.T) {
	// Shorten the bound so the timeout path runs fast; restore after.
	origTimeout := writeBufferCloseTimeout
	writeBufferCloseTimeout = 40 * time.Millisecond
	defer func() { writeBufferCloseTimeout = origTimeout }()

	var firedPending int
	var fired bool
	SetWriteBufferCloseTimeoutHook(func(pending int) { fired = true; firedPending = pending })
	defer SetWriteBufferCloseTimeoutHook(nil)

	gw := &gatedWriter{gate: make(chan struct{})} // Write blocks forever
	wb := NewWriteBuffer(gw)
	if _, err := wb.Write([]byte("stuck")); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := wb.Close()
	if !errors.Is(err, ErrWriteBufferCloseTimeout) {
		t.Fatalf("Close on a stuck writer must return the timeout error, got %v", err)
	}
	if !fired {
		t.Fatal("the close timeout must fire the diagnostics hook — a silent goroutine+fd leak is not diagnosable")
	}
	if firedPending != 5 {
		t.Fatalf("the hook should report the undrained byte count (5), got %d", firedPending)
	}
	close(gw.gate) // let the blocked drain goroutine unwind so it does not leak into other tests
}

func TestWriteBuffer_BasicWrite(t *testing.T) {
	var dest bytes.Buffer
	wb := NewWriteBuffer(&dest)

	n, err := wb.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if n != 5 {
		t.Fatalf("expected 5 bytes, got %d", n)
	}

	// Close waits for drain to complete, making dest safe to read.
	wb.Close()

	if dest.String() != "hello" {
		t.Fatalf("expected %q, got %q", "hello", dest.String())
	}
}

func TestWriteBuffer_Backpressure(t *testing.T) {
	gw := &gatedWriter{gate: make(chan struct{})}
	wb := NewWriteBuffer(gw)
	defer func() {
		close(gw.gate)
		wb.Close()
	}()

	// Fill buffer to capacity.
	data := make([]byte, writeBufferMax)
	_, err := wb.Write(data)
	if err != nil {
		t.Fatalf("write to fill: %v", err)
	}

	// Exceeding capacity should fail.
	_, err = wb.Write([]byte("x"))
	if err != ErrWriteBufferFull {
		t.Fatalf("expected ErrWriteBufferFull, got: %v", err)
	}
}

// gatedWriter blocks on Write until gate channel is closed.
type gatedWriter struct {
	gate chan struct{}
}

func (w *gatedWriter) Write(p []byte) (int, error) {
	<-w.gate
	return len(p), nil
}

func TestWriteBuffer_Pending(t *testing.T) {
	gw := &gatedWriter{gate: make(chan struct{})}
	wb := NewWriteBuffer(gw)
	defer func() {
		close(gw.gate)
		wb.Close()
	}()

	wb.Write([]byte("hello"))
	// Pending must reflect the buffered bytes while the drain is blocked. drain()
	// only reslices wb.buf *after* the underlying Write returns (here: never,
	// until the gate closes in the deferred cleanup), so all 5 bytes stay buffered
	// and Pending() is deterministically 5. (The old `p < 0` check on a len()-based
	// value could never fail and proved nothing.)
	if p := wb.Pending(); p != 5 {
		t.Fatalf("expected 5 pending bytes while drain is blocked, got %d", p)
	}
}

func TestWriteBuffer_CloseFlushes(t *testing.T) {
	var dest bytes.Buffer
	wb := NewWriteBuffer(&dest)

	wb.Write([]byte("data"))
	wb.Close()

	if dest.String() != "data" {
		t.Fatalf("expected %q after close, got %q", "data", dest.String())
	}
}

func TestWriteBuffer_DoubleClose(t *testing.T) {
	var dest bytes.Buffer
	wb := NewWriteBuffer(&dest)
	wb.Close()
	if err := wb.Close(); err != nil {
		t.Fatalf("double close: %v", err)
	}
}

// A write to a CLOSED buffer (the shell exited) and a write that overflows the
// backpressure cap (the child stopped draining stdin) are different failures with
// different remedies, and both used to return ErrWriteBufferFull — so no caller
// could tell "your terminal is gone" from "your terminal is behind" (finding S9).
func TestWriteBuffer_ClosedIsDistinctFromFull(t *testing.T) {
	var dest bytes.Buffer
	closed := NewWriteBuffer(&dest)
	closed.Close()

	_, err := closed.Write([]byte("x"))
	if !errors.Is(err, ErrWriteBufferClosed) {
		t.Fatalf("write after close: got %v, want ErrWriteBufferClosed", err)
	}
	if errors.Is(err, ErrWriteBufferFull) {
		t.Fatal("a closed buffer must not report backpressure — callers cannot tell the two apart")
	}

	gw := &gatedWriter{gate: make(chan struct{})} // never drains
	full := NewWriteBuffer(gw)
	if _, err := full.Write(make([]byte, writeBufferMax)); err != nil {
		t.Fatalf("filling the buffer: %v", err)
	}
	_, err = full.Write([]byte("overflow"))
	if !errors.Is(err, ErrWriteBufferFull) {
		t.Fatalf("write past the cap: got %v, want ErrWriteBufferFull", err)
	}
	if errors.Is(err, ErrWriteBufferClosed) {
		t.Fatal("a full buffer must not report itself closed — the shell is still alive")
	}
	close(gw.gate)
}

func TestWriteBuffer_LargeWrite(t *testing.T) {
	var dest bytes.Buffer
	wb := NewWriteBuffer(&dest)

	// Write data larger than one chunk to exercise chunked flushing.
	data := make([]byte, writeChunkSize*3)
	for i := range data {
		data[i] = byte(i % 256)
	}

	n, err := wb.Write(data)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if n != len(data) {
		t.Fatalf("expected %d bytes, got %d", len(data), n)
	}

	// Close waits for drain to complete, making dest safe to read.
	wb.Close()

	if dest.Len() != len(data) {
		t.Fatalf("expected %d drained bytes, got %d", len(data), dest.Len())
	}
}

func TestWriteBuffer_ConcurrentWriteDuringClose(t *testing.T) {
	// Write signals wb.notify and Close closes it: a keystroke arriving as the
	// shell exits pits handlers.go's Write against relay.go's deferred Close on
	// the same channel. Unsynchronized, that is a data race the detector flags
	// and can panic with "send on closed channel". Many rounds with several
	// concurrent writers make the interleaving reliable under `go test -race`.
	for round := 0; round < 300; round++ {
		wb := NewWriteBuffer(io.Discard)
		var wg sync.WaitGroup
		for w := 0; w < 4; w++ {
			wg.Add(1)
			go func() { // lint:allow-bare-goroutine — bounded by the loop, joined via wg
				defer wg.Done()
				for i := 0; i < 25; i++ {
					_, _ = wb.Write([]byte("x"))
				}
			}()
		}
		// Close concurrently with the in-flight writers — the whole point.
		_ = wb.Close()
		wg.Wait()
	}
}
