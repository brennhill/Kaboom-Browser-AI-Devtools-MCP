// Purpose: Orchestrates MCP daemon startup wiring and runtime lifecycle.
// Why: Keeps top-level daemon flow readable while delegating setup/shutdown details to focused modules.

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/binarywatch"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/daemonhttp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/daemonlife"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/httpapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/httpguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal"
	terminalintent "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/intent"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/sessionrelay"
	terminalsupervisor "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/supervisor"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/wsframe"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/settingscache"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session/clientreg"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

func terminalDeps() terminal.Deps {
	return terminal.Deps{
		JSONResponse:   httpapi.JSON,
		CORSMiddleware: httpguard.CORS,
		Stderrf:        diag.Printf,
		MaxPostBody:    maxPostBodySize,
		WSReadFrame:    wsframe.ReadFrame,
		WSWriteFrame:   wsframe.WriteFrame,
		WSAcceptKey:    wsframe.AcceptKey,
	}
}

func setupTerminalMux(server *Server, manager *pty.Manager, store *capture.Capture) (*http.ServeMux, *sessionrelay.Map) {
	deps := terminalDeps()
	deps.LogEvent = func(event string, fields map[string]any) { server.logLifecycle(event, 0, fields) }
	return terminal.SetupMux(deps, server.activeCodebase, terminalintent.Runtime{
		Relays: func() terminalintent.RelayMap { return server.ptyRelays },
		Store:  func() *terminalintent.Store { return server.intentStore },
	}, manager, store)
}

func startTerminalServer(port int, mux *http.ServeMux) (*http.Server, <-chan struct{}, error) {
	return terminal.StartServer(terminalDeps(), port, mux)
}

func handleActiveCodebase(w http.ResponseWriter, r *http.Request, server *Server) {
	terminal.HandleActiveCodebase(w, r, terminalDeps(), server.activeCodebase)
}

// runMCPMode runs the server in MCP mode:
// - HTTP server runs in a goroutine (for browser extension)
// - MCP protocol runs over stdin/stdout (for Claude Code)
// If stdin closes (EOF), the HTTP server keeps running until killed.
// Returns error if port binding fails (race condition with another client).
// Never returns on success (blocks forever serving MCP protocol).
func runMCPMode(server *Server, port int, apiKey string, opts daemonlife.LaunchOptions) error {
	server.listenPort.Set(port)
	cap := initCapture(server, port)
	mux, mcpHandler := setupHTTPRoutes(server, cap)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server.runtime.ReleaseChecker().Start(ctx)
	server.startScreenshotRateLimiterCleanup(ctx)
	startBinaryUpgradeWatcher(server, port, ctx)
	reportUpgradeMarker(server, port)

	policyErr := daemonlife.EnforceStartupPolicy(server.daemonRecovery.LifecycleDeps(), port, opts)
	if err := handleStartupPolicyResult(server, port, policyErr); err != nil {
		return err
	}

	if err := server.daemonRecovery.CleanupStalePIDFile(port); err != nil {
		return err
	}
	httpDeps := daemonHTTPDeps(server)
	if err := ensurePortAvailable(server, httpDeps, port, "main"); err != nil {
		return err
	}

	// Identity and its canonical diagnostics must be ready before lifecycle
	// assessment can publish an incident or the listener can accept a request.
	telemetry.Warm(server.incidents)

	// Crash-loop self-defense: if this SAME install (version + epoch) has restarted
	// too many times too fast, log loudly and back off a bounded amount before binding
	// so a pathological loop degrades gracefully instead of hammering launchd (which
	// would throttle/disable the LaunchAgent and take the terminal server on port+1
	// dark too). Never refuses to start; an upgrade/epoch takeover resets the counter.
	daemonlife.ApplyStartupRestartThrottle(server.daemonRecovery.LifecycleDeps(), port)

	srv, httpDone, err := daemonhttp.Start(httpDeps, port, apiKey, mux)
	if err != nil {
		return err
	}
	persistDaemonRuntimeState(server, port)

	termPort, termSrv, _ := startTerminalServerWithRecovery(server, cap, port)

	server.logLifecycle("startup", port, map[string]any{
		"version":       version,
		"go_version":    runtime.Version(),
		"os":            runtime.GOOS,
		"arch":          runtime.GOARCH,
		"terminal_port": termPort,
	})
	server.logLifecycle("mcp_transport_ready", port, nil)

	// Start periodic usage beacon loop (structured tool stats every 5 minutes).
	if tracker := mcpHandler.usageTracker; tracker != nil {
		telemetry.StartUsageBeaconLoop(ctx, tracker)
	}

	awaitShutdownSignal(server, srv, port, httpDone, termSrv, mcpHandler)
	return nil
}

// startBinaryUpgradeWatcher watches for a newer installed binary and, on
// detection, warns and then self-terminates so the new binary takes over.
func startBinaryUpgradeWatcher(server *Server, port int, ctx context.Context) {
	server.runtime.SetUpgrade(binarywatch.Start(ctx, version,
		func(newVersion string) {
			server.logLifecycle("binary_upgrade_detected", port, map[string]any{
				"current_version": version, "new_version": newVersion,
			})
			server.warnings.Add("UPGRADE DETECTED: v" + newVersion + " installed. Auto-restart in ~5s.")
		},
		func() {
			if server.runtime.Upgrade() != nil {
				if _, newVersion, _ := server.runtime.Upgrade().UpgradeInfo(); newVersion != "" {
					if markerPath, err := state.UpgradeMarkerFile(); err == nil {
						_ = binarywatch.WriteMarker(version, newVersion, markerPath)
					}
				}
			}
			server.logLifecycle("binary_upgrade_shutdown", port, map[string]any{"version": version})
			process, _ := os.FindProcess(os.Getpid())
			_ = process.Signal(syscall.SIGTERM)
		},
	))
}

// reportUpgradeMarker announces a just-completed binary upgrade (if any marker
// was left by the previous version) and clears the marker.
func reportUpgradeMarker(server *Server, port int) {
	if markerPath, err := state.UpgradeMarkerFile(); err == nil {
		if marker, markerErr := binarywatch.ReadAndClearMarker(markerPath); markerErr == nil && marker != nil {
			server.warnings.Add(fmt.Sprintf("Upgraded from v%s to v%s", marker.FromVersion, marker.ToVersion))
			server.logLifecycle("binary_upgrade_complete", port, map[string]any{
				"from_version": marker.FromVersion, "to_version": marker.ToVersion,
			})
		}
	}
}

// handleStartupPolicyResult interprets EnforceStartupPolicy's outcome: deferring
// to a healthy incumbent daemon logs, names the incumbent, and reports success
// (clean exit); every other error propagates.
func handleStartupPolicyResult(server *Server, port int, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, daemonlife.ErrDeferToHealthyDaemon) {
		// A healthy, compatible daemon already owns this port. Exit cleanly
		// (exit 0) and let it keep serving — do NOT start a rival server or
		// kill the incumbent. Returning nil unwinds to a graceful exit.
		server.logLifecycle("daemon_deferred_exit", port, nil)
		// Name the incumbent, not the port we were asked for: the lock is
		// per state directory, so the daemon we are deferring to may be on
		// a different port entirely, and saying otherwise sends an operator
		// looking for a listener that was never there.
		var deferral *daemonlife.Deferral
		if errors.As(err, &deferral) {
			diag.Printf("[Kaboom] Deferring to the daemon already registered for this state directory (pid=%d, port=%d, version=%s); this instance is exiting.\n",
				deferral.PID, deferral.Port, deferral.Version)
		} else {
			diag.Printf("[Kaboom] A healthy daemon is already serving on port %d; this instance is exiting.\n", port)
		}
		return nil
	}
	return err
}

// ensurePortAvailable verifies the port is bindable; a leftover holder is
// reclaimed once and the check retried before giving up.
func ensurePortAvailable(server *Server, deps daemonhttp.Deps, port int, purpose string) error {
	if err := daemonhttp.Preflight(deps, port); err != nil {
		// A leftover process holds the main port and the lock-based takeover did
		// not clear it. Reclaim it (find + log + kill) so we stay single-instance,
		// then re-check; only abort if it is still stuck.
		server.logLifecycle("port_reclaim_attempt", port, map[string]any{"purpose": purpose, "reason": err.Error()})
		server.daemonRecovery.ReclaimPort(port, purpose)
		if err := daemonhttp.Preflight(deps, port); err != nil {
			return err
		}
	}
	return nil
}

// startTerminalServerWithRecovery starts the dedicated terminal server on
// port+1, reclaiming the port once if a leftover process holds it, and records
// availability either way. Non-fatal: the daemon continues without terminal
// features when the port cannot be bound.
func startTerminalServerWithRecovery(server *Server, cap *capture.Capture, port int) (int, *http.Server, <-chan struct{}) {
	termPort := port + terminal.PortOffset
	termMux, termRelays := setupTerminalMux(server, server.ptyManager, cap)
	server.ptyRelays = termRelays
	termSrv, termDone, termErr := startTerminalServer(termPort, termMux)
	if termErr != nil {
		// A leftover process holds the terminal port — this is the exact state that
		// makes 7891 "connection refused" for the extension (can't type / Start
		// fails). Reclaim it and retry once so the terminal server reliably binds.
		server.logLifecycle("terminal_server_bind_retry", termPort, map[string]any{"error": termErr.Error()})
		if server.daemonRecovery.ReclaimPort(termPort, "terminal") {
			termSrv, termDone, termErr = startTerminalServer(termPort, termMux)
		}
	}
	if termErr != nil {
		recordTerminalUnavailable(server, termPort, termErr)
	} else {
		wireTerminalSupervisor(server, termPort, termMux, termSrv, termDone)
	}
	return termPort, termSrv, termDone
}

// recordTerminalUnavailable reports a terminal-server bind failure into the
// health payload and stderr, naming whoever holds the port.
func recordTerminalUnavailable(server *Server, termPort int, termErr error) {
	// Identify whoever holds the port. "Port busy" is not actionable; "postgres
	// (pid 4242) is on 7891" is. This is recorded into /health because the stderr
	// lines below go to /dev/null for a bridge-spawned daemon (spawned with
	// Stdout/Stderr = nil so it cannot die of SIGPIPE), making the health payload
	// the only place a user or agent can learn what happened.
	blockingPID, blockingCmd := server.daemonRecovery.IdentifyPortHolder(termPort)
	server.terminalStatus.SetUnavailable(termPort, termErr.Error(), blockingPID, blockingCmd)

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
}

// wireTerminalSupervisor marks the terminal port live and supervises the
// terminal server — restarting it with backoff if it dies unexpectedly, so a
// transient terminal-server death does not leave the terminal permanently dead
// until a full daemon restart. Never brings down the main daemon; never
// restarts during graceful shutdown.
func wireTerminalSupervisor(server *Server, termPort int, termMux *http.ServeMux, termSrv *http.Server, termDone <-chan struct{}) {
	server.terminalStatus.SetPort(termPort)
	server.logLifecycle("terminal_server_started", termPort, nil)
	sup := terminalsupervisor.New(terminalsupervisor.Dependencies{
		Start:   startTerminalServer,
		Reclaim: func(port int) { server.daemonRecovery.ReclaimPort(port, "terminal") },
		SetPort: server.terminalStatus.SetPort,
		Log:     func(event string, fields map[string]any) { server.logLifecycle(event, termPort, fields) },
		Warn:    diag.Printf,
	}, termPort, termMux, termSrv, termDone)
	server.terminalSupervisor = sup
	sup.Start()
}

// initCapture creates and configures the capture buffers with lifecycle logging.
func initCapture(server *Server, port int) *capture.Capture {
	cap := capture.NewCapture()
	cap.Clients().Set(clientreg.NewClientRegistry())
	cap.Extension().SetServerVersion(version)
	cap.Lifecycle().Subscribe(func(event lifecycle.Event, data map[string]any) {
		entry := types.LogEntry{
			"type":      "lifecycle",
			"event":     event.String(),
			"pid":       os.Getpid(),
			"port":      port,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		for k, v := range data {
			entry[k] = v
		}
		server.logs.AddEntries([]types.LogEntry{entry})
	})

	server.logLifecycle("loading_settings", port, nil)
	if err := settingscache.Load(cap.Extension().ApplyCachedPilot, server.stateRecovery); err != nil {
		server.logLifecycle("settings_load_failed", port, map[string]any{"reason": "settings_loader_boundary_invalid"})
	}
	server.logLifecycle("settings_loaded", port, nil)
	return cap
}

// startScreenshotRateLimiterCleanup starts a background goroutine that removes
// stale entries from the screenshot rate limiter every 30 seconds.
func (s *Server) startScreenshotRateLimiterCleanup(ctx context.Context) {
	util.SafeGo(func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.mediaHTTP.CleanupRateLimits(time.Now(), time.Minute)
			}
		}
	})
}

func daemonHTTPDeps(server *Server) daemonhttp.Deps {
	return daemonhttp.Deps{
		IdentifyPortHolder: server.daemonRecovery.IdentifyPortHolder,
		LogLifecycle:       server.logLifecycle,
		RecordFailure: func(event string, fields map[string]any) {
			_ = server.runtime.ExitDiagnostics().Append(event, fields)
		},
		Diagnosticf: diag.Printf,
	}
}

// persistDaemonRuntimeState records process metadata used by lifecycle/stop flows.
func persistDaemonRuntimeState(server *Server, port int) {
	if err := procctl.WritePIDFile(port); err != nil {
		server.logLifecycle("pid_file_error", port, map[string]any{"error": err.Error()})
	}
	if err := daemonlife.PersistCurrentLock(port, version, server.stateRecovery); err != nil {
		server.logLifecycle("daemon_lock_write_failed", port, map[string]any{"error": err.Error()})
	}
}

const (
	terminalShutdownTimeout = 2 * time.Second
	httpShutdownTimeout     = 3 * time.Second
	asyncLoggerDrainTimeout = 2 * time.Second
)

// awaitShutdownSignal blocks until a termination signal is received or the
// HTTP listener dies unexpectedly, then performs graceful cleanup.
func awaitShutdownSignal(server *Server, srv *http.Server, port int, httpDone <-chan struct{}, termSrv *http.Server, mcpHandler *ToolHandler) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	var shutdownSignal os.Signal
	var shutdownSource string
	select {
	case shutdownSignal = <-sigCh:
		shutdownSource = daemonlife.SignalSource(shutdownSignal)
	case <-httpDone:
		shutdownSource = "http_listener_died"
		shutdownSignal = syscall.SIGTERM
		diag.Printf("[Kaboom] HTTP listener exited unexpectedly, shutting down to avoid zombie process\n")
	}

	server.logLifecycle("shutdown", port, map[string]any{
		"signal":          shutdownSignal.String(),
		"shutdown_source": shutdownSource,
		"uptime_seconds":  time.Since(server.runtime.StartedAt()).Seconds(),
	})
	if shutdownSource != "http_listener_died" {
		daemonlife.ClearRestartHistoryOnCleanShutdown(server.daemonRecovery.LifecycleDeps(), port)
	}
	if diagPath := server.runtime.ExitDiagnostics().Append("daemon_shutdown", map[string]any{
		"port":            port,
		"signal":          shutdownSignal.String(),
		"shutdown_source": shutdownSource,
		"uptime_seconds":  time.Since(server.runtime.StartedAt()).Seconds(),
		"unexpected":      shutdownSource == "http_listener_died",
	}); diagPath != "" && shutdownSource == "http_listener_died" {
		diag.Printf("[Kaboom] Shutdown diagnostics written to: %s\n", diagPath)
	}

	shutdownTerminalServer(server, termSrv, port)
	closeToolHandler(mcpHandler)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), httpShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		server.logLifecycle("http_shutdown_error", port, map[string]any{"error": err.Error()})
	}

	server.logs.Shutdown(asyncLoggerDrainTimeout)
	server.annotationRuntime.Close()
	closeCaptureStore(mcpHandler)
	closeTerminalResources(server)
	persistTokenSavings(server)

	procctl.RemovePIDFile(port)
	if err := daemonlife.RemoveLockIfOwned(os.Getpid()); err != nil {
		server.logLifecycle("daemon_lock_cleanup_failed", port, map[string]any{"reason": "state_remove_failed"})
	}
}

func shutdownTerminalServer(server *Server, termSrv *http.Server, port int) {
	termCtx, termCancel := context.WithTimeout(context.Background(), terminalShutdownTimeout)
	defer termCancel()
	if server.terminalSupervisor != nil {
		server.terminalSupervisor.Shutdown(termCtx)
		return
	}
	if termSrv != nil {
		if err := termSrv.Shutdown(termCtx); err != nil {
			server.logLifecycle("terminal_shutdown_error", port, map[string]any{"error": err.Error()})
		}
	}
}

func closeToolHandler(handler *ToolHandler) {
	if handler == nil {
		return
	}
	handler.Close()
}

func closeCaptureStore(handler *ToolHandler) {
	if handler == nil || handler.capture == nil {
		return
	}
	handler.capture.Close()
}

func closeTerminalResources(server *Server) {
	if server.ptyManager != nil {
		server.ptyManager.StopAll()
	}
	if server.ptyRelays != nil {
		server.ptyRelays.CloseAll()
	}
}

func persistTokenSavings(server *Server) {
	if server.tokenTracker == nil {
		return
	}
	if summary := server.tokenTracker.GetSessionSummary(); summary != "" {
		diag.Printf("[Kaboom] %s", summary)
	}
	if root, err := state.RootDir(); err == nil {
		lifetimePath := filepath.Join(root, "stats", "lifetime.json")
		if err := server.tokenTracker.SaveLifetime(lifetimePath); err != nil {
			diag.Printf("[Kaboom] Failed to save lifetime token stats: %v\n", err)
		}
	}
}
