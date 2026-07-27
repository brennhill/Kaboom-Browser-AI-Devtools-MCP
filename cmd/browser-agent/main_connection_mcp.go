// Purpose: Orchestrates MCP daemon startup wiring and runtime lifecycle.
// Why: Keeps top-level daemon flow readable while delegating setup/shutdown details to focused modules.

package main

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/daemonlife"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

// runMCPMode runs the server in MCP mode:
// - HTTP server runs in a goroutine (for browser extension)
// - MCP protocol runs over stdin/stdout (for Claude Code)
// If stdin closes (EOF), the HTTP server keeps running until killed.
// Returns error if port binding fails (race condition with another client).
// Never returns on success (blocks forever serving MCP protocol).
func runMCPMode(server *Server, port int, apiKey string, opts daemonlife.LaunchOptions) error {
	server.setListenPort(port)
	cap := initCapture(server, port)
	mux, mcpHandler := setupHTTPRoutes(server, cap)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startVersionCheckLoop(ctx)
	server.startScreenshotRateLimiterCleanup(ctx)
	configureBinaryUpgradeMonitoring(ctx, server, port)

	if err := daemonlife.EnforceStartupPolicy(daemonlifeDeps(server), port, opts); err != nil {
		if errors.Is(err, daemonlife.ErrDeferToHealthyDaemon) {
			// A healthy, compatible daemon already owns this port. Exit cleanly
			// (exit 0) and let it keep serving — do NOT start a rival server or
			// kill the incumbent. Returning nil unwinds to a graceful exit.
			server.logLifecycle("daemon_deferred_exit", port, nil)
			diag.Printf("[Kaboom] A healthy daemon is already serving on port %d; this instance is exiting.\n", port)
			return nil
		}
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

	// Crash-loop self-defense: if this SAME install (version + epoch) has restarted
	// too many times too fast, log loudly and back off a bounded amount before binding
	// so a pathological loop degrades gracefully instead of hammering launchd (which
	// would throttle/disable the LaunchAgent and take the terminal server on port+1
	// dark too). Never refuses to start; an upgrade/epoch takeover resets the counter.
	daemonlife.ApplyStartupRestartThrottle(daemonlifeDeps(server), port)

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
		// Identify whoever holds the port. "Port busy" is not actionable; "postgres
		// (pid 4242) is on 7891" is. This is recorded into /health because the stderr
		// lines below go to /dev/null for a bridge-spawned daemon (spawned with
		// Stdout/Stderr = nil so it cannot die of SIGPIPE), making the health payload
		// the only place a user or agent can learn what happened.
		blockingPID, blockingCmd := identifyPortHolder(termPort)
		server.setTerminalUnavailable(termPort, termErr.Error(), blockingPID, blockingCmd)

		diag.Printf("[Kaboom] WARNING: terminal server failed to start on port %d: %v\n", termPort, termErr)
		if blockingPID > 0 {
			diag.Printf("[Kaboom] Port %d is held by pid %d (%s).\n", termPort, blockingPID, blockingCmd)
		}
		diag.Printf("[Kaboom] Terminal features are unavailable. Free port %d or use a different base port.\n", termPort)
		server.logLifecycle("terminal_server_bind_failed", termPort, map[string]any{
			"error":          termErr.Error(),
			"term_port":      termPort,
			"blocked_by_pid": blockingPID,
			"blocked_by_cmd": blockingCmd,
		})
	} else {
		server.setTerminalPort(termPort)
		server.logLifecycle("terminal_server_started", termPort, nil)
		// Supervise the terminal server — restart it with backoff if it dies
		// unexpectedly, so a transient terminal-server death does not leave the
		// terminal permanently dead until a full daemon restart. Never brings
		// down the main daemon; never restarts during graceful shutdown.
		sup := newTerminalSupervisor(server, termPort, termMux, termSrv, termDone)
		server.terminalSupervisor = sup
		sup.superviseAsync()
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
