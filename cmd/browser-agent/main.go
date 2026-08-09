// Purpose: Program entry point — dispatches to MCP server, bridge, CLI, connect, stop, doctor, or install modes based on flags.
// Why: Provides a single binary with multiple operating modes selected at startup via command-line arguments.

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/appruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/cli"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/daemonrecovery"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/exitdiag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcpprotocol"
	playbookresources "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/playbooks/resources"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/pushapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/testowner"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/identity"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/serverdefaults"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

// version is set at build time via -ldflags "-X main.version=..."
// Fallback used for `go run` and `make dev` (no ldflags).
var version = "0.9.0"

// daemonProcessArgv0 binds the build-time version to the process-title builder in
// internal/procctl. It lives beside `version` because that is the only reason it
// exists — the bridge lifecycle collaborator and CLI config both take a plain
// func(string) string.
func daemonProcessArgv0(exePath string) string {
	return procctl.Argv0ForVersion(exePath, version)
}

func cliRuntimeConfig(runtime *appruntime.Runtime) cli.RuntimeConfig {
	return cli.RuntimeConfig{
		DefaultPort: serverdefaults.Port, MaxPostBodySize: maxPostBodySize,
		DiagnosticOutput: diag.Sink(),
		IsServerRunning:  runtime.BridgeRunner().IsServerRunning,
		WaitForServer: func(port int, timeout time.Duration) bool {
			return runtime.BridgeRunner().WaitForServer(port, timeout)
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

func buildBridgeRunner(exitDiagnostics *exitdiag.Recorder) *bridge.Runner {
	debugLogger := diag.NewDebugFileFromEnv()
	return bridge.NewRunner(
		bridge.Identity{Version: version, ServerName: identity.MCPServerName, ServerInstructions: mcpprotocol.Instructions},
		bridge.Transport{
			MaxBodySize: maxPostBodySize, Stderrf: diag.Printf, Debugf: debugLogger.Printf,
			Write: bridge.WriteMCPPayload, Sync: bridge.SyncStdoutBestEffort, SetStderr: diag.SetSink,
		},
		bridge.Protocol{
			GetFraming: bridge.PushRuntime.Framing, StoreFraming: bridge.PushRuntime.StoreFraming,
			SetCapabilities: func(capabilities push.ClientCapabilities) {
				bridge.PushRuntime.SetCapabilities(capabilities)
				if capabilities.ClientName != "" {
					telemetry.SetLLMName(capabilities.ClientName)
				}
			},
			ExtractCapabilities: pushapi.ExtractClientCapabilities, NegotiateVersion: mcp.NegotiateProtocolVersion,
			Resources: playbookresources.Resources, ResourceTemplates: playbookresources.ResourceTemplates,
			ResolveResource: playbookresources.ResolveResourceContent,
		},
		bridge.Lifecycle{
			ProcessArgv0: daemonProcessArgv0, StopServerForUpgrade: daemonrecovery.StopServerForUpgrade,
			FindProcessOnPort: procctl.FindProcessOnPort, IsProcessAlive: procctl.IsProcessAlive,
			AppendExitDiagnostic: exitDiagnostics.Append,
		},
	)
}

func bridgeRuntime() *bridge.Runner {
	return buildBridgeRunner(exitdiag.New(exitdiag.Options{Version: version}))
}

const (
	maxPostBodySize = 10 * 1024 * 1024 // 10 MB

	// Server health check parameters
	healthCheckMaxAttempts   = 30                     // 30 attempts * 100ms = 3 seconds total
	healthCheckRetryInterval = 100 * time.Millisecond // Retry interval between health check attempts
)

func main() {
	runtime := appruntime.New(version)
	runtime.SetBridgeRunner(buildBridgeRunner(runtime.ExitDiagnostics()))
	defer func() {
		if r := recover(); r != nil {
			runtime.ExitDiagnostics().Recover(r)
		}
	}()

	if len(os.Args) >= 2 && cli.IsCLIMode(os.Args[1:]) {
		os.Exit(cli.Run(os.Args[1:], cliRuntimeConfig(runtime)))
	}

	cfg := parseAndValidateFlags()

	server, err := NewServer(cfg.LogFile, cfg.MaxEntries)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] Error creating server: %v\n", err)
		os.Exit(1)
	}
	server.runtime = runtime
	server.uploadAutomation = cfg.UploadAutomation
	server.uploadSecurity = cfg.UploadSecurity
	for _, warning := range cfg.Warnings {
		server.warnings.Add(warning)
	}

	// Inert unless the integration harness started this process: a daemon owned
	// by a test exits when that test process dies, so a killed `go test` — which
	// never runs t.Cleanup — cannot leave the port and state directory held.
	stopOwnerWatch, _ := testowner.Watch(procctl.IsProcessAlive, func() {
		fmt.Fprintln(os.Stderr, "[Kaboom] test owner exited; shutting down test daemon")
		os.Exit(0)
	}, testowner.DefaultInterval)
	defer stopOwnerWatch()

	dispatchMode(server, cfg)
}
