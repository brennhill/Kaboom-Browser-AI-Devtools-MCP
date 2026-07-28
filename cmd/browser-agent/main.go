// Purpose: Program entry point — dispatches to MCP server, bridge, CLI, connect, stop, doctor, or install modes based on flags.
// Why: Provides a single binary with multiple operating modes selected at startup via command-line arguments.

package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/cli"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/exitdiag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/playbooks"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/pushapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/versioncheck"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/identity"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

// version is set at build time via -ldflags "-X main.version=..."
// Fallback used for `go run` and `make dev` (no ldflags).
var version = "0.8.8"

var exitDiagnostics = exitdiag.New(exitdiag.Options{Version: version})

type upgradeInfoProvider interface {
	UpgradeInfo() (pending bool, version string, detectedAt time.Time)
}

var binaryUpgradeState upgradeInfoProvider

var releaseChecker = versioncheck.New(versioncheck.Options{
	CurrentVersion: version,
	ReleaseURL:     os.Getenv("KABOOM_RELEASES_URL"),
	HTTPClient:     &http.Client{Timeout: 10 * time.Second},
})

// daemonProcessArgv0 binds the build-time version to the process-title builder in
// internal/procctl. It lives beside `version` because that is the only reason it
// exists — bridge.Deps and cli.Deps both take a plain func(string) string.
func daemonProcessArgv0(exePath string) string {
	return procctl.Argv0ForVersion(exePath, version)
}

func cliRuntimeConfig() cli.RuntimeConfig {
	return cli.RuntimeConfig{
		DefaultPort: defaultPort, MaxPostBodySize: maxPostBodySize,
		IsServerRunning: bridge.IsServerRunning,
		WaitForServer: func(port int, timeout time.Duration) bool {
			return bridge.WaitForServer(port, timeout)
		},
		DaemonProcessArgv0: daemonProcessArgv0,
	}
}

func init() {
	// Sync telemetry version from main for go run (no ldflags) fallback.
	if telemetry.Version == "dev" {
		telemetry.Version = version
	}
}

func initBridge() {
	debugLogger := diag.NewDebugFileFromEnv()
	bridge.Init(bridge.Deps{
		Version: version, MaxPostBodySize: maxPostBodySize,
		MCPServerName: identity.MCPServerName, LegacyMCPServerNames: identity.LegacyMCPServerNames,
		ServerInstructions: serverInstructions,
		Stderrf:            diag.Printf, Debugf: debugLogger.Printf,
		WriteMCPPayload: bridge.WriteMCPPayload, SyncStdoutBestEffort: bridge.SyncStdoutBestEffort,
		SetStderrSink:    diag.SetSink,
		GetBridgeFraming: bridge.PushRuntime.Framing, StoreBridgeFraming: bridge.PushRuntime.StoreFraming,
		SetPushClientCapabilities: func(capabilities push.ClientCapabilities) {
			bridge.PushRuntime.SetCapabilities(capabilities)
			if capabilities.ClientName != "" {
				telemetry.SetLLMName(capabilities.ClientName)
			}
		},
		ExtractClientCapabilities: pushapi.ExtractClientCapabilities,
		NegotiateProtocolVersion:  mcp.NegotiateProtocolVersion,
		MCPResources:              playbooks.Resources, MCPResourceTemplates: playbooks.ResourceTemplates,
		ResolveResourceContent: playbooks.ResolveResourceContent,
		DaemonProcessArgv0:     daemonProcessArgv0, StopServerForUpgrade: stopServerForUpgrade,
		FindProcessOnPort: procctl.FindProcessOnPort, IsProcessAlive: procctl.IsProcessAlive,
		AppendExitDiagnostic: exitDiagnostics.Append,
	})
}

// startTime tracks when the server started for uptime calculation
var startTime = time.Now()

const (
	defaultPort     = 7890
	maxPostBodySize = 10 * 1024 * 1024 // 10 MB

	// Server health check parameters
	healthCheckMaxAttempts   = 30                     // 30 attempts * 100ms = 3 seconds total
	healthCheckRetryInterval = 100 * time.Millisecond // Retry interval between health check attempts
)

var (
	// Upload automation security flags (set by CLI flags, consumed by ToolHandler)
	osUploadAutomationFlag bool                // --enable-os-upload-automation (Stage 4 only)
	uploadSecurityConfig   *uploadsec.Security // validated upload security config

	startupWarnings []string
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			exitDiagnostics.Recover(r)
		}
	}()

	if len(os.Args) >= 2 && cli.IsCLIMode(os.Args[1:]) {
		os.Exit(cli.Run(os.Args[1:], cliRuntimeConfig()))
	}

	cfg := parseAndValidateFlags()

	server, err := NewServer(cfg.logFile, cfg.maxEntries)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] Error creating server: %v\n", err)
		os.Exit(1)
	}
	for _, warning := range startupWarnings {
		server.AddWarning(warning)
	}

	initBridge()
	dispatchMode(server, cfg)
}

// #lizard forgives
func printHelp() {
	diag.Print(`
Kaboom - Agentic Browser Devtools - rapid e2e web development

Usage: kaboom [options]

Options:
  --port <number>        Port to listen on (default: 7890)
  --log-file <path>      Path to log file (default: in runtime state dir)
  --state-dir <path>     Directory for runtime state (default: OS app state dir)
  --parallel             Opt-in parallel mode (isolated state dir, no takeover)
  --max-entries <number> Max log entries before rotation (default: 1000)
  --stop                 Stop the running server on the specified port
  --force                Force kill ALL running kaboom daemons (used during install)
  --api-key <key>        Require API key for HTTP requests (optional)
  --connect              Connect to existing server (multi-client mode)
  --client-id <id>       Override client ID (default: derived from CWD)
  --check                Verify setup (check port availability, print status)
  --doctor               Run full diagnostics (alias of --check)
  --fastpath-min-samples Minimum telemetry samples required for threshold check (default: 50)
  --fastpath-max-failure-ratio Maximum allowed fast-path failure ratio for --check (disabled by default)
  --version              Show version
  --help                 Show this help message

Kaboom always runs in MCP mode: the HTTP server starts in the background
(for the browser extension) and MCP protocol runs over stdio (for Claude Code, Cursor, etc.).
The server persists until explicitly stopped with --stop or killed.

Examples:
  kaboom                              # Start server (daemon mode)
  kaboom --stop                       # Stop server on default port
  kaboom --stop --port 8080           # Stop server on specific port
  kaboom --force                      # Force kill all daemons (for clean upgrade)
  kaboom --api-key s3cret             # Start with API key auth
  kaboom --connect --port 7890        # Connect to existing server
  kaboom --check                      # Verify setup before running
  kaboom --port 8080 --max-entries 500

CLI Mode (direct tool access):
  kaboom observe errors --limit 50
  kaboom analyze dom --selector "button"
  kaboom observe logs --min-level warn
  kaboom generate har --save-to out.har
  kaboom configure health
  kaboom interact click --selector "#btn"

  CLI flags: --port, --format (human|json|csv), --timeout (ms)
  Env vars: KABOOM_PORT, KABOOM_FORMAT, KABOOM_STATE_DIR

MCP Configuration:
  kaboom-agentic-browser --install     Auto-install to all detected AI clients
  kaboom-agentic-browser --config      Show configuration and detected clients
  kaboom-agentic-browser --doctor      Run diagnostics on installed configs

  Supported clients: Claude Code, Claude Desktop, Cursor, Windsurf, VS Code
`)
}
