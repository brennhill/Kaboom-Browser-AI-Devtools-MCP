// response.go — Builds MCP tool results and JSON-RPC responses.
// Purpose: Defensive JSON marshal/unmarshal helpers plus the canonical
// Succeed/SucceedText/Fail/ParseArgs vocabulary every tool package calls.
// Why: One place owns "turn a summary + payload into a wire response", so the
// helpers cannot drift into per-tool copies again.
// Docs: docs/features/feature/query-service/index.md
package mcp

import (
	"encoding/json"
	"fmt"
	"os"
)

// SafeMarshal performs defensive JSON marshaling with a fallback value.
func SafeMarshal(v any, fallback string) json.RawMessage {
	resultJSON, err := json.Marshal(v)
	if err != nil {
		// This should never happen with simple structs, but handle it defensively
		fmt.Fprintf(os.Stderr, "[Kaboom] JSON marshal error: %v\n", err)
		return json.RawMessage(fallback)
	}
	return json.RawMessage(resultJSON)
}

// LenientUnmarshal parses optional JSON params, logging failures to stderr for debugging.
// Behavior is deliberately lenient: malformed optional params are logged but not rejected,
// allowing callers to fall through to defaults.
func LenientUnmarshal(args json.RawMessage, v any) {
	if len(args) == 0 {
		return
	}
	if err := json.Unmarshal(args, v); err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] optional param parse: %v (args: %.100s)\n", err, string(args))
	}
}

// TextResponse constructs an MCP tool result containing a single text content block.
func TextResponse(text string) json.RawMessage {
	result := MCPToolResult{
		Content: []MCPContentBlock{
			{Type: "text", Text: text},
		},
	}
	return SafeMarshal(result, `{"content":[{"type":"text","text":"Internal error: failed to marshal result"}]}`)
}

// errorResponse constructs an MCP tool error result containing a single text content block.
func errorResponse(text string) json.RawMessage {
	result := MCPToolResult{
		Content: []MCPContentBlock{
			{Type: "text", Text: text},
		},
		IsError: true,
	}
	return SafeMarshal(result, `{"content":[{"type":"text","text":"Internal error: failed to marshal result"}],"isError":true}`)
}

func marshalSummaryData(summary string, data any) (string, error) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	if summary == "" {
		return string(dataJSON), nil
	}
	return summary + "\n" + string(dataJSON), nil
}

func jsonResultWithSummary(summary string, data any, isError bool) json.RawMessage {
	text, err := marshalSummaryData(summary, data)
	if err != nil {
		return errorResponse("Failed to serialize response: " + err.Error())
	}
	result := MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: text}},
		IsError: isError,
	}
	if isError {
		return SafeMarshal(result, `{"content":[{"type":"text","text":"Internal error: failed to marshal result"}],"isError":true}`)
	}
	return SafeMarshal(result, `{"content":[{"type":"text","text":"Internal error: failed to marshal result"}]}`)
}

// JSONErrorResponse constructs an MCP tool error result with a summary line
// followed by compact JSON. Sets IsError: true so LLMs recognize the failure.
func JSONErrorResponse(summary string, data any) json.RawMessage {
	return jsonResultWithSummary(summary, data, true)
}

// JSONResponse constructs an MCP tool result with a summary line prefix
// followed by compact JSON. Use for nested, irregular, or highly variable data.
func JSONResponse(summary string, data any) json.RawMessage {
	return jsonResultWithSummary(summary, data, false)
}

// Succeed wraps a JSONResponse result in a JSONRPCResponse for req.
func Succeed(req JSONRPCRequest, summary string, data any) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: JSONResponse(summary, data)}
}

// SucceedText wraps a TextResponse result in a JSONRPCResponse for req.
func SucceedText(req JSONRPCRequest, text string) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: req.ID, Result: TextResponse(text)}
}

// Fail builds an error JSONRPCResponse with a structured error payload (isError=true).
func Fail(req JSONRPCRequest, code, message, recovery string, opts ...func(*StructuredError)) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: req.ID, Result: StructuredErrorResponse(code, message, recovery, opts...)}
}

// ParseArgs unmarshals JSON args into dest. Returns (resp, true) if parsing failed.
func ParseArgs(req JSONRPCRequest, args json.RawMessage, dest any) (JSONRPCResponse, bool) {
	if err := json.Unmarshal(args, dest); err != nil {
		return Fail(req, ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again"), true
	}
	return JSONRPCResponse{}, false
}
