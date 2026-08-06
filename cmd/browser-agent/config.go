// Purpose: Defines serverConfig struct, CLI flag parsing, runtime mode constants, and startup orchestration.
// Why: Centralizes all command-line configuration and mode selection logic for the daemon entry point.

package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge/fastpathtelemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge/fingerprint"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/connectmode"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/daemonlife"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/launchmode"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/nativeinstall"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/runtimeconfig"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/runtimeflags"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/configdiscovery"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/serverdefaults"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session/clientreg"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
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

// serverConfig holds the parsed command-line flags for the server.
type serverConfig struct {
	port             int
	logFile          string
	maxEntries       int
	apiKey           string
	stateDir         string
	clientID         string
	bridgeMode       bool
	daemonMode       bool
	parallelMode     bool
	uploadAutomation bool
	uploadSecurity   *uploadsec.Security
	startupWarnings  []string
}

type setupCheckOptions struct {
	minSamples      int
	maxFailureRatio float64
}

// parseAndValidateFlags parses CLI flags, validates them, and handles early-exit modes.
func parseAndValidateFlags() *serverConfig {
	f, err := runtimeflags.Parse(os.Args[1:], os.Getenv("KABOOM_API_KEY"))
	if err != nil {
		diag.Printf("[Kaboom] Invalid command-line options: %v\n", err)
		os.Exit(2)
	}

	uploadsec.SetSSRFAllowedHosts(f.SSRFAllowedHosts)
	uploadSecurity := initUploadSecurity(f.EnableOSUpload, f.UploadDir, f.UploadDenyPatterns)
	validatePort(f.Port)
	normalizeStateDir(&f.StateDir)
	var warnings []string
	resolvedStateDir, parallelWarnings, err := runtimeconfig.ApplyParallelStateDir(
		f.ParallelMode,
		f.StateDir,
		time.Now(),
		os.Getpid(),
	)
	if err != nil {
		diag.Printf("[Kaboom] Invalid --parallel setup: %v\n", err)
		os.Exit(1)
	}
	f.StateDir = resolvedStateDir
	warnings = append(warnings, parallelWarnings...)
	handleEarlyExitModes(&f)
	resolveDefaultLogFile(&f.LogFile, &warnings)

	return &serverConfig{
		port:             f.Port,
		logFile:          f.LogFile,
		maxEntries:       f.MaxEntries,
		apiKey:           f.APIKey,
		stateDir:         f.StateDir,
		clientID:         f.ClientID,
		bridgeMode:       f.BridgeMode,
		daemonMode:       f.DaemonMode,
		parallelMode:     f.ParallelMode,
		uploadAutomation: f.EnableOSUpload,
		uploadSecurity:   uploadSecurity,
		startupWarnings:  warnings,
	}
}

func initUploadSecurity(enabled bool, dir string, denyPatterns []string) *uploadsec.Security {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			if enabled {
				diag.Printf("[Kaboom] Cannot determine home directory for default upload dir: %v\n", err)
				os.Exit(1)
			}
			return &uploadsec.Security{}
		}
		dir = filepath.Join(home, "kaboom-upload-dir")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			if enabled {
				diag.Printf("[Kaboom] Cannot create default upload dir %s: %v\n", dir, err)
				os.Exit(1)
			}
			return &uploadsec.Security{}
		}
	}
	security, err := uploadsec.ValidateUploadDir(dir, denyPatterns)
	if err != nil {
		diag.Printf("[Kaboom] Upload security validation failed: %v\n", err)
		os.Exit(1)
	}
	return security
}

func validatePort(port int) {
	if port < 1 || port > 65535 {
		diag.Printf("[Kaboom] Invalid port: %d (must be 1-65535)\n", port)
		os.Exit(1)
	}
}

func normalizeStateDir(stateDir *string) {
	if *stateDir == "" {
		return
	}
	absolute, err := filepath.Abs(*stateDir)
	if err != nil {
		diag.Printf("[Kaboom] Invalid --state-dir: %v\n", err)
		os.Exit(1)
	}
	*stateDir = filepath.Clean(absolute)
	if err := os.Setenv(state.StateDirEnv, *stateDir); err != nil {
		diag.Printf("[Kaboom] Failed to set %s: %v\n", state.StateDirEnv, err)
		os.Exit(1)
	}
}

func resolveDefaultLogFile(logFile *string, warnings *[]string) {
	if *logFile != "" {
		return
	}
	defaultLogFile, err := state.DefaultLogFile()
	if err != nil {
		fallback := filepath.Join(os.TempDir(), "kaboom", "logs", "kaboom.jsonl")
		*warnings = append(*warnings, fmt.Sprintf("state_dir_unwritable: %v; falling back to %s", err, fallback))
		*logFile = fallback
		return
	}
	*logFile = defaultLogFile
}

func handleEarlyExitModes(flags *runtimeflags.Values) {
	if flags.ShowVersion {
		diag.Printf("kaboom v%s\n", version)
		os.Exit(0)
	}
	if flags.ShowHelp {
		printHelp()
		os.Exit(0)
	}
	if flags.ForceCleanup {
		procctl.ForceCleanup()
		os.Exit(0)
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

func dispatchMode(server *Server, config *serverConfig) {
	isTTY, stdinMode := detectStdinMode()
	mcpConfigPath := configdiscovery.Find()
	mode := launchmode.SelectRuntimeMode(config.bridgeMode, config.daemonMode)
	launchInfo := launchmode.Classify(config.daemonMode, isTTY, launchmode.DetectParentProcessName())
	launchmode.SetCurrent(launchInfo)
	if mode == launchmode.RuntimeDaemon {
		diag.SetSink(os.Stderr)
	}

	server.logLifecycle("mode_detection", config.port, map[string]any{
		"is_tty":           isTTY,
		"stdin_mode":       fmt.Sprintf("%v", stdinMode),
		"has_mcp_config":   mcpConfigPath != "",
		"selected_runtime": mode,
	})
	server.logLifecycle("launch_mode_classified", config.port, map[string]any{
		"launch_mode":      launchInfo.Mode,
		"launch_reason":    launchInfo.Reason,
		"parent_process":   launchInfo.ParentProcess,
		"is_tty":           launchInfo.IsTTY,
		"strict_required":  launchInfo.StrictRequired,
		"under_supervisor": launchInfo.UnderSupervisor,
		"selected_runtime": mode,
	})

	if warning := launchmode.Warning(launchInfo, config.port); warning != "" {
		server.warnings.Add(warning)
		diag.Printf("[Kaboom] Kaboom appears to be running in non-persistent mode (%s).\n", launchInfo.Reason)
		diag.Println("[Kaboom] This will disconnect the extension when the process exits.")
		diag.Printf("[Kaboom] Start persistently: kaboom-agentic-browser --daemon --port %d\n", config.port)
	}
	if err := launchmode.EnforcePersistent(launchInfo, serverdefaults.Port); err != nil {
		diag.Printf("[Kaboom] %v\n", err)
		os.Exit(1)
	}

	switch mode {
	case launchmode.RuntimeDaemon:
		server.logLifecycle("daemon_mode_start", config.port, nil)
		if err := runMCPMode(server, config.port, config.apiKey, daemonlife.LaunchOptions{Parallel: config.parallelMode}); err != nil {
			telemetry.AppError(incident.CodeDaemonStartFailed)
			diagnosticPath := server.runtime.ExitDiagnostics().Append("daemon_start_failed", map[string]any{
				"port":  config.port,
				"error": err.Error(),
			})
			if diagnosticPath != "" {
				diag.Printf("[Kaboom] Startup diagnostics written to: %s\n", diagnosticPath)
			}
			diag.Printf("[Kaboom] Daemon error: %v\n", err)
			os.Exit(1)
		}
	case launchmode.RuntimeBridge:
		if err := server.runtime.BridgeRunner().EnsureIOIsolation(config.logFile); err != nil {
			bridge.SendStartupError("Bridge stdio isolation failed: " + err.Error())
			os.Exit(1)
		}
		server.logLifecycle("bridge_mode_start", config.port, fingerprint.Capture(version, os.Executable))
		if config.bridgeMode {
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
		server.runtime.BridgeRunner().RunMode(config.port, config.logFile, config.maxEntries)
	default:
		server.runtime.BridgeRunner().RunMode(config.port, config.logFile, config.maxEntries)
	}
}
