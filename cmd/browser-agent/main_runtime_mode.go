// Purpose: Runtime mode detection and dispatch for daemon/bridge startup.
// Why: Keeps main entrypoint small while isolating mode policy and launch behavior.
// Docs: docs/features/architecture/index.md

package main

import (
	"fmt"
	"os"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/daemonlife"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

// detectStdinMode returns whether stdin is a TTY and the file mode for diagnostics.
func detectStdinMode() (isTTY bool, stdinMode os.FileMode) {
	stat, err := os.Stdin.Stat()
	if err == nil {
		isTTY = (stat.Mode() & os.ModeCharDevice) != 0
		stdinMode = stat.Mode()
	}
	return isTTY, stdinMode
}

// selectRuntimeMode decides how to run based on flags.
// Default is bridge mode so MCP startup is reliable regardless of PTY/stdio behavior.
func selectRuntimeMode(cfg *serverConfig, _ bool) runtimeMode {
	if cfg.bridgeMode {
		return modeBridge
	}
	if cfg.daemonMode {
		return modeDaemon
	}
	return modeBridge
}

// dispatchMode selects and runs the appropriate runtime mode based on config and stdin.
func dispatchMode(server *Server, cfg *serverConfig) {
	isTTY, stdinMode := detectStdinMode()
	mcpConfigPath := findMCPConfig()
	mode := selectRuntimeMode(cfg, isTTY)
	launchInfo := classifyLaunchMode(cfg, isTTY)
	setCurrentLaunchMode(launchInfo)
	if mode == modeDaemon {
		diag.SetSink(os.Stderr)
	}

	server.logLifecycle("mode_detection", cfg.port, map[string]any{
		"is_tty":           isTTY,
		"stdin_mode":       fmt.Sprintf("%v", stdinMode),
		"has_mcp_config":   mcpConfigPath != "",
		"selected_runtime": mode,
	})
	server.logLifecycle("launch_mode_classified", cfg.port, map[string]any{
		"launch_mode":      launchInfo.Mode,
		"launch_reason":    launchInfo.Reason,
		"parent_process":   launchInfo.ParentProcess,
		"is_tty":           launchInfo.IsTTY,
		"strict_required":  launchInfo.StrictRequired,
		"under_supervisor": launchInfo.UnderSupervisor,
		"selected_runtime": mode,
	})

	if warning := buildLaunchModeWarning(launchInfo, cfg.port); warning != "" {
		server.AddWarning(warning)
		diag.Printf("[Kaboom] Kaboom appears to be running in non-persistent mode (%s).\n", launchInfo.Reason)
		diag.Printf("[Kaboom] This will disconnect the extension when the process exits.\n")
		diag.Printf("[Kaboom] Start persistently: kaboom-agentic-browser --daemon --port %d\n", cfg.port)
	}
	if err := enforcePersistentMode(launchInfo); err != nil {
		diag.Printf("[Kaboom] %v\n", err)
		os.Exit(1)
	}

	switch mode {
	case modeDaemon:
		server.logLifecycle("daemon_mode_start", cfg.port, nil)
		if err := runMCPMode(server, cfg.port, cfg.apiKey, daemonlife.LaunchOptions{Parallel: cfg.parallelMode}); err != nil {
			telemetry.AppError("daemon_start_failed", nil)
			diagPath := appendExitDiagnostic("daemon_start_failed", map[string]any{
				"port":  cfg.port,
				"error": err.Error(),
			})
			if diagPath != "" {
				diag.Printf("[Kaboom] Startup diagnostics written to: %s\n", diagPath)
			}
			diag.Printf("[Kaboom] Daemon error: %v\n", err)
			os.Exit(1)
		}
		return
	case modeBridge:
		if err := bridge.EnsureIOIsolation(cfg.logFile); err != nil {
			sendStartupError("Bridge stdio isolation failed: " + err.Error())
			os.Exit(1)
		}
		server.logLifecycle("bridge_mode_start", cfg.port, bridge.LaunchFingerprint())
		if cfg.bridgeMode {
			diag.Printf("[Kaboom] Starting in bridge mode (stdio -> HTTP)\n")
		} else if isTTY && mcpConfigPath != "" {
			diag.Printf("[Kaboom] MCP config detected at %s; running in bridge mode for tool compatibility.\n", mcpConfigPath)
		} else if isTTY {
			diag.Printf("[Kaboom] Running in bridge mode by default. Use --daemon for server-only mode.\n")
		}
		if os.Getenv("KABOOM_TEST_BRIDGE_NOISE") == "1" {
			// Test-only probe: verifies transport isolation prevents accidental
			// stdout/stderr writes from corrupting MCP responses.
			fmt.Fprintln(os.Stderr, "KABOOM_TEST_NOISE_STDOUT")
			fmt.Fprintln(os.Stderr, "KABOOM_TEST_NOISE_STDERR")
		}
		bridge.RunMode(cfg.port, cfg.logFile, cfg.maxEntries)
		return
	default:
		// Defensive fallback (should be unreachable).
		bridge.RunMode(cfg.port, cfg.logFile, cfg.maxEntries)
		return
	}
}
