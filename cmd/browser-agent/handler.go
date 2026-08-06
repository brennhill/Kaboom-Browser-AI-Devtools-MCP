// Purpose: Defines MCP handler core types, interfaces, and bootstrap wiring.
// Why: Keeps shared handler state concise while method behavior lives in focused files.

package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/appruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcpcall"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcpresponse"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcprouter"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcptelemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

// MCPHandler owns JSON-RPC request routing and response post-processing for MCP.
//
// Invariants:
// - tools is configured once during bootstrap before serving requests.
// - passive telemetry cursor synchronization is owned by mcptelemetry.Owner.
//
// Failure semantics:
// - Unknown methods/tools return JSON-RPC method-not-found errors.
// - Notification requests (no id) intentionally produce no response.
type MCPHandler struct {
	server  *Server
	tools   mcpcall.Backend
	version string
	runtime *appruntime.Runtime

	passiveTelemetry *mcptelemetry.Owner
	responsePolicy   *mcpresponse.Owner
}

// NewMCPHandler creates a new MCP handler.
func NewMCPHandler(server *Server, version string) *MCPHandler {
	runtime := appruntime.New(version)
	telemetryConfig := mcptelemetry.Config{}
	if server != nil && server.runtime != nil {
		runtime = server.runtime
	}
	if server != nil && server.logs != nil {
		telemetryConfig.ErrorTotal = server.logs.ErrorTotalAdded
		telemetryConfig.Mode = server.logs.TelemetryMode
	}
	responseConfig := mcpresponse.Config{Runtime: runtime}
	if server != nil {
		responseConfig.AddWarning = server.warnings.Add
		responseConfig.DrainWarnings = server.warnings.Drain
		responseConfig.PendingAudit = func() bool {
			return server.intentStore != nil && server.intentStore.NudgeAndClean()
		}
	}
	return &MCPHandler{
		server: server, version: version, runtime: runtime,
		passiveTelemetry: mcptelemetry.New(telemetryConfig),
		responsePolicy:   mcpresponse.New(responseConfig),
	}
}

// SetToolBackend injects the tool execution and transport dependencies.
//
// Invariants:
// - Intended for one-time startup wiring; runtime swapping is unsupported.
func (h *MCPHandler) SetToolBackend(backend mcpcall.Backend) {
	h.tools = backend
	h.passiveTelemetry.SetCapture(backend.Capture)
	h.responsePolicy.SetCapture(backend.Capture)
}

// GetUsageTracker returns the configured usage tracker.
func (h *MCPHandler) GetUsageTracker() *telemetry.UsageTracker {
	return h.tools.UsageTracker
}

// HandleRequest validates and routes one JSON-RPC request.
func (h *MCPHandler) HandleRequest(request mcp.JSONRPCRequest) *mcp.JSONRPCResponse {
	return mcprouter.Handle(request, mcprouter.Config{
		Version: h.version,
		Schemas: h.tools.Schemas,
		ToolCall: func(request mcp.JSONRPCRequest) mcp.JSONRPCResponse {
			return mcpcall.Handle(request, h.tools, h.responsePolicy, h.passiveTelemetry)
		},
	})
}
