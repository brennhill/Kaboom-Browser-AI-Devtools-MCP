// Purpose: Orchestrates MCP daemon startup wiring and runtime lifecycle.
// Why: Keeps top-level daemon flow readable while delegating setup/shutdown details to focused modules.

package main

import (
	"context"
	"fmt"
	"runtime"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// runMCPMode runs the server in MCP mode:
// - HTTP server runs in a goroutine (for browser extension)
// - MCP protocol runs over stdin/stdout (for Claude Code)
// If stdin closes (EOF), the HTTP server keeps running until killed.
// Returns error if port binding fails (race condition with another client).
// Never returns on success (blocks forever serving MCP protocol).
func runMCPMode(server *Server, port int, apiKey string, opts daemonLaunchOptions) error {
	server.setListenPort(port)
	cap := initCapture(server, port)
	mux, mcpHandler := setupHTTPRoutes(server, cap)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startVersionCheckLoop(ctx)
	server.startScreenshotRateLimiterCleanup(ctx)
	configureBinaryUpgradeMonitoring(ctx, server, port)

	if err := enforceDaemonStartupPolicy(server, port, opts); err != nil {
		return err
	}

	if err := cleanupStalePIDFile(server, port); err != nil {
		return err
	}
	if err := preflightPortCheck(server, port); err != nil {
		// A leftover process holds the main port and the lock-based takeover did
		// not clear it. Reclaim it (find + log + kill) so we stay single-instance,
		// then re-check; only abort if it is still stuck.
		server.logLifecycle("port_reclaim_attempt", port, map[string]any{"purpose": "main", "reason": err.Error()})
		reclaimPort(server, port, "main")
		if err := preflightPortCheck(server, port); err != nil {
			return err
		}
	}

	srv, httpDone, err := startHTTPServer(server, port, apiKey, mux)
	if err != nil {
		return err
	}
	persistDaemonRuntimeState(server, port)

	// Start dedicated terminal server on port+1.
	// Non-fatal: if the terminal port is busy, log a warning and continue without terminal.
	termPort := port + terminal.PortOffset
	termMux, termRelays := setupTerminalMux(server, server.ptyManager, cap)
	server.ptyRelays = termRelays
	termSrv, termDone, termErr := startTerminalServer(termPort, termMux)
	if termErr != nil {
		// A leftover process holds the terminal port — this is the exact state that
		// makes 7891 "connection refused" for the extension (can't type / Start
		// fails). Reclaim it and retry once so the terminal server reliably binds.
		server.logLifecycle("terminal_server_bind_retry", termPort, map[string]any{"error": termErr.Error()})
		if reclaimPort(server, termPort, "terminal") {
			termSrv, termDone, termErr = startTerminalServer(termPort, termMux)
		}
	}
	if termErr != nil {
		stderrf("[Kaboom] WARNING: terminal server failed to start on port %d: %v\n", termPort, termErr)
		stderrf("[Kaboom] Terminal features are unavailable. Free port %d or use a different base port.\n", termPort)
		server.logLifecycle("terminal_server_bind_failed", termPort, map[string]any{
			"error":     termErr.Error(),
			"term_port": termPort,
		})
	} else {
		server.setTerminalPort(termPort)
		server.logLifecycle("terminal_server_started", termPort, nil)
		// Monitor terminal server — log if it dies, but do NOT bring down main daemon.
		util.SafeGo(func() {
			<-termDone
			stderrf("[Kaboom] terminal server on port %d exited unexpectedly\n", termPort)
			server.logLifecycle("terminal_server_died", termPort, nil)
			server.setTerminalPort(0) // Mark as unavailable
		})
	}

	server.logLifecycle("startup", port, map[string]any{
		"version":       version,
		"go_version":    runtime.Version(),
		"os":            runtime.GOOS,
		"arch":          runtime.GOARCH,
		"terminal_port": termPort,
	})
	server.logLifecycle("mcp_transport_ready", port, nil)

	telemetry.Warm() // Pre-load install ID and session off the hot path.
	telemetry.BeaconEvent("daemon_start", map[string]string{
		"mode": "daemon",
		"port": fmt.Sprintf("%d", port),
	})

	// Start periodic usage beacon loop (structured tool stats every 5 minutes).
	if tracker := mcpHandler.GetUsageTracker(); tracker != nil {
		telemetry.StartUsageBeaconLoop(ctx, tracker)
	}

	awaitShutdownSignal(server, srv, port, httpDone, termSrv, termDone, mcpHandler)
	return nil
}
