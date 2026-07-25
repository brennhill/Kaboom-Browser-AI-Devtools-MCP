// terminal_supervisor_test.go -- Deterministic tests for the terminal-server
// auto-restart supervisor. All timing/binding is injected so the tests never
// touch a real port or sleep.

package main

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"
)

// newTestSupervisor builds a supervisor with all external seams stubbed and an
// event recorder wired to logFn.
func newTestSupervisor(t *testing.T, initialDone <-chan struct{}) (*terminalSupervisor, *supEvents) {
	t.Helper()
	ev := &supEvents{}
	ts := &terminalSupervisor{
		server:      &Server{},
		port:        7891,
		startFn:     func(int, *http.ServeMux) (*http.Server, <-chan struct{}, error) { return nil, nil, nil },
		reclaimFn:   func(int) {},
		logFn:       ev.record,
		warnFn:      func(string, ...any) {},
		afterFn:     func(time.Duration) <-chan time.Time { return closedTimer() }, // no real sleep
		stop:        make(chan struct{}),
		loopDone:    make(chan struct{}),
		initialDone: initialDone,
	}
	return ts, ev
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
	ts, ev := newTestSupervisor(t, initialDone)

	// startFn succeeds and hands back a fresh (never-closed) done channel, so the
	// supervisor settles into watching a healthy restarted server.
	restarted := make(chan struct{})
	ts.startFn = func(int, *http.ServeMux) (*http.Server, <-chan struct{}, error) {
		return &http.Server{}, restarted, nil
	}

	go ts.supervise(ts.initialDone)

	// Kill the initial server.
	close(initialDone)

	ev.waitFor(t, "terminal_server_died", 1)
	ev.waitFor(t, "terminal_server_restarted", 1)

	if got := ts.server.getTerminalPort(); got != ts.port {
		t.Fatalf("expected terminal port restored to %d after restart, got %d", ts.port, got)
	}

	// Clean up the loop.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ts.shutdown(ctx)
}

// TestSupervisor_GivesUpAfterMaxAttempts verifies bounded restart attempts: if
// every rebind fails, the supervisor logs a giveup and stops (never spins).
func TestSupervisor_GivesUpAfterMaxAttempts(t *testing.T) {
	initialDone := make(chan struct{})
	ts, ev := newTestSupervisor(t, initialDone)

	var attempts int
	var amu sync.Mutex
	ts.startFn = func(int, *http.ServeMux) (*http.Server, <-chan struct{}, error) {
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
	if ev.count("terminal_server_restart_failed") != terminalRestartMaxAttempts {
		t.Fatalf("expected %d failed attempts, got %d", terminalRestartMaxAttempts, ev.count("terminal_server_restart_failed"))
	}
	amu.Lock()
	defer amu.Unlock()
	if attempts != terminalRestartMaxAttempts {
		t.Fatalf("expected %d bind attempts, got %d", terminalRestartMaxAttempts, attempts)
	}
}

// TestSupervisor_NoRestartDuringShutdown verifies the graceful-shutdown race:
// when shutdown closes the current server's done channel AND signals stop, the
// supervisor must NOT treat it as an unexpected death or attempt a restart.
func TestSupervisor_NoRestartDuringShutdown(t *testing.T) {
	initialDone := make(chan struct{})
	ts, ev := newTestSupervisor(t, initialDone)

	var restartAttempted bool
	var rmu sync.Mutex
	ts.startFn = func(int, *http.ServeMux) (*http.Server, <-chan struct{}, error) {
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
