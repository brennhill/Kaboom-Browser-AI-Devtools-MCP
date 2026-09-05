// config.go — Composes process exits and runtime-mode launch callbacks.

package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge/fastpathtelemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge/fingerprint"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/censuscmd"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/connectmode"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/daemonlife"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/launchmode"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/nativeinstall"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/runtimeflags"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/startupconfig"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/configdiscovery"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/serverdefaults"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session/clientreg"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

func runSetupCheckWithOptions(port int, options setupCheckOptions) bool {
	// Doctor must inspect a stable snapshot. Fast-path writes are intentionally
	// asynchronous, so drain accepted records before reading their log.
	fastpathtelemetry.Flush()
	return health.RunSetupCheckWithOptions(port, health.SetupCheckOptions{
		MinSamples: options.minSamples, MaxFailureRatio: options.maxFailureRatio,
	}, health.SetupDeps{
		Version: version, PortKillHint: procctl.PortKillHint,
		FastPathTelemetryLogPath: fastpathtelemetry.MethodLogPath,
	})
}

type setupCheckOptions struct {
	minSamples      int
	maxFailureRatio float64
}

// parseAndValidateFlags parses CLI flags, validates them, and handles early-exit modes.
func parseAndValidateFlags() *startupconfig.Runtime {
	f, err := runtimeflags.Parse(os.Args[1:], os.Getenv("KABOOM_API_KEY"))
	if err != nil {
		diag.Printf("[Kaboom] Invalid command-line options: %v\n", err)
		os.Exit(2)
	}
	if err := startupconfig.ValidatePort(f.Port); err != nil {
		diag.Printf("[Kaboom] %v\n", err)
		os.Exit(1)
	}
	if err := activateEarlyModeStateDir(f); err != nil {
		diag.Printf("[Kaboom] Invalid state directory: %v\n", err)
		os.Exit(1)
	}

	handleEarlyExitModes(&f)
	uploadsec.SetSSRFAllowedHosts(f.SSRFAllowedHosts)
	config, err := startupconfig.BuildRuntime(f, time.Now(), os.Getpid())
	if err != nil {
		diag.Printf("[Kaboom] Invalid startup configuration: %v\n", err)
		os.Exit(1)
	}
	return &config
}

func activateEarlyModeStateDir(flags runtimeflags.Values) error {
	if flags.StateDir == "" || (!flags.ForceCleanup && !flags.DoctorMode && !flags.StopMode &&
		!flags.InstallMode && !flags.ConnectMode) {
		return nil
	}
	_, err := startupconfig.NormalizeStateDir(flags.StateDir)
	return err
}

func handleEarlyExitModes(flags *runtimeflags.Values) {
	if flags.ShowVersion {
		diag.Printf("kaboom v%s\n", version)
		os.Exit(0)
	}
	if flags.ShowHelp {
		diag.Print(startupconfig.HelpText)
		os.Exit(0)
	}
	if flags.ForceCleanup {
		procctl.ForceCleanup()
		os.Exit(0)
	}
	if flags.CensusMode {
		os.Exit(censuscmd.Census())
	}
	if flags.ReapMode {
		os.Exit(censuscmd.Reap(flags.ReapDryRun))
	}
	if flags.DoctorMode {
		ok := runSetupCheckWithOptions(flags.Port, setupCheckOptions{
			minSamples:      flags.FastPathMinSamples,
			maxFailureRatio: flags.FastPathMaxFailureRatio,
		})
		if !ok {
			os.Exit(1)
		}
		os.Exit(0)
	}
	if flags.StopMode {
		procctl.Stop(flags.Port, bridgeRuntime().IsServerRunning)
		os.Exit(0)
	}
	if flags.InstallMode {
		if err := nativeinstall.Run(procctl.ForceCleanupQuietly, flags.Arguments...); err != nil {
			fmt.Fprintf(os.Stderr, "install_failed: %v\n", err)
			os.Exit(2)
		}
		os.Exit(0)
	}
	if flags.ConnectMode {
		cwd, _ := os.Getwd()
		id := flags.ClientID
		if id == "" {
			id = clientreg.DeriveClientID(cwd)
		}
		connectmode.New(connectmode.Deps{
			Input: os.Stdin, HTTPClient: http.DefaultClient,
			Diagnosticf: diag.Printf, WriteMCP: bridge.WriteMCPPayload, Exit: os.Exit,
		}).Run(flags.Port, id, cwd)
		os.Exit(0)
	}
}

func detectStdinMode() (isTTY bool, stdinMode os.FileMode) {
	stat, err := os.Stdin.Stat()
	if err == nil {
		isTTY = (stat.Mode() & os.ModeCharDevice) != 0
		stdinMode = stat.Mode()
	}
	return isTTY, stdinMode
}

func dispatchMode(server *Server, config *startupconfig.Runtime) {
	isTTY, stdinMode := detectStdinMode()
	mcpConfigPath := configdiscovery.Find()
	mode := launchmode.SelectRuntimeMode(config.BridgeMode, config.DaemonMode)
	launchInfo := launchmode.Classify(config.DaemonMode, isTTY, launchmode.DetectParentProcessName())
	launchmode.SetCurrent(launchInfo)
	if mode == launchmode.RuntimeDaemon {
		diag.SetSink(os.Stderr)
	}

	server.logLifecycle("mode_detection", config.Port, map[string]any{
		"is_tty":           isTTY,
		"stdin_mode":       fmt.Sprintf("%v", stdinMode),
		"has_mcp_config":   mcpConfigPath != "",
		"selected_runtime": mode,
	})
	server.logLifecycle("launch_mode_classified", config.Port, map[string]any{
		"launch_mode":      launchInfo.Mode,
		"launch_reason":    launchInfo.Reason,
		"parent_process":   launchInfo.ParentProcess,
		"is_tty":           launchInfo.IsTTY,
		"strict_required":  launchInfo.StrictRequired,
		"under_supervisor": launchInfo.UnderSupervisor,
		"selected_runtime": mode,
	})

	if warning := launchmode.Warning(launchInfo, config.Port); warning != "" {
		server.warnings.Add(warning)
		diag.Printf("[Kaboom] Kaboom appears to be running in non-persistent mode (%s).\n", launchInfo.Reason)
		diag.Println("[Kaboom] This will disconnect the extension when the process exits.")
		diag.Printf("[Kaboom] Start persistently: kaboom-agentic-browser --daemon --port %d\n", config.Port)
	}
	if err := launchmode.EnforcePersistent(launchInfo, serverdefaults.Port); err != nil {
		diag.Printf("[Kaboom] %v\n", err)
		os.Exit(1)
	}

	switch mode {
	case launchmode.RuntimeDaemon:
		server.logLifecycle("daemon_mode_start", config.Port, nil)
		if err := runMCPMode(server, config.Port, config.APIKey, daemonlife.LaunchOptions{Parallel: config.ParallelMode}); err != nil {
			telemetry.AppError(incident.CodeDaemonStartFailed)
			diagnosticPath := server.runtime.ExitDiagnostics().Append("daemon_start_failed", map[string]any{
				"port":  config.Port,
				"error": err.Error(),
			})
			if diagnosticPath != "" {
				diag.Printf("[Kaboom] Startup diagnostics written to: %s\n", diagnosticPath)
			}
			diag.Printf("[Kaboom] Daemon error: %v\n", err)
			os.Exit(1)
		}
	case launchmode.RuntimeBridge:
		if err := server.runtime.BridgeRunner().EnsureIOIsolation(config.LogFile); err != nil {
			bridge.SendStartupError("Bridge stdio isolation failed: " + err.Error())
			os.Exit(1)
		}
		server.logLifecycle("bridge_mode_start", config.Port, fingerprint.Capture(version, os.Executable))
		if config.BridgeMode {
			diag.Println("[Kaboom] Starting in bridge mode (stdio -> HTTP)")
		} else if isTTY && mcpConfigPath != "" {
			diag.Printf("[Kaboom] MCP config detected at %s; running in bridge mode for tool compatibility.\n", mcpConfigPath)
		} else if isTTY {
			diag.Println("[Kaboom] Running in bridge mode by default. Use --daemon for server-only mode.")
		}
		if os.Getenv("KABOOM_TEST_BRIDGE_NOISE") == "1" {
			fmt.Fprintln(os.Stderr, "KABOOM_TEST_NOISE_STDOUT")
			fmt.Fprintln(os.Stderr, "KABOOM_TEST_NOISE_STDERR")
		}
		server.runtime.BridgeRunner().RunMode(config.Port, config.LogFile, config.MaxEntries)
	default:
		server.runtime.BridgeRunner().RunMode(config.Port, config.LogFile, config.MaxEntries)
	}
}
