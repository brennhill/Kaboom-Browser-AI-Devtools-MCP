// server.go — Owns daemon HTTP listener preflight, binding, and bounded server configuration.

package daemonhttp

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/httpguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// Deps supplies diagnostics that couple listener failures to daemon recovery.
type Deps struct {
	IdentifyPortHolder func(int) (int, string)
	LogLifecycle       func(string, int, map[string]any)
	RecordFailure      func(string, map[string]any)
	Diagnosticf        func(string, ...any)
}

func (deps Deps) normalized() Deps {
	if deps.IdentifyPortHolder == nil {
		deps.IdentifyPortHolder = func(int) (int, string) { return 0, "" }
	}
	if deps.LogLifecycle == nil {
		deps.LogLifecycle = func(string, int, map[string]any) {}
	}
	if deps.RecordFailure == nil {
		deps.RecordFailure = func(string, map[string]any) {}
	}
	if deps.Diagnosticf == nil {
		deps.Diagnosticf = diag.Printf
	}
	return deps
}

// Preflight verifies that the loopback daemon port can be bound and describes
// its current owner when it cannot.
func Preflight(deps Deps, port int) error {
	deps = deps.normalized()
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		blockingPID, blockingCommand := deps.IdentifyPortHolder(port)
		deps.LogLifecycle("port_conflict_detected", port, map[string]any{
			"error": err.Error(), "blocked_by_pid": blockingPID, "blocked_by_cmd": blockingCommand,
		})
		if blockingPID > 0 {
			return fmt.Errorf("port %d already in use by pid %d (%s); free that port or start Kaboom on a different one: %w", port, blockingPID, blockingCommand, err)
		}
		return fmt.Errorf("port %d already in use (owner could not be identified, try '%s'): %w", port, procctl.PortKillHintForce(port), err)
	}
	return listener.Close()
}

// Start binds and serves the loopback daemon API. Successful return guarantees
// that the listener is already bound; done closes when serving stops.
func Start(deps Deps, port int, apiKey string, handler http.Handler) (*http.Server, <-chan struct{}, error) {
	deps = deps.normalized()
	ready := make(chan error, 1)
	done := make(chan struct{})
	server := &http.Server{
		ReadTimeout: 5 * time.Second, WriteTimeout: 65 * time.Second, IdleTimeout: 120 * time.Second,
		Handler: httpguard.APIKey(apiKey)(handler),
	}
	util.SafeGo(func() {
		defer close(done)
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			ready <- err
			return
		}
		ready <- nil
		// #nosec G114 -- localhost-only MCP background server
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			fields := map[string]any{"port": port, "error": err.Error()}
			deps.RecordFailure("http_listener_error", fields)
			deps.Diagnosticf("[Kaboom] HTTP server error: %v\n", err)
		}
	})
	if err := <-ready; err != nil {
		deps.LogLifecycle("http_bind_failed", port, map[string]any{"error": err.Error()})
		return nil, nil, fmt.Errorf("cannot bind port %d: %w", port, err)
	}
	deps.LogLifecycle("http_bind_success", port, nil)
	return server, done, nil
}
