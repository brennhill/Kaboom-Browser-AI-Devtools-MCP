// bridge_adapter.go -- Wires the bridge sub-package to main-package dependencies.
// Purpose: Provides the dependency injection glue so the bridge package can call main-package functions.
// Why: Keeps the bridge package decoupled while allowing it to access logging, stdout, MCP identity, and daemon lifecycle helpers.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	cmbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/playbooks"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/identity"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

// mcpStdoutMu serializes all writes to stdout so concurrent bridgeForwardRequest
// goroutines cannot interleave JSON-RPC responses.
var mcpStdoutMu sync.Mutex

// initBridge wires the bridge sub-package to main-package dependencies.
// Must be called before any bridge function is used.
func initBridge() {
	debugLogger := diag.NewDebugFileFromEnv()
	cmbridge.Init(cmbridge.Deps{
		Version:              version,
		MaxPostBodySize:      maxPostBodySize,
		MCPServerName:        identity.MCPServerName,
		LegacyMCPServerNames: identity.LegacyMCPServerNames,
		ServerInstructions:   serverInstructions,

		// Logging
		Stderrf: diag.Printf,
		Debugf:  debugLogger.Printf,

		// Stdout transport
		WriteMCPPayload:      writeMCPPayload,
		SyncStdoutBestEffort: syncStdoutBestEffort,
		SetStderrSink:        diag.SetSink,

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
		NegotiateProtocolVersion: mcp.NegotiateProtocolVersion,
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
		AppendExitDiagnostic: exitDiagnostics.Append,
	})
}

func syncStdoutBestEffort() {
	if err := cmbridge.ActiveMCPTransportWriter().Sync(); err != nil && !isIgnorableStdoutSyncError(err) {
		diag.Printf("[Kaboom] warning: stdout.Sync failed: %v\n", err)
	}
}

func isIgnorableStdoutSyncError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.EBADF) {
		return true
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return errors.Is(pathErr.Err, syscall.EINVAL) || errors.Is(pathErr.Err, syscall.EBADF)
	}
	return false
}

// writeMCPPayload is the only stdout emitter used by MCP wrapper responses.
func writeMCPPayload(payload []byte, framing bridge.StdioFraming) {
	normalized := normalizeMCPPayload(payload)
	out := cmbridge.ActiveMCPTransportWriter()
	mcpStdoutMu.Lock()
	defer mcpStdoutMu.Unlock()
	if framing == bridge.StdioFramingContentLength {
		_, _ = fmt.Fprintf(out, "Content-Length: %d\r\nContent-Type: application/json\r\n\r\n%s", len(normalized), normalized)
	} else {
		_, _ = out.Write(normalized)
		_, _ = out.Write([]byte("\n"))
	}
	cmbridge.FlushStdout()
}

func normalizeMCPPayload(payload []byte) []byte {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) > 0 && json.Valid(trimmed) {
		return trimmed
	}

	diag.Printf("[kaboom-bridge] ERROR: stdout invariant violation: invalid JSON payload (len=%d)\n", len(payload))
	errResp := mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      nil,
		Error: &mcp.JSONRPCError{
			Code:    -32603,
			Message: "Wrapper emitted invalid JSON payload",
		},
	}
	respJSON, _ := json.Marshal(errResp)
	return respJSON
}

// sendStartupError sends a JSON-RPC error response before exiting.
func sendStartupError(message string) {
	errResp := mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      "startup",
		Error: &mcp.JSONRPCError{
			Code:    -32603,
			Message: message,
		},
	}
	respJSON, _ := json.Marshal(errResp)
	writeMCPPayload(respJSON, bridge.StdioFramingLine)
	syncStdoutBestEffort()
	time.Sleep(100 * time.Millisecond)
}
