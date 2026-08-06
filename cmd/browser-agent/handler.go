// Purpose: Defines MCP handler core types, interfaces, and bootstrap wiring.
// Why: Keeps shared handler state concise while method behavior lives in focused files.

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/appruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcpprotocol"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcpresponse"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcptelemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
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
	tools   ToolBackend
	version string
	runtime *appruntime.Runtime

	passiveTelemetry *mcptelemetry.Owner
	responsePolicy   *mcpresponse.Owner
}

// ToolExecutor is the sole behavior required from the five-tool backend.
type ToolExecutor interface {
	HandleToolCall(req mcp.JSONRPCRequest, name string, arguments json.RawMessage) (mcp.JSONRPCResponse, bool)
}

// ToolBackend composes execution with MCP transport policy and telemetry owners.
type ToolBackend struct {
	Executor     ToolExecutor
	Capture      *capture.Capture
	Limiter      RateLimiter
	Redactor     RedactionEngine
	Schemas      []mcp.MCPTool
	UsageTracker *telemetry.UsageTracker
}

// RateLimiter interface for tool call rate limiting.
type RateLimiter interface {
	Allow() bool
}

// RedactionEngine interface for response redaction.
type RedactionEngine interface {
	Redact(input string) string
	RedactJSON(data json.RawMessage) json.RawMessage
	RedactMapValues(data map[string]any) map[string]any
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
func (h *MCPHandler) SetToolBackend(backend ToolBackend) {
	h.tools = backend
	h.passiveTelemetry.SetCapture(backend.Capture)
	h.responsePolicy.SetCapture(backend.Capture)
}

// GetUsageTracker returns the configured usage tracker.
func (h *MCPHandler) GetUsageTracker() *telemetry.UsageTracker {
	return h.tools.UsageTracker
}

type mcpMethodHandler func(handler *MCPHandler, request mcp.JSONRPCRequest) mcp.JSONRPCResponse

var mcpMethodHandlers = map[string]mcpMethodHandler{
	"initialize": func(handler *MCPHandler, request mcp.JSONRPCRequest) mcp.JSONRPCResponse {
		return mcpprotocol.Initialize(request, handler.version)
	},
	"tools/list": func(handler *MCPHandler, request mcp.JSONRPCRequest) mcp.JSONRPCResponse {
		return mcpprotocol.ToolsList(request, handler.tools.Schemas)
	},
	"tools/call": func(handler *MCPHandler, request mcp.JSONRPCRequest) mcp.JSONRPCResponse {
		return handler.handleToolsCall(request)
	},
	"resources/list": func(handler *MCPHandler, request mcp.JSONRPCRequest) mcp.JSONRPCResponse {
		return mcpprotocol.ResourcesList(request)
	},
	"resources/read": func(handler *MCPHandler, request mcp.JSONRPCRequest) mcp.JSONRPCResponse {
		return mcpprotocol.ResourcesRead(request)
	},
	"resources/templates/list": func(handler *MCPHandler, request mcp.JSONRPCRequest) mcp.JSONRPCResponse {
		return mcpprotocol.ResourceTemplatesList(request)
	},
}

var mcpStaticResponses = map[string]string{
	"initialized":  `{}`,
	"ping":         `{}`,
	"prompts/list": `{"prompts":[]}`,
}

// HandleRequest validates and routes one JSON-RPC request.
func (h *MCPHandler) HandleRequest(request mcp.JSONRPCRequest) *mcp.JSONRPCResponse {
	if request.HasInvalidID() {
		response := mcp.JSONRPCResponse{
			JSONRPC: mcp.JSONRPCVersion,
			ID:      nil,
			Error:   &mcp.JSONRPCError{Code: -32600, Message: "Invalid Request: id must be string or number when present"},
		}
		return &response
	}
	if !request.HasID() {
		return nil
	}
	if request.JSONRPC != mcp.JSONRPCVersion {
		return &mcp.JSONRPCResponse{
			JSONRPC: mcp.JSONRPCVersion,
			ID:      request.ID,
			Error:   &mcp.JSONRPCError{Code: -32600, Message: `Invalid Request: jsonrpc must be "2.0"`},
		}
	}
	if methodHandler, ok := mcpMethodHandlers[request.Method]; ok {
		response := methodHandler(h, request)
		if response.Result != nil {
			response.Result = mcp.ClampResponseSize(response.Result)
		}
		return &response
	}
	if staticResult, ok := mcpStaticResponses[request.Method]; ok {
		response := toolresp.SucceedRaw(request, json.RawMessage(staticResult))
		return &response
	}
	response := mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      request.ID,
		Error:   &mcp.JSONRPCError{Code: -32601, Message: "Method not found: " + request.Method},
	}
	return &response
}

// handleToolsCall validates tool call payload, executes the tool, and applies
// response guards and diagnostics owned by the MCP request lifecycle.
func (h *MCPHandler) handleToolsCall(req mcp.JSONRPCRequest) mcp.JSONRPCResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return mcp.JSONRPCResponse{
			JSONRPC: mcp.JSONRPCVersion, ID: req.ID,
			Error: &mcp.JSONRPCError{Code: -32602, Message: "Invalid params: " + err.Error()},
		}
	}
	if h.tools.Executor == nil {
		return mcp.JSONRPCResponse{
			JSONRPC: mcp.JSONRPCVersion, ID: req.ID,
			Error: &mcp.JSONRPCError{Code: -32601, Message: "Unknown tool: " + params.Name},
		}
	}
	h.responsePolicy.WarnUnknownArguments(params.Name, params.Arguments, h.tools.Schemas)
	if err := h.checkToolRateLimit(); err != nil {
		telemetry.AppError(incident.CodeToolRateLimited)
		return mcp.JSONRPCResponse{JSONRPC: mcp.JSONRPCVersion, ID: req.ID, Error: err}
	}
	resp, handled := h.tools.Executor.HandleToolCall(req, params.Name, params.Arguments)
	if !handled {
		return mcp.JSONRPCResponse{
			JSONRPC: mcp.JSONRPCVersion, ID: req.ID,
			Error: &mcp.JSONRPCError{Code: -32601, Message: "Unknown tool: " + params.Name},
		}
	}
	return h.applyToolResponsePostProcessing(resp, req.ClientID, params.Name, params.Arguments)
}

func (h *MCPHandler) checkToolRateLimit() *mcp.JSONRPCError {
	limiter := h.tools.Limiter
	if limiter != nil && !limiter.Allow() {
		return &mcp.JSONRPCError{
			Code:    -32603,
			Message: "Tool call rate limit exceeded (500 calls/minute). Please wait before retrying.",
		}
	}
	return nil
}

func (h *MCPHandler) applyToolResponsePostProcessing(resp mcp.JSONRPCResponse, clientID, toolName string, arguments json.RawMessage) mcp.JSONRPCResponse {
	redactor := h.tools.Redactor
	if redactor != nil && resp.Result != nil {
		resp.Result = redactor.RedactJSON(resp.Result)
	}
	resp = h.responsePolicy.Augment(resp, h.tools.Executor != nil)
	return h.passiveTelemetry.Augment(resp, clientID, toolName, arguments)
}
