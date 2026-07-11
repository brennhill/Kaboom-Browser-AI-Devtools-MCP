// helpers.go — Local wrappers for MCP response helpers used across generate handlers.
// Purpose: Provides package-local convenience functions that delegate to internal/mcp.
// Why: Avoids importing the main package while keeping handler code concise.

package generatehandler

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// succeed builds a success JSONRPCResponse with a JSON summary + data payload.
func succeed(req mcp.JSONRPCRequest, summary string, data any) mcp.JSONRPCResponse {
	return mcp.Succeed(req, summary, data)
}

// succeedText builds a success JSONRPCResponse with a plain text payload.
func succeedText(req mcp.JSONRPCRequest, text string) mcp.JSONRPCResponse {
	return mcp.SucceedText(req, text)
}

// fail builds an error JSONRPCResponse with a structured error payload (isError=true).
func fail(req mcp.JSONRPCRequest, code, message, retry string, opts ...func(*mcp.StructuredError)) mcp.JSONRPCResponse {
	return mcp.Fail(req, code, message, retry, opts...)
}

// parseArgs unmarshals JSON args into v. Returns (resp, true) if parsing failed.
func parseArgs(req mcp.JSONRPCRequest, args json.RawMessage, v any) (mcp.JSONRPCResponse, bool) {
	return mcp.ParseArgs(req, args, v)
}

// lenientUnmarshal attempts to unmarshal args, ignoring errors.
func lenientUnmarshal(args json.RawMessage, v any) {
	mcp.LenientUnmarshal(args, v)
}
