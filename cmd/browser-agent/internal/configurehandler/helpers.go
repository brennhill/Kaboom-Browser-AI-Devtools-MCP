// helpers.go — Response builders and utilities for configure-local handlers.
// Why: Thin wrappers around internal/mcp so handlers use concise call sites.

package configurehandler

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// succeed builds a success JSONRPCResponse with a JSON summary + data payload.
func succeed(req mcp.JSONRPCRequest, summary string, data any) mcp.JSONRPCResponse {
	return mcp.JSONRPCResponse{JSONRPC: mcp.JSONRPCVersion, ID: req.ID, Result: mcp.JSONResponse(summary, data)}
}

// fail builds an error JSONRPCResponse with a structured error payload (isError=true).
func fail(req mcp.JSONRPCRequest, code, message, retry string, opts ...func(*mcp.StructuredError)) mcp.JSONRPCResponse {
	return mcp.JSONRPCResponse{JSONRPC: mcp.JSONRPCVersion, ID: req.ID, Result: mcp.StructuredErrorResponse(code, message, retry, opts...)}
}

// lenientUnmarshal unmarshals JSON args, ignoring errors.
func lenientUnmarshal(args json.RawMessage, v any) {
	if len(args) > 0 {
		_ = json.Unmarshal(args, v)
	}
}
