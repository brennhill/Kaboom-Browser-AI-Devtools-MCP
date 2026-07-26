// daemon_lifecycle_wiring.go — adapts this package's Server and process/port seams to the daemonlife dependency contract.
// Why: daemonlife owns the single-instance policy but not the primitives it acts through; this is the one place those are bound.

package main

import (
	"context"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/daemonlife"
)

// lifecycleLogger adapts *Server to daemonlife.Logger so the package never has to
// know about the server type.
type lifecycleLogger struct{ server *Server }

func (l lifecycleLogger) LogLifecycle(event string, port int, fields map[string]any) {
	l.server.logLifecycle(event, port, fields)
}

// daemonlifeDeps binds this package's process/port seams for daemonlife. It is
// built fresh at every call site so the injectable `daemon*` vars (and `version`,
// which tests swap) are read at call time, exactly as when this code lived here.
//
// Every func field must be non-nil — daemonlife calls them unconditionally.
// TestDaemonlifeDeps_AllSeamsWired is the regression guard for that.
func daemonlifeDeps(server *Server) daemonlife.Deps {
	return daemonlife.Deps{
		Log:     lifecycleLogger{server: server},
		Version: version,
		Warnf:   stderrf,

		IsProcessAlive:     daemonIsProcessAlive,
		IsServerRunning:    daemonIsServerRunning,
		TryShutdown:        daemonTryShutdown,
		WaitForPortRelease: daemonWaitForPortRelease,
		TerminatePID:       daemonTerminatePID,
		FetchHealth:        fetchDaemonHealth,
		ReadPIDFile:        procctl.ReadPIDFile,
		RemovePIDFile:      procctl.RemovePIDFile,
	}
}

// fetchDaemonHealth performs one /health probe and reduces it to the three facts
// the takeover policy needs: did it answer, what version did it claim, and was the
// connection refused (nothing listening at all).
func fetchDaemonHealth(ctx context.Context, port int, timeout time.Duration) (reachable bool, version string, refused bool) {
	h := fetchInstallHealth(ctx, port, timeout)
	return h.reachable, h.version, h.refused
}
