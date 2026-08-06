// handler.go — Owns MCP tool-call validation, rate limiting, execution, and response policy.

package mcpcall

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcpresponse"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcptelemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

// Backend composes tool execution with MCP transport policy and telemetry owners.
type Backend struct {
	Executor interface {
		HandleToolCall(request mcp.JSONRPCRequest, name string, arguments json.RawMessage) (mcp.JSONRPCResponse, bool)
	}
	Capture *capture.Capture
	Limiter interface {
		Allow() bool
	}
	Redactor interface {
		RedactJSON(json.RawMessage) json.RawMessage
	}
	Schemas      []mcp.MCPTool
	UsageTracker *telemetry.UsageTracker
}

// Handle validates, executes, redacts, and augments one MCP tools/call request.
func Handle(request mcp.JSONRPCRequest, backend Backend, responses *mcpresponse.Owner, passive *mcptelemetry.Owner) mcp.JSONRPCResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(request.Params, &params); err != nil {
		return mcp.JSONRPCResponse{
			JSONRPC: mcp.JSONRPCVersion, ID: request.ID,
			Error: &mcp.JSONRPCError{Code: -32602, Message: "Invalid params: " + err.Error()},
		}
	}
	if backend.Executor == nil {
		return unknownTool(request, params.Name)
	}
	if responses == nil {
		responses = mcpresponse.New(mcpresponse.Config{})
	}
	if passive == nil {
		passive = mcptelemetry.New(mcptelemetry.Config{})
	}
	responses.WarnUnknownArguments(params.Name, params.Arguments, backend.Schemas)
	if backend.Limiter != nil && !backend.Limiter.Allow() {
		telemetry.AppError(incident.CodeToolRateLimited)
		return mcp.JSONRPCResponse{
			JSONRPC: mcp.JSONRPCVersion, ID: request.ID,
			Error: &mcp.JSONRPCError{Code: -32603, Message: "Tool call rate limit exceeded (500 calls/minute). Please wait before retrying."},
		}
	}
	response, handled := backend.Executor.HandleToolCall(request, params.Name, params.Arguments)
	if !handled {
		return unknownTool(request, params.Name)
	}
	if backend.Redactor != nil && response.Result != nil {
		response.Result = backend.Redactor.RedactJSON(response.Result)
	}
	response = responses.Augment(response, true)
	return passive.Augment(response, request.ClientID, params.Name, params.Arguments)
}

func unknownTool(request mcp.JSONRPCRequest, name string) mcp.JSONRPCResponse {
	return mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPCVersion, ID: request.ID,
		Error: &mcp.JSONRPCError{Code: -32601, Message: "Unknown tool: " + name},
	}
}
