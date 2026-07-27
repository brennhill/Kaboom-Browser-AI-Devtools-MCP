// bridge_adapter.go -- Wires the bridge sub-package to main-package dependencies.
// Purpose: Provides the dependency injection glue so the bridge package can call main-package functions.
// Why: Keeps the bridge package decoupled while allowing it to access logging, stdout, MCP identity, and daemon lifecycle helpers.

package main

import (
	"encoding/json"
	"sync"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/playbooks"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"

	bridgepkg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
)

// mcpStdoutMu serializes all writes to stdout so concurrent bridgeForwardRequest
// goroutines cannot interleave JSON-RPC responses.
var mcpStdoutMu sync.Mutex

// initBridge wires the bridge sub-package to main-package dependencies.
// Must be called before any bridge function is used.
func initBridge() {
	bridgepkg.Init(bridgepkg.Deps{
		Version:              version,
		MaxPostBodySize:      maxPostBodySize,
		MCPServerName:        mcpServerName,
		LegacyMCPServerNames: legacyMCPServerNames,
		ServerInstructions:   serverInstructions,

		// Logging
		Stderrf: stderrf,
		Debugf:  debugf,

		// Stdout transport
		WriteMCPPayload:      writeMCPPayload,
		SyncStdoutBestEffort: syncStdoutBestEffort,
		SetStderrSink:        setStderrSink,

		// Push state
		GetBridgeFraming:   getBridgeFraming,
		StoreBridgeFraming: storeBridgeFraming,
		SetPushClientCapabilities: func(caps push.ClientCapabilities) {
			setPushClientCapabilities(caps)
			if caps.ClientName != "" {
				telemetry.SetLLMName(caps.ClientName)
			}
		},
		ExtractClientCapabilities: func(rawParams json.RawMessage) push.ClientCapabilities {
			return extractClientCapabilities(rawParams)
		},

		// MCP content
		NegotiateProtocolVersion: negotiateProtocolVersion,
		MCPResources: func() []mcp.MCPResource {
			return playbooks.Resources()
		},
		MCPResourceTemplates: func() []any {
			return playbooks.ResourceTemplates()
		},
		ResolveResourceContent: playbooks.ResolveResourceContent,

		// Daemon lifecycle
		DaemonProcessArgv0:   daemonProcessArgv0,
		StopServerForUpgrade: stopServerForUpgrade,
		FindProcessOnPort:    procctl.FindProcessOnPort,
		IsProcessAlive:       procctl.IsProcessAlive,
		VersionsMatch:        versionsMatch,
		DecodeHealthMetadata: func(body []byte) (bridgepkg.HealthMeta, bool) {
			meta, ok := decodeHealthMetadata(body)
			if !ok {
				return bridgepkg.HealthMeta{}, false
			}
			return bridgepkg.HealthMeta{
				Version:     meta.Version,
				ServiceName: meta.resolvedServiceName(),
			}, true
		},
		AppendExitDiagnostic: appendExitDiagnostic,
	})
}

// Note: writeMCPPayload and mcpStdoutMu remain in mcp_stdout.go.
// The bridge package calls deps.WriteMCPPayload which routes back to main's writeMCPPayload.
