// Purpose: Defines serverConfig struct, CLI flag parsing, runtime mode constants, and startup orchestration.
// Why: Centralizes all command-line configuration and mode selection logic for the daemon entry point.

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

// multiFlag implements flag.Value for repeatable string flags (e.g., --upload-deny-pattern).
type multiFlag []string

func (f *multiFlag) String() string { return strings.Join(*f, ", ") }
func (f *multiFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

// serverConfig holds the parsed command-line flags for the server.
type serverConfig struct {
	port         int
	logFile      string
	maxEntries   int
	apiKey       string
	stateDir     string
	clientID     string
	bridgeMode   bool
	daemonMode   bool
	parallelMode bool
}

type runtimeMode string

const (
	modeBridge runtimeMode = "bridge"
	modeDaemon runtimeMode = "daemon"
)

// parsedFlags holds the raw parsed flag values before validation.
type parsedFlags struct {
	port, maxEntries                                                     *int
	fastPathMinSamples                                                   *int
	logFile, apiKey, clientID, stateDir, uploadDir                       *string
	fastPathMaxFailureRatio                                              *float64
	showVersion, showHelp, checkSetup, doctorMode, stopMode, connectMode *bool
	bridgeMode, daemonMode, enableOsUploadAutomation                     *bool
	parallelMode                                                         *bool
	forceCleanup                                                         *bool
	installMode                                                          *bool
	uploadDenyPatterns                                                   multiFlag
	ssrfAllowedHosts                                                     multiFlag
}

// registerFlags defines all CLI flags and returns the parsed values.
func registerFlags() *parsedFlags {
	f := &parsedFlags{}
	f.port = flag.Int("port", defaultPort, "Port to listen on")
	f.logFile = flag.String("log-file", "", "Path to log file (default: in runtime state dir)")
	f.maxEntries = flag.Int("max-entries", defaultMaxEntries, "Max log entries before rotation")
	f.fastPathMinSamples = flag.Int("fastpath-min-samples", 50, "Minimum fast-path telemetry samples required when threshold check is enabled")
	f.fastPathMaxFailureRatio = flag.Float64("fastpath-max-failure-ratio", -1, "Maximum allowed fast-path failure ratio in --check (set >=0 to enforce)")
	f.showVersion = flag.Bool("version", false, "Show version")
	f.showHelp = flag.Bool("help", false, "Show help")
	f.apiKey = flag.String("api-key", os.Getenv("KABOOM_API_KEY"), "API key for HTTP authentication (optional, or KABOOM_API_KEY env)")
	f.checkSetup = flag.Bool("check", false, "Verify setup: check if port is available and print status")
	f.doctorMode = flag.Bool("doctor", false, "Run full diagnostics (alias of --check)")
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
	flag.Bool("mcp", false, "Run in MCP mode (default, kept for backwards compatibility)")
	flag.Bool("persist", true, "Deprecated no-op (server persistence is default, kept for backwards compatibility)")
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

	osUploadAutomationFlag = *f.enableOsUploadAutomation
	uploadsec.SetSSRFAllowedHosts(f.ssrfAllowedHosts)
	initUploadSecurity(*f.enableOsUploadAutomation, *f.uploadDir, f.uploadDenyPatterns)
	validatePort(*f.port)
	normalizeStateDir(f.stateDir)
	if err := applyParallelModeStateDir(*f.parallelMode, f.stateDir); err != nil {
		diag.Printf("[Kaboom] Invalid --parallel setup: %v\n", err)
		os.Exit(1)
	}
	handleEarlyExitModes(f)
	resolveDefaultLogFile(f.logFile)

	return &serverConfig{
		port:         *f.port,
		logFile:      *f.logFile,
		maxEntries:   *f.maxEntries,
		apiKey:       *f.apiKey,
		stateDir:     *f.stateDir,
		clientID:     *f.clientID,
		bridgeMode:   *f.bridgeMode,
		daemonMode:   *f.daemonMode,
		parallelMode: *f.parallelMode,
	}
}

func initUploadSecurity(enabled bool, dir string, denyPatterns multiFlag) {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			if enabled {
				diag.Printf("[Kaboom] Cannot determine home directory for default upload dir: %v\n", err)
				os.Exit(1)
			}
			uploadSecurityConfig = &uploadsec.Security{}
			return
		}
		dir = filepath.Join(home, "kaboom-upload-dir")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			if enabled {
				diag.Printf("[Kaboom] Cannot create default upload dir %s: %v\n", dir, err)
				os.Exit(1)
			}
			uploadSecurityConfig = &uploadsec.Security{}
			return
		}
	}
	security, err := uploadsec.ValidateUploadDir(dir, denyPatterns)
	if err != nil {
		diag.Printf("[Kaboom] Upload security validation failed: %v\n", err)
		os.Exit(1)
	}
	uploadSecurityConfig = security
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

func applyParallelModeStateDir(parallel bool, stateDir *string) error {
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
	startupWarnings = append(startupWarnings, fmt.Sprintf("parallel_mode_state_dir_auto: %s", *stateDir))
	return nil
}

func resolveDefaultLogFile(logFile *string) {
	if *logFile != "" {
		return
	}
	defaultLogFile, err := state.DefaultLogFile()
	if err != nil {
		fallback := filepath.Join(os.TempDir(), "kaboom", "logs", "kaboom.jsonl")
		startupWarnings = append(startupWarnings, fmt.Sprintf("state_dir_unwritable: %v; falling back to %s", err, fallback))
		*logFile = fallback
		return
	}
	*logFile = defaultLogFile
}
