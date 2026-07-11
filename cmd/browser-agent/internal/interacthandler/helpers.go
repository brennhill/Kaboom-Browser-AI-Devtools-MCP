// helpers.go — Local wrappers for MCP response helpers used across interact handlers.
// Purpose: Provides package-local convenience functions that delegate to internal/mcp.
// Why: Avoids importing the main package while keeping handler code concise.

package interacthandler

import (
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// Type aliases for brevity within the package.
type JSONRPCRequest = mcp.JSONRPCRequest
type JSONRPCResponse = mcp.JSONRPCResponse
type MCPToolResult = mcp.MCPToolResult
type MCPContentBlock = mcp.MCPContentBlock
type StructuredError = mcp.StructuredError

// JSONRPCVersion re-exports the protocol version.
const JSONRPCVersion = mcp.JSONRPCVersion

// Error code re-exports.
const (
	ErrInvalidJSON          = mcp.ErrInvalidJSON
	ErrMissingParam         = mcp.ErrMissingParam
	ErrInvalidParam         = mcp.ErrInvalidParam
	ErrUnknownMode          = mcp.ErrUnknownMode
	ErrPathNotAllowed       = mcp.ErrPathNotAllowed
	ErrNotInitialized       = mcp.ErrNotInitialized
	ErrNoData               = mcp.ErrNoData
	ErrCodePilotDisabled    = mcp.ErrCodePilotDisabled
	ErrOsAutomationDisabled = mcp.ErrOsAutomationDisabled
	ErrRateLimited          = mcp.ErrRateLimited
	ErrCursorExpired        = mcp.ErrCursorExpired
	ErrExtTimeout           = mcp.ErrExtTimeout
	ErrExtError             = mcp.ErrExtError
	ErrQueueFull            = mcp.ErrQueueFull
	ErrInternal             = mcp.ErrInternal
	ErrMarshalFailed        = mcp.ErrMarshalFailed
	ErrExportFailed         = mcp.ErrExportFailed
)

// succeed builds a success JSONRPCResponse with a JSON summary + data payload.
func succeed(req JSONRPCRequest, summary string, data any) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: req.ID, Result: mcp.JSONResponse(summary, data)}
}

// fail builds an error JSONRPCResponse with a structured error payload (isError=true).
func fail(req JSONRPCRequest, code, message, retry string, opts ...func(*StructuredError)) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: req.ID, Result: mcp.StructuredErrorResponse(code, message, retry, opts...)}
}

// parseArgs unmarshals JSON args into v. Returns (resp, true) if parsing failed.
func parseArgs(req JSONRPCRequest, args json.RawMessage, v any) (JSONRPCResponse, bool) {
	if err := json.Unmarshal(args, v); err != nil {
		return fail(req, ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again"), true
	}
	return JSONRPCResponse{}, false
}

// requireString validates that a string parameter is non-empty.
func requireString(req JSONRPCRequest, value, paramName, hint string) (JSONRPCResponse, bool) {
	if value != "" {
		return JSONRPCResponse{}, false
	}
	return fail(req, ErrMissingParam,
		"Required parameter '"+paramName+"' is missing",
		hint,
		withParam(paramName)), true
}

// lenientUnmarshal attempts to unmarshal args, ignoring errors.
func lenientUnmarshal(args json.RawMessage, v any) {
	mcp.LenientUnmarshal(args, v)
}

// buildQueryParams marshals a string-keyed map into JSON for query dispatch.
func buildQueryParams(fields map[string]any) json.RawMessage {
	return mcp.SafeMarshal(fields, "{}")
}

// safeMarshal marshals v to JSON, returning fallback on error.
func safeMarshal(v any, fallback string) json.RawMessage {
	return mcp.SafeMarshal(v, fallback)
}

// StructuredError option helpers.
func withParam(p string) func(*StructuredError)    { return mcp.WithParam(p) }
func withHint(h string) func(*StructuredError)     { return mcp.WithHint(h) }
func withAction(a string) func(*StructuredError)   { return mcp.WithAction(a) }
func withSelector(s string) func(*StructuredError) { return mcp.WithSelector(s) }
func withRetryable(retryable bool) func(*StructuredError) {
	return mcp.WithRetryable(retryable)
}
func withRetryAfterMs(ms int) func(*StructuredError) { return mcp.WithRetryAfterMs(ms) }
func withFinal(final bool) func(*StructuredError)    { return mcp.WithFinal(final) }
func withRecoveryToolCall(toolCall map[string]any) func(*StructuredError) {
	return mcp.WithRecoveryToolCall(toolCall)
}

// checkGuards runs guard checks in sequence. First blocker short-circuits.
func checkGuards(req JSONRPCRequest, guards ...GuardCheck) (JSONRPCResponse, bool) {
	for _, g := range guards {
		if resp, blocked := g(req); blocked {
			return resp, true
		}
	}
	return JSONRPCResponse{}, false
}

// checkGuardsWithOpts runs guard checks with StructuredError options.
func checkGuardsWithOpts(req JSONRPCRequest, opts []func(*StructuredError), guards ...GuardCheck) (JSONRPCResponse, bool) {
	for _, g := range guards {
		if resp, blocked := g(req, opts...); blocked {
			return resp, true
		}
	}
	return JSONRPCResponse{}, false
}

// mutateToolResult unmarshals the response result into MCPToolResult, applies fn, and remarshals.
func mutateToolResult(resp JSONRPCResponse, fn func(*MCPToolResult)) JSONRPCResponse {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}
	fn(&result)
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return resp
	}
	resp.Result = json.RawMessage(resultJSON)
	return resp
}

// appendWarningsToResponse wraps mcp.AppendWarningsToResponse.
func appendWarningsToResponse(resp JSONRPCResponse, warnings []string) JSONRPCResponse {
	return mcp.AppendWarningsToResponse(resp, warnings)
}

// newCorrelationID generates a unique correlation ID with the given prefix.
func newCorrelationID(prefix string) string {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), time.Now().UnixNano())
	}
	n := int64(b[0]) | int64(b[1])<<8 | int64(b[2])<<16 | int64(b[3])<<24 |
		int64(b[4])<<32 | int64(b[5])<<40 | int64(b[6])<<48 | int64(b[7]&0x7f)<<56
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), n)
}
