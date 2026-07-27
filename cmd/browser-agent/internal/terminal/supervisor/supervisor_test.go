// supervisor_test.go — Deterministic tests for the terminal-server
// auto-restart supervisor. All timing/binding is injected so the tests never
// touch a real port or sleep.

package supervisor

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"
)

// newTestSupervisor builds a supervisor with all external seams stubbed and an
// event recorder wired to logFn.
func newTestSupervisor(t *testing.T, initialDone <-chan struct{}) (*Supervisor, *supEvents, *int) {
	t.Helper()
	ev := &supEvents{}
	currentPort := 0
	ts := &Supervisor{
		deps: Dependencies{
			Start:   func(int, *http.ServeMux) (*http.Server, <-chan struct{}, error) { return nil, nil, nil },
			Reclaim: func(int) {},
			SetPort: func(port int) { currentPort = port },
			Log:     ev.record,
			Warn:    func(string, ...any) {},
			After:   func(time.Duration) <-chan time.Time { return closedTimer() },
		},
		port:        7891,
		stop:        make(chan struct{}),
		loopDone:    make(chan struct{}),
		initialDone: initialDone,
	}
	return ts, ev, &currentPort
}

// closedTimer returns an already-fired timer channel so backoff waits resolve
// instantly in tests.
func closedTimer() <-chan time.Time {
	ch := make(chan time.Time, 1)
	ch <- time.Now()
	return ch
}

type supEvents struct {
	mu     sync.Mutex
	events []string
}

func (e *supEvents) record(event string, _ map[string]any) {
	e.mu.Lock()
	e.events = append(e.events, event)
	e.mu.Unlock()
}

func (e *supEvents) count(event string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, ev := range e.events {
		if ev == event {
			n++
		}
	}
	return n
}

func (e *supEvents) waitFor(t *testing.T, event string, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if e.count(event) >= want {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d %q events (got %d)", want, event, e.count(event))
}

// TestSupervisor_RestartsAfterUnexpectedDeath verifies that when the terminal
// server dies, the supervisor restarts it and marks the port live again.
func TestSupervisor_RestartsAfterUnexpectedDeath(t *testing.T) {
	initialDone := make(chan struct{})
	ts, ev, currentPort := newTestSupervisor(t, initialDone)

	// startFn succeeds and hands back a fresh (never-closed) done channel, so the
	// supervisor settles into watching a healthy restarted server.
	restarted := make(chan struct{})
	ts.deps.Start = func(int, *http.ServeMux) (*http.Server, <-chan struct{}, error) {
		return &http.Server{}, restarted, nil
	}

	go ts.supervise(ts.initialDone)

	// Kill the initial server.
	close(initialDone)

	ev.waitFor(t, "terminal_server_died", 1)
	ev.waitFor(t, "terminal_server_restarted", 1)

	if got := *currentPort; got != ts.port {
		t.Fatalf("expected terminal port restored to %d after restart, got %d", ts.port, got)
	}

	// Clean up the loop.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ts.Shutdown(ctx)
}

// TestSupervisor_GivesUpAfterMaxAttempts verifies bounded restart attempts: if
// every rebind fails, the supervisor logs a giveup and stops (never spins).
func TestSupervisor_GivesUpAfterMaxAttempts(t *testing.T) {
	initialDone := make(chan struct{})
	ts, ev, _ := newTestSupervisor(t, initialDone)

	var attempts int
	var amu sync.Mutex
	ts.deps.Start = func(int, *http.ServeMux) (*http.Server, <-chan struct{}, error) {
		amu.Lock()
		attempts++
		amu.Unlock()
		return nil, nil, context.DeadlineExceeded // always fails to bind
	}

	done := make(chan struct{})
	go func() { ts.supervise(ts.initialDone); close(done) }()

	close(initialDone) // trigger death -> restart attempts

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("supervisor did not give up / exit after exhausting restart attempts")
	}

	if ev.count("terminal_server_restart_giveup") != 1 {
		t.Fatalf("expected exactly one giveup event, got %d", ev.count("terminal_server_restart_giveup"))
	}
	if ev.count("terminal_server_restart_failed") != restartMaxAttempts {
		t.Fatalf("expected %d failed attempts, got %d", restartMaxAttempts, ev.count("terminal_server_restart_failed"))
	}
	amu.Lock()
	defer amu.Unlock()
	if attempts != restartMaxAttempts {
		t.Fatalf("expected %d bind attempts, got %d", restartMaxAttempts, attempts)
	}
}

// TestSupervisor_NoRestartDuringShutdown verifies the graceful-shutdown race:
// when shutdown closes the current server's done channel AND signals stop, the
// supervisor must NOT treat it as an unexpected death or attempt a restart.
func TestSupervisor_NoRestartDuringShutdown(t *testing.T) {
	initialDone := make(chan struct{})
	ts, ev, _ := newTestSupervisor(t, initialDone)

	var restartAttempted bool
	var rmu sync.Mutex
	ts.deps.Start = func(int, *http.ServeMux) (*http.Server, <-chan struct{}, error) {
		rmu.Lock()
		restartAttempted = true
		rmu.Unlock()
		return &http.Server{}, make(chan struct{}), nil
	}
	// The current server is what shutdown() will close via Shutdown(); emulate
	// that by closing initialDone inside a fake srv Shutdown is not available,
	// so drive the race directly: set srv nil and close initialDone under stop.
	ts.srv = nil

	loopDone := make(chan struct{})
	go func() { ts.supervise(ts.initialDone); close(loopDone) }()

	// Simulate graceful shutdown: signal stop, then close the current done.
	close(ts.stop)
	close(initialDone)

	select {
	case <-loopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("supervise loop did not exit on shutdown")
	}

	rmu.Lock()
	defer rmu.Unlock()
	if restartAttempted {
		t.Fatal("supervisor must not restart the terminal server during graceful shutdown")
	}
	if ev.count("terminal_server_died") != 0 {
		t.Fatal("shutdown must not be logged as an unexpected death")
	}
}

// TestSupervisor_RestartRacingShutdownDoesNotPublishNewServer covers the window
// where shutdown() closes `stop` WHILE a restart is mid-bind: shutdown has already
// sampled the old srv, so the freshly-bound server must be closed and NOT
// published (otherwise its listener/goroutines leak until process exit). The only
// path that returns false after a successful bind is the race branch, which closes
// the new server — so "not published + loop exits" proves the new server was torn
// down rather than left orphaned.
func TestSupervisor_RestartRacingShutdownDoesNotPublishNewServer(t *testing.T) {
	initialDone := make(chan struct{})
	ts, ev, currentPort := newTestSupervisor(t, initialDone)

	ts.deps.Start = func(int, *http.ServeMux) (*http.Server, <-chan struct{}, error) {
		// Shutdown arrives while we are binding (after the backoff select passed).
		ts.stopOnce.Do(func() { close(ts.stop) })
		return &http.Server{}, make(chan struct{}), nil
	}

	loopDone := make(chan struct{})
	go func() { ts.supervise(ts.initialDone); close(loopDone) }()
	close(initialDone) // trigger death -> restart attempt that races shutdown

	select {
	case <-loopDone:
	case <-time.After(3 * time.Second):
		t.Fatal("supervise did not exit when shutdown raced the restart")
	}
	if ev.count("terminal_server_restarted") != 0 {
		t.Fatalf("a server bound after shutdown must NOT be published as restarted, got %d", ev.count("terminal_server_restarted"))
	}
	if got := *currentPort; got != 0 {
		t.Fatalf("terminal port must stay 0 when shutdown raced the restart, got %d", got)
	}
}
