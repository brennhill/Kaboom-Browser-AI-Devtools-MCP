// Purpose: Routes JSON-RPC MCP methods and builds typed MCP method responses.
// Why: Keeps method dispatch and response shaping separate from transport and tool-call post-processing.

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/playbooks"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// mcpMethodHandler is a function that handles a specific MCP method.
type mcpMethodHandler func(h *MCPHandler, req JSONRPCRequest) JSONRPCResponse

// mcpMethodHandlers maps MCP method names to their handlers.
var mcpMethodHandlers = map[string]mcpMethodHandler{
	"initialize":               func(h *MCPHandler, req JSONRPCRequest) JSONRPCResponse { return h.handleInitialize(req) },
	"tools/list":               func(h *MCPHandler, req JSONRPCRequest) JSONRPCResponse { return h.handleToolsList(req) },
	"tools/call":               func(h *MCPHandler, req JSONRPCRequest) JSONRPCResponse { return h.handleToolsCall(req) },
	"resources/list":           func(h *MCPHandler, req JSONRPCRequest) JSONRPCResponse { return h.handleResourcesList(req) },
	"resources/read":           func(h *MCPHandler, req JSONRPCRequest) JSONRPCResponse { return h.handleResourcesRead(req) },
	"resources/templates/list": func(h *MCPHandler, req JSONRPCRequest) JSONRPCResponse { return h.handleResourcesTemplatesList(req) },
}

// mcpStaticResponses maps MCP methods to static JSON result bodies.
var mcpStaticResponses = map[string]string{
	"initialized":  `{}`,
	"ping":         `{}`,
	"prompts/list": `{"prompts":[]}`,
}

// HandleRequest routes one JSON-RPC request to the corresponding MCP method.
//
// Invariants:
// - id validation and JSON-RPC version checks run before method dispatch.
//
// Failure semantics:
// - Invalid request shape yields JSON-RPC -32600.
// - Unknown method yields JSON-RPC -32601.
// - Notifications return nil by design.
func (h *MCPHandler) HandleRequest(req JSONRPCRequest) *JSONRPCResponse {
	if req.HasInvalidID() {
		resp := JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      nil,
			Error: &JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request: id must be string or number when present",
			},
		}
		return &resp
	}

	// Notifications do not get responses per JSON-RPC 2.0.
	if !req.HasID() {
		return nil
	}

	// JSON-RPC 2.0: All requests must include "jsonrpc": "2.0"
	if req.JSONRPC != JSONRPCVersion {
		return &JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error:   &JSONRPCError{Code: -32600, Message: `Invalid Request: jsonrpc must be "2.0"`},
		}
	}

	if handler, ok := mcpMethodHandlers[req.Method]; ok {
		resp := handler(h, req)
		if resp.Result != nil {
			resp.Result = mcp.ClampResponseSize(resp.Result)
		}
		return &resp
	}

	if staticResult, ok := mcpStaticResponses[req.Method]; ok {
		resp := succeedRaw(req, json.RawMessage(staticResult))
		return &resp
	}

	resp := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Error:   &JSONRPCError{Code: -32601, Message: "Method not found: " + req.Method},
	}
	return &resp
}

func (h *MCPHandler) handleInitialize(req JSONRPCRequest) JSONRPCResponse {
	negotiatedVersion := negotiateProtocolVersion(req.Params)

	result := MCPInitializeResult{
		ProtocolVersion: negotiatedVersion,
		ServerInfo: MCPServerInfo{
			Name:    mcpServerName,
			Version: h.version,
		},
		Capabilities: MCPCapabilities{
			Tools:     MCPToolsCapability{},
			Resources: MCPResourcesCapability{},
		},
		Instructions: serverInstructions,
	}

	// Error impossible: MCPInitResult is a simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	return succeedRaw(req, resultJSON)
}

func (h *MCPHandler) handleResourcesList(req JSONRPCRequest) JSONRPCResponse {
	result := MCPResourcesListResult{Resources: playbooks.Resources()}
	// Error impossible: MCPResourcesListResult is a simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	return succeedRaw(req, resultJSON)
}

func (h *MCPHandler) handleResourcesRead(req JSONRPCRequest) JSONRPCResponse {
	var params struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params: " + err.Error(),
			},
		}
	}

	canonicalURI, text, ok := playbooks.ResolveResourceContent(params.URI)
	if !ok {
		return JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32002,
				Message: "Resource not found: " + params.URI,
			},
		}
	}

	result := MCPResourcesReadResult{Contents: []MCPResourceContent{
		{URI: canonicalURI, MimeType: "text/markdown", Text: text},
	}}
	// Error impossible: MCPResourceContentResult is a simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	return succeedRaw(req, resultJSON)
}

func (h *MCPHandler) handleResourcesTemplatesList(req JSONRPCRequest) JSONRPCResponse {
	result := MCPResourceTemplatesListResult{ResourceTemplates: playbooks.ResourceTemplates()}
	// Error impossible: MCPResourceTemplatesListResult is a simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	return succeedRaw(req, resultJSON)
}

func (h *MCPHandler) handleToolsList(req JSONRPCRequest) JSONRPCResponse {
	var tools []MCPTool
	if h.toolHandler != nil {
		tools = h.toolHandler.ToolsList()
	}

	result := MCPToolsListResult{Tools: tools}
	// Error impossible: MCPToolsListResult is a simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	return succeedRaw(req, resultJSON)
}
