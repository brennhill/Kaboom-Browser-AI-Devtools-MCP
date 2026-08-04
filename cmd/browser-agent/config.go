// Purpose: Defines serverConfig struct, CLI flag parsing, runtime mode constants, and startup orchestration.
// Why: Centralizes all command-line configuration and mode selection logic for the daemon entry point.

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/connectmode"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/daemonlife"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/launchmode"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/nativeinstall"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/configdiscovery"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session/clientreg"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

// multiFlag implements flag.Value for repeatable string flags (e.g., --upload-deny-pattern).
type multiFlag []string

func runSetupCheckWithOptions(port int, options setupCheckOptions) bool {
	// Doctor must inspect a stable snapshot. Fast-path writes are intentionally
	// asynchronous, so drain accepted records before reading their log.
	bridge.FlushFastPathTelemetry()
	return health.RunSetupCheckWithOptions(port, health.SetupCheckOptions{
		MinSamples: options.minSamples, MaxFailureRatio: options.maxFailureRatio,
	}, health.SetupDeps{
		Version: version, PortKillHint: procctl.PortKillHint,
		FastPathTelemetryLogPath: bridge.FastPathTelemetryLogPath,
	})
}

func (f *multiFlag) String() string { return strings.Join(*f, ", ") }
func (f *multiFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
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

type runtimeMode string

const (
	modeBridge runtimeMode = "bridge"
	modeDaemon runtimeMode = "daemon"
)

// parsedFlags holds the raw parsed flag values before validation.
type parsedFlags struct {
	port, maxEntries                                         *int
	fastPathMinSamples                                       *int
	logFile, apiKey, clientID, stateDir, uploadDir           *string
	fastPathMaxFailureRatio                                  *float64
	showVersion, showHelp, doctorMode, stopMode, connectMode *bool
	bridgeMode, daemonMode, enableOsUploadAutomation         *bool
	parallelMode                                             *bool
	forceCleanup                                             *bool
	installMode                                              *bool
	uploadDenyPatterns                                       multiFlag
	ssrfAllowedHosts                                         multiFlag
}

// registerFlags defines all CLI flags and returns the parsed values.
func registerFlags() *parsedFlags {
	f := &parsedFlags{}
	f.port = flag.Int("port", defaultPort, "Port to listen on")
	f.logFile = flag.String("log-file", "", "Path to log file (default: in runtime state dir)")
	f.maxEntries = flag.Int("max-entries", defaultMaxEntries, "Max log entries before rotation")
	f.fastPathMinSamples = flag.Int("fastpath-min-samples", 50, "Minimum fast-path telemetry samples required when threshold check is enabled")
	f.fastPathMaxFailureRatio = flag.Float64("fastpath-max-failure-ratio", -1, "Maximum allowed fast-path failure ratio in --doctor (set >=0 to enforce)")
	f.showVersion = flag.Bool("version", false, "Show version")
	f.showHelp = flag.Bool("help", false, "Show help")
	f.apiKey = flag.String("api-key", os.Getenv("KABOOM_API_KEY"), "API key for HTTP authentication (optional, or KABOOM_API_KEY env)")
	f.doctorMode = flag.Bool("doctor", false, "Run setup diagnostics")
	f.stopMode = flag.Bool("stop", false, "Stop the running server on the specified port")
	f.connectMode = flag.Bool("connect", false, "Connect to existing server (multi-client mode)")
	f.clientID = flag.String("client-id", "", "Override client ID (default: derived from CWD)")
	f.bridgeMode = flag.Bool("bridge", false, "Run as stdio-to-HTTP bridge (spawns daemon if needed)")
	f.daemonMode = flag.Bool("daemon", false, "Run as background server daemon (internal use)")
	f.parallelMode = flag.Bool("parallel", false, "Enable isolated parallel daemon mode (skip takeover; requires unique port/state-dir)")
	f.stateDir = flag.String("state-dir", "", "Directory for runtime state (default: OS app state directory)")
	f.enableOsUploadAutomation = flag.Bool("enable-os-upload-automation", false, "Enable OS-level file upload automation (Stage 4: AppleScript/xdotool)")
	f.uploadDir = flag.String("upload-dir", "", "Directory from which file uploads are allowed (required for Stages 2-4)")
	f.forceCleanup = flag.Bool("force", false, "Force kill all running kaboom daemons (used during install to ensure clean upgrade)")
	f.installMode = flag.Bool("install", false, "Auto-install Kaboom to all detected MCP clients")
	flag.Var(&f.uploadDenyPatterns, "upload-deny-pattern", "Additional sensitive path patterns to block (repeatable)")
	flag.Var(&f.ssrfAllowedHosts, "ssrf-allow-host", "Host:port to allow for form submit SSRF (repeatable, test use)")
	flag.Parse()
	return f
}

type setupCheckOptions struct {
	minSamples      int
	maxFailureRatio float64
}

// parseAndValidateFlags parses CLI flags, validates them, and handles early-exit modes.
func parseAndValidateFlags() *serverConfig {
	f := registerFlags()

	uploadsec.SetSSRFAllowedHosts(f.ssrfAllowedHosts)
	uploadSecurity := initUploadSecurity(*f.enableOsUploadAutomation, *f.uploadDir, f.uploadDenyPatterns)
	validatePort(*f.port)
	normalizeStateDir(f.stateDir)
	var warnings []string
	if err := applyParallelModeStateDir(*f.parallelMode, f.stateDir, &warnings); err != nil {
		diag.Printf("[Kaboom] Invalid --parallel setup: %v\n", err)
		os.Exit(1)
	}
	handleEarlyExitModes(f)
	resolveDefaultLogFile(f.logFile, &warnings)

	return &serverConfig{
		port:             *f.port,
		logFile:          *f.logFile,
		maxEntries:       *f.maxEntries,
		apiKey:           *f.apiKey,
		stateDir:         *f.stateDir,
		clientID:         *f.clientID,
		bridgeMode:       *f.bridgeMode,
		daemonMode:       *f.daemonMode,
		parallelMode:     *f.parallelMode,
		uploadAutomation: *f.enableOsUploadAutomation,
		uploadSecurity:   uploadSecurity,
		startupWarnings:  warnings,
	}
}

func initUploadSecurity(enabled bool, dir string, denyPatterns multiFlag) *uploadsec.Security {
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

func applyParallelModeStateDir(parallel bool, stateDir *string, warnings *[]string) error {
	if !parallel || strings.TrimSpace(*stateDir) != "" {
		return nil
	}
	root, err := state.RootDir()
	if err != nil {
		return fmt.Errorf("cannot resolve runtime state root: %w", err)
	}
	generated := filepath.Join(root, "parallel", fmt.Sprintf("run-%d-%d", time.Now().UnixNano(), os.Getpid()))
	if err := os.MkdirAll(generated, 0o750); err != nil {
		return fmt.Errorf("cannot create parallel state dir %q: %w", generated, err)
	}
	*stateDir = filepath.Clean(generated)
	if err := os.Setenv(state.StateDirEnv, *stateDir); err != nil {
		return fmt.Errorf("failed to set %s: %w", state.StateDirEnv, err)
	}
	*warnings = append(*warnings, fmt.Sprintf("parallel_mode_state_dir_auto: %s", *stateDir))
	return nil
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

func handleEarlyExitModes(flags *parsedFlags) {
	if *flags.showVersion {
		diag.Printf("kaboom v%s\n", version)
		os.Exit(0)
	}
	if *flags.showHelp {
		printHelp()
		os.Exit(0)
	}
	if *flags.forceCleanup {
		procctl.ForceCleanup()
		os.Exit(0)
	}
	if *flags.doctorMode {
		ok := runSetupCheckWithOptions(*flags.port, setupCheckOptions{
			minSamples:      *flags.fastPathMinSamples,
			maxFailureRatio: *flags.fastPathMaxFailureRatio,
		})
		if !ok {
			os.Exit(1)
		}
		os.Exit(0)
	}
	if *flags.stopMode {
		procctl.Stop(*flags.port, bridgeRunner.IsServerRunning)
		os.Exit(0)
	}
	if *flags.installMode {
		if err := nativeinstall.Run(procctl.ForceCleanupQuietly, flag.Args()...); err != nil {
			fmt.Fprintf(os.Stderr, "install_failed: %v\n", err)
			os.Exit(2)
		}
		os.Exit(0)
	}
	if *flags.connectMode {
		cwd, _ := os.Getwd()
		id := *flags.clientID
		if id == "" {
			id = clientreg.DeriveClientID(cwd)
		}
		connectmode.New(connectmode.Deps{
			Input: os.Stdin, HTTPClient: http.DefaultClient,
			Diagnosticf: diag.Printf, WriteMCP: bridge.WriteMCPPayload, Exit: os.Exit,
		}).Run(*flags.port, id, cwd)
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

func selectRuntimeMode(config *serverConfig, _ bool) runtimeMode {
	if config.bridgeMode {
		return modeBridge
	}
	if config.daemonMode {
		return modeDaemon
	}
	return modeBridge
}

func dispatchMode(server *Server, config *serverConfig) {
	isTTY, stdinMode := detectStdinMode()
	mcpConfigPath := configdiscovery.Find()
	mode := selectRuntimeMode(config, isTTY)
	launchInfo := launchmode.Classify(config.daemonMode, isTTY, launchmode.DetectParentProcessName())
	launchmode.SetCurrent(launchInfo)
	if mode == modeDaemon {
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
		server.AddWarning(warning)
		diag.Printf("[Kaboom] Kaboom appears to be running in non-persistent mode (%s).\n", launchInfo.Reason)
		diag.Println("[Kaboom] This will disconnect the extension when the process exits.")
		diag.Printf("[Kaboom] Start persistently: kaboom-agentic-browser --daemon --port %d\n", config.port)
	}
	if err := launchmode.EnforcePersistent(launchInfo, defaultPort); err != nil {
		diag.Printf("[Kaboom] %v\n", err)
		os.Exit(1)
	}

	switch mode {
	case modeDaemon:
		server.logLifecycle("daemon_mode_start", config.port, nil)
		if err := runMCPMode(server, config.port, config.apiKey, daemonlife.LaunchOptions{Parallel: config.parallelMode}); err != nil {
			telemetry.AppError(incident.CodeDaemonStartFailed)
			diagnosticPath := exitDiagnostics.Append("daemon_start_failed", map[string]any{
				"port":  config.port,
				"error": err.Error(),
			})
			if diagnosticPath != "" {
				diag.Printf("[Kaboom] Startup diagnostics written to: %s\n", diagnosticPath)
			}
			diag.Printf("[Kaboom] Daemon error: %v\n", err)
			os.Exit(1)
		}
	case modeBridge:
		if err := bridgeRunner.EnsureIOIsolation(config.logFile); err != nil {
			bridge.SendStartupError("Bridge stdio isolation failed: " + err.Error())
			os.Exit(1)
		}
		server.logLifecycle("bridge_mode_start", config.port, bridgeRunner.LaunchFingerprint())
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
		bridgeRunner.RunMode(config.port, config.logFile, config.maxEntries)
	default:
		bridgeRunner.RunMode(config.port, config.logFile, config.maxEntries)
	}
}
