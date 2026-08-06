// router.go — Validates and routes MCP JSON-RPC requests without application state.

package mcprouter

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcpprotocol"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// Config supplies immutable protocol data and the sole stateful tool-call boundary.
type Config struct {
	Version  string
	Schemas  []mcp.MCPTool
	ToolCall func(mcp.JSONRPCRequest) mcp.JSONRPCResponse
}

// Handle validates and routes one JSON-RPC request. Notifications return nil.
func Handle(request mcp.JSONRPCRequest, config Config) *mcp.JSONRPCResponse {
	if request.HasInvalidID() {
		return responseError(nil, -32600, "Invalid Request: id must be string or number when present")
	}
	if !request.HasID() {
		return nil
	}
	if request.JSONRPC != mcp.JSONRPCVersion {
		return responseError(request.ID, -32600, `Invalid Request: jsonrpc must be "2.0"`)
	}
	if response, ok := dynamicResponse(request, config); ok {
		if response.Result != nil {
			response.Result = mcp.ClampResponseSize(response.Result)
		}
		return &response
	}
	if result, ok := staticResult(request.Method); ok {
		response := toolresp.SucceedRaw(request, json.RawMessage(result))
		return &response
	}
	return responseError(request.ID, -32601, "Method not found: "+request.Method)
}

func dynamicResponse(request mcp.JSONRPCRequest, config Config) (mcp.JSONRPCResponse, bool) {
	switch request.Method {
	case "initialize":
		return mcpprotocol.Initialize(request, config.Version), true
	case "tools/list":
		return mcpprotocol.ToolsList(request, config.Schemas), true
	case "tools/call":
		if config.ToolCall == nil {
			return mcp.JSONRPCResponse{}, false
		}
		return config.ToolCall(request), true
	case "resources/list":
		return mcpprotocol.ResourcesList(request), true
	case "resources/read":
		return mcpprotocol.ResourcesRead(request), true
	case "resources/templates/list":
		return mcpprotocol.ResourceTemplatesList(request), true
	default:
		return mcp.JSONRPCResponse{}, false
	}
}

func staticResult(method string) (string, bool) {
	switch method {
	case "initialized", "ping":
		return `{}`, true
	case "prompts/list":
		return `{"prompts":[]}`, true
	default:
		return "", false
	}
}

func responseError(id any, code int, message string) *mcp.JSONRPCResponse {
	return &mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      id,
		Error:   &mcp.JSONRPCError{Code: code, Message: message},
	}
}
