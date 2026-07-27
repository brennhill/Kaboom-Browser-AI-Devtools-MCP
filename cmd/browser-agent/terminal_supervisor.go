// terminal_supervisor.go -- Supervises the dedicated terminal HTTP server and
// restarts it with exponential backoff if it dies unexpectedly.
// Why: A transient terminal-server death (its listener drops, Serve returns)
// previously left the terminal permanently dead until a full daemon restart —
// the monitor only logged `terminal_server_died` and set the port to 0. This
// supervisor keeps the terminal self-healing while never restarting during a
// graceful daemon shutdown and giving up loudly if restarts keep failing.

package main

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const (
	// terminalRestartInitialBackoff is the wait before the first restart attempt.
	terminalRestartInitialBackoff = 500 * time.Millisecond
	// terminalRestartMaxBackoff caps the exponential backoff between attempts.
	terminalRestartMaxBackoff = 30 * time.Second
	// terminalRestartMaxAttempts bounds consecutive restart attempts for a single
	// death before the supervisor gives up loudly (terminal marked unavailable).
	terminalRestartMaxAttempts = 8
)

// terminalSupervisor owns the lifecycle of the terminal HTTP server after its
// initial bind: it watches the current server, restarts it on unexpected death,
// and shuts it down gracefully when the daemon stops.
type terminalSupervisor struct {
	server *Server
	port   int
	mux    *http.ServeMux

	// Injectable seams (overridden in tests for determinism).
	startFn   func(port int, mux *http.ServeMux) (*http.Server, <-chan struct{}, error)
	reclaimFn func(port int)
	logFn     func(event string, fields map[string]any)
	warnFn    func(format string, args ...any)
	afterFn   func(d time.Duration) <-chan time.Time

	stop        chan struct{} // closed to request graceful shutdown (no more restarts)
	stopOnce    sync.Once
	loopDone    chan struct{}   // closed when the supervise loop fully exits
	initialDone <-chan struct{} // done channel of the initial (already-bound) server

	mu  sync.Mutex
	srv *http.Server // current live terminal server (for graceful shutdown)
}

// newTerminalSupervisor wires a supervisor for an already-bound terminal server.
// initialSrv/initialDone are the server returned by the initial startTerminalServer.
func newTerminalSupervisor(server *Server, port int, mux *http.ServeMux, initialSrv *http.Server, initialDone <-chan struct{}) *terminalSupervisor {
	ts := &terminalSupervisor{
		server:      server,
		port:        port,
		mux:         mux,
		startFn:     startTerminalServer,
		reclaimFn:   func(p int) { reclaimPort(server, p, "terminal") },
		logFn:       func(event string, fields map[string]any) { server.logLifecycle(event, port, fields) },
		warnFn:      diag.Printf,
		afterFn:     time.After,
		stop:        make(chan struct{}),
		loopDone:    make(chan struct{}),
		initialDone: initialDone,
		srv:         initialSrv,
	}
	return ts
}

// superviseAsync launches the supervise loop in a panic-recovered goroutine.
func (ts *terminalSupervisor) superviseAsync() {
	util.SafeGo(func() { ts.supervise(ts.initialDone) })
}

// supervise blocks watching the current terminal server; on unexpected death it
// restarts with backoff. It returns (closing loopDone) only when shutdown is
// requested or restarts are exhausted.
func (ts *terminalSupervisor) supervise(done <-chan struct{}) {
	defer close(ts.loopDone)
	for {
		// Phase 1: wait for the current server to die — unless shutdown is requested.
		select {
		case <-ts.stop:
			return
		case <-done:
			// Prefer shutdown over a death that raced with it: on graceful
			// shutdown, srv.Shutdown closes `done` AND stop is closed. Do not
			// treat that as an unexpected death or attempt a restart.
			select {
			case <-ts.stop:
				return
			default:
			}
		}

		ts.server.setTerminalPort(0)
		ts.logFn("terminal_server_died", nil)
		ts.warnFn("[Kaboom] terminal server on port %d exited unexpectedly; attempting restart\n", ts.port)

		newDone, ok := ts.restartWithBackoff()
		if !ok {
			return
		}
		done = newDone
	}
}

// restartWithBackoff attempts to rebind the terminal server, backing off
// exponentially between tries. Returns the new done channel and true on success,
// or (nil, false) if shutdown was requested or all attempts failed.
func (ts *terminalSupervisor) restartWithBackoff() (<-chan struct{}, bool) {
	backoff := terminalRestartInitialBackoff
	for attempt := 1; attempt <= terminalRestartMaxAttempts; attempt++ {
		select {
		case <-ts.stop:
			return nil, false
		case <-ts.afterFn(backoff):
		}

		ts.reclaimFn(ts.port)
		srv, newDone, err := ts.startFn(ts.port, ts.mux)
		if err == nil {
			ts.mu.Lock()
			// If shutdown was requested while we were binding, shutdown() has
			// already sampled the OLD ts.srv, so this fresh server would never be
			// gracefully closed (its listener/goroutines would leak until process
			// exit, cutting in-flight writes). Close it here and stop instead of
			// publishing it.
			select {
			case <-ts.stop:
				ts.mu.Unlock()
				_ = srv.Close()
				return nil, false
			default:
				ts.srv = srv
			}
			ts.mu.Unlock()
			ts.server.setTerminalPort(ts.port)
			ts.logFn("terminal_server_restarted", map[string]any{"attempt": attempt})
			ts.warnFn("[Kaboom] terminal server restarted on port %d (attempt %d)\n", ts.port, attempt)
			return newDone, true
		}

		ts.logFn("terminal_server_restart_failed", map[string]any{"attempt": attempt, "error": err.Error()})
		backoff *= 2
		if backoff > terminalRestartMaxBackoff {
			backoff = terminalRestartMaxBackoff
		}
	}

	ts.logFn("terminal_server_restart_giveup", map[string]any{"attempts": terminalRestartMaxAttempts})
	ts.warnFn("[Kaboom] terminal server on port %d could not be restarted after %d attempts; terminal features unavailable until daemon restart\n", ts.port, terminalRestartMaxAttempts)
	return nil, false
}

// shutdown requests a graceful stop: it signals the supervise loop to stop
// restarting, gracefully shuts down the current server (which unblocks the loop),
// and waits for the loop to exit so no restart races daemon teardown.
func (ts *terminalSupervisor) shutdown(ctx context.Context) {
	ts.stopOnce.Do(func() { close(ts.stop) })

	ts.mu.Lock()
	srv := ts.srv
	ts.mu.Unlock()
	if srv != nil {
		_ = srv.Shutdown(ctx)
	}

	select {
	case <-ts.loopDone:
	case <-ctx.Done():
	}
}
