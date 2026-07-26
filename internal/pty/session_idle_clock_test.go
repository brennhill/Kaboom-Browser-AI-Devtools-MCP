// session_idle_clock_test.go — deterministic idle-detection tests via an
// injected fake clock. Previously the only way to test "idle fires after silence"
// was a real short timeout + sleep, which is flaky and slow (repo rule 9).

package pty

import (
	"os/exec"
	"sync"
	"testing"
	"time"
)

// fakeClock is a controllable clock for tests. Advance moves time forward and
// synchronously fires any timers whose deadline has passed.
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
