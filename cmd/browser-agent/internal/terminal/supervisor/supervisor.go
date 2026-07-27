// supervisor.go — Restarts the terminal HTTP server after unexpected exits.

package supervisor

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const (
	restartInitialBackoff = 500 * time.Millisecond
	restartMaxBackoff     = 30 * time.Second
	restartMaxAttempts    = 8
)

// Dependencies are the host operations required by terminal supervision.
type Dependencies struct {
	Start   func(port int, mux *http.ServeMux) (*http.Server, <-chan struct{}, error)
	Reclaim func(port int)
	SetPort func(port int)
	Log     func(event string, fields map[string]any)
	Warn    func(format string, args ...any)
	After   func(duration time.Duration) <-chan time.Time
}

// Supervisor owns restart and graceful-shutdown lifecycle for one terminal server.
type Supervisor struct {
	deps Dependencies
	port int
	mux  *http.ServeMux

	stop        chan struct{}
	stopOnce    sync.Once
	loopDone    chan struct{}
	initialDone <-chan struct{}

	mu  sync.Mutex
	srv *http.Server
}

// New creates a supervisor for an already-bound terminal server.
func New(deps Dependencies, port int, mux *http.ServeMux, initialServer *http.Server, initialDone <-chan struct{}) *Supervisor {
	if deps.After == nil {
		deps.After = time.After
	}
	return &Supervisor{
		deps:        deps,
		port:        port,
		mux:         mux,
		stop:        make(chan struct{}),
		loopDone:    make(chan struct{}),
		initialDone: initialDone,
		srv:         initialServer,
	}
}

// Start begins supervising in a panic-contained goroutine.
func (supervisor *Supervisor) Start() {
	util.SafeGo(func() { supervisor.supervise(supervisor.initialDone) })
}

func (supervisor *Supervisor) supervise(done <-chan struct{}) {
	defer close(supervisor.loopDone)
	for {
		select {
		case <-supervisor.stop:
			return
		case <-done:
			select {
			case <-supervisor.stop:
				return
			default:
			}
		}

		supervisor.deps.SetPort(0)
		supervisor.deps.Log("terminal_server_died", nil)
		supervisor.deps.Warn("[Kaboom] terminal server on port %d exited unexpectedly; attempting restart\n", supervisor.port)

		restarted, ok := supervisor.restartWithBackoff()
		if !ok {
			return
		}
		done = restarted
	}
}

func (supervisor *Supervisor) restartWithBackoff() (<-chan struct{}, bool) {
	backoff := restartInitialBackoff
	for attempt := 1; attempt <= restartMaxAttempts; attempt++ {
		select {
		case <-supervisor.stop:
			return nil, false
		case <-supervisor.deps.After(backoff):
		}

		supervisor.deps.Reclaim(supervisor.port)
		server, done, err := supervisor.deps.Start(supervisor.port, supervisor.mux)
		if err == nil {
			supervisor.mu.Lock() // lint:manual-unlock -- shutdown-race branch unlocks before return.
			select {
			case <-supervisor.stop:
				supervisor.mu.Unlock()
				_ = server.Close()
				return nil, false
			default:
				supervisor.srv = server
			}
			supervisor.mu.Unlock()
			supervisor.deps.SetPort(supervisor.port)
			supervisor.deps.Log("terminal_server_restarted", map[string]any{"attempt": attempt})
			supervisor.deps.Warn("[Kaboom] terminal server restarted on port %d (attempt %d)\n", supervisor.port, attempt)
			return done, true
		}

		supervisor.deps.Log("terminal_server_restart_failed", map[string]any{"attempt": attempt, "error": err.Error()})
		backoff *= 2
		if backoff > restartMaxBackoff {
			backoff = restartMaxBackoff
		}
	}

	supervisor.deps.Log("terminal_server_restart_giveup", map[string]any{"attempts": restartMaxAttempts})
	supervisor.deps.Warn("[Kaboom] terminal server on port %d could not be restarted after %d attempts; terminal features unavailable until daemon restart\n", supervisor.port, restartMaxAttempts)
	return nil, false
}

// Shutdown prevents further restarts, closes the current server, and waits for supervision to stop.
func (supervisor *Supervisor) Shutdown(ctx context.Context) {
	supervisor.stopOnce.Do(func() { close(supervisor.stop) })

	supervisor.mu.Lock() // lint:manual-unlock -- copy the current server without holding the lock during Shutdown.
	server := supervisor.srv
	supervisor.mu.Unlock()
	if server != nil {
		_ = server.Shutdown(ctx)
	}

	select {
	case <-supervisor.loopDone:
	case <-ctx.Done():
	}
}
