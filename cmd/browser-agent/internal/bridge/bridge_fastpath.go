// bridge_fastpath.go -- Answers initialize/tools/list/resources directly in the
// bridge, before a daemon exists, and records what those answers cost.
// Why: fast start — a client must get a usable handshake while the daemon is
// still spawning. The telemetry counters live here rather than in their own file
// because they have exactly one producer (handleFastPath) and exist only to
// explain its behaviour; the split implied a reusable subsystem that never was.
// Docs: docs/features/feature/lazy-server-start/index.md

package bridge

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge/fastpathtelemetry"
	internbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// fastPathResponses maps MCP methods to their static JSON result bodies.
// Methods in this map are handled without waiting for the daemon.
var fastPathResponses = map[string]string{
	"ping":         `{}`,
	"prompts/list": `{"prompts":[]}`,
}

// sendFastResponse marshals and sends a JSON-RPC response for the fast path.
func (r *Runner) sendFastResponse(id any, result json.RawMessage, framing internbridge.StdioFraming) {
	resp := mcp.JSONRPCResponse{JSONRPC: mcp.JSONRPCVersion, ID: id, Result: result}
	// Error impossible: simple struct with no circular refs or unsupported types
	respJSON, _ := json.Marshal(resp)
	r.transport.Write(respJSON, framing)
}

func (r *Runner) sendFastError(id any, code int, message string, framing internbridge.StdioFraming) {
	resp := mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      id,
		Error:   &mcp.JSONRPCError{Code: code, Message: message},
	}
	respJSON, _ := json.Marshal(resp)
	r.transport.Write(respJSON, framing)
}

// handleFastPath handles MCP methods that don't require the daemon.
// Returns true if the method was handled.
func (r *Runner) handleFastPath(req mcp.JSONRPCRequest, toolsList []mcp.MCPTool, framing internbridge.StdioFraming) bool {
	if req.HasInvalidID() {
		r.sendBridgeError(nil, -32600, "Invalid Request: id must be string or number when present", framing)
		return true
	}

	// JSON-RPC notifications are fire-and-forget; never respond on stdio.
	if !req.HasID() {
		return true
	}

	switch req.Method {
	case "initialize":
		// Extract client capabilities for push delivery pipeline
		caps := r.protocol.ExtractCapabilities(req.Params)
		r.protocol.SetCapabilities(caps)
		r.protocol.StoreFraming(framing)

		result := map[string]any{
			"protocolVersion": r.protocol.NegotiateVersion(req.Params),
			"serverInfo":      map[string]any{"name": r.identity.ServerName, "version": r.identity.Version},
			"capabilities":    map[string]any{"tools": map[string]any{}, "resources": map[string]any{}},
			"instructions":    r.identity.ServerInstructions,
		}
		// Error impossible: map contains only primitive types and nested maps
		resultJSON, _ := json.Marshal(result)
		r.sendFastResponse(req.ID, resultJSON, framing)
		fastpathtelemetry.RecordMethod(r.identity.Version, req.Method, true, 0)
		return true

	case "initialized":
		if req.HasID() {
			r.sendFastResponse(req.ID, json.RawMessage(`{}`), framing)
			fastpathtelemetry.RecordMethod(r.identity.Version, req.Method, true, 0)
		}
		return true

	case "tools/list":
		result := map[string]any{"tools": toolsList}
		// Error impossible: map contains only serializable tool definitions
		resultJSON, _ := json.Marshal(result)
		r.sendFastResponse(req.ID, resultJSON, framing)
		fastpathtelemetry.RecordMethod(r.identity.Version, req.Method, true, 0)
		return true

	case "resources/list":
		result := mcp.MCPResourcesListResult{Resources: r.protocol.Resources()}
		resultJSON, _ := json.Marshal(result)
		r.sendFastResponse(req.ID, resultJSON, framing)
		return true
	case "resources/templates/list":
		result := mcp.MCPResourceTemplatesListResult{ResourceTemplates: r.protocol.ResourceTemplates()}
		resultJSON, _ := json.Marshal(result)
		r.sendFastResponse(req.ID, resultJSON, framing)
		return true
	case "resources/read":
		var params struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			fastpathtelemetry.RecordResourceRead(r.identity.Version, "", false, -32602)
			fastpathtelemetry.RecordMethod(r.identity.Version, req.Method, false, -32602)
			r.sendFastError(req.ID, -32602, "Invalid params: "+err.Error(), framing)
			return true
		}
		canonicalURI, text, ok := r.protocol.ResolveResource(params.URI)
		if !ok {
			fastpathtelemetry.RecordResourceRead(r.identity.Version, params.URI, false, -32002)
			fastpathtelemetry.RecordMethod(r.identity.Version, req.Method, false, -32002)
			r.sendFastError(req.ID, -32002, "Resource not found: "+params.URI, framing)
			return true
		}
		fastpathtelemetry.RecordResourceRead(r.identity.Version, params.URI, true, 0)
		fastpathtelemetry.RecordMethod(r.identity.Version, req.Method, true, 0)
		result := map[string]any{
			"contents": []map[string]any{
				{
					"uri":      canonicalURI,
					"mimeType": "text/markdown",
					"text":     text,
				},
			},
		}
		resultJSON, _ := json.Marshal(result)
		r.sendFastResponse(req.ID, resultJSON, framing)
		return true
	}

	if staticResult, ok := fastPathResponses[req.Method]; ok {
		r.sendFastResponse(req.ID, json.RawMessage(staticResult), framing)
		fastpathtelemetry.RecordMethod(r.identity.Version, req.Method, true, 0)
		return true
	}

	return false
}
