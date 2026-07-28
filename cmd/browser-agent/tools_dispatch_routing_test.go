// Purpose: Tests canonical dispatch routing and recovery_tool_call behavior.
// Docs: docs/features/feature/analyze-tool/index.md

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestAnalyzeDispatch_NavigationPatterns(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.analyzeDispatcher.Handle(req, json.RawMessage(`{"what":"navigation_patterns"}`))
	result := parseToolResult(t, resp)
	// Should not be an "unknown mode" error
	if result.IsError && strings.Contains(result.Content[0].Text, "unknown_mode") {
		t.Fatalf("navigation_patterns should be a valid mode, got: %s", result.Content[0].Text)
	}
}

func TestToolDispatchRequiresCanonicalWhat(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	calls := map[string]func(json.RawMessage) mcp.JSONRPCResponse{
		"observe mode": func(args json.RawMessage) mcp.JSONRPCResponse {
			return h.observeDispatcher.Handle(req, args)
		},
		"analyze mode": func(args json.RawMessage) mcp.JSONRPCResponse {
			return h.analyzeDispatcher.Handle(req, args)
		},
		"generate format": func(args json.RawMessage) mcp.JSONRPCResponse {
			return h.generateDispatcher.Handle(req, args)
		},
		"configure action": func(args json.RawMessage) mcp.JSONRPCResponse {
			return h.toolConfigure(req, args)
		},
		"interact action": func(args json.RawMessage) mcp.JSONRPCResponse {
			return h.toolInteract(req, args)
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			field := strings.Split(name, " ")[1]
			result := parseToolResult(t, call(json.RawMessage(`{"`+field+`":"health"}`)))
			if !result.IsError || !strings.Contains(result.Content[0].Text, "missing_param") {
				t.Fatalf("%s selector must not substitute for what: %+v", field, result)
			}
		})
	}
}

func TestToolDispatchRejectsShorthandModeValues(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	calls := map[string]func() mcp.JSONRPCResponse{
		"observe network": func() mcp.JSONRPCResponse {
			return h.observeDispatcher.Handle(req, json.RawMessage(`{"what":"network"}`))
		},
		"observe ws": func() mcp.JSONRPCResponse {
			return h.observeDispatcher.Handle(req, json.RawMessage(`{"what":"ws"}`))
		},
		"analyze a11y": func() mcp.JSONRPCResponse {
			return h.analyzeDispatcher.Handle(req, json.RawMessage(`{"what":"a11y"}`))
		},
		"analyze history": func() mcp.JSONRPCResponse {
			return h.analyzeDispatcher.Handle(req, json.RawMessage(`{"what":"history"}`))
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			result := parseToolResult(t, call())
			if !result.IsError || !strings.Contains(result.Content[0].Text, "unknown_mode") {
				t.Fatalf("shorthand mode must be rejected: %+v", result)
			}
		})
	}
}

func TestUnknownMode_IncludesRecoveryToolCall_Observe(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.observeDispatcher.Handle(req, json.RawMessage(`{"what":"nonexistent_mode"}`))
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("expected error for unknown mode")
	}
	assertRecoveryToolCall(t, result, "observe")
}

func TestUnknownMode_IncludesRecoveryToolCall_Analyze(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.analyzeDispatcher.Handle(req, json.RawMessage(`{"what":"nonexistent_mode"}`))
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("expected error for unknown mode")
	}
	assertRecoveryToolCall(t, result, "analyze")
}

func TestUnknownMode_IncludesRecoveryToolCall_Generate(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.generateDispatcher.Handle(req, json.RawMessage(`{"what":"nonexistent_format"}`))
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("expected error for unknown mode")
	}
	assertRecoveryToolCall(t, result, "generate")
}

func TestUnknownMode_IncludesRecoveryToolCall_Configure(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.toolConfigure(req, json.RawMessage(`{"what":"nonexistent_action"}`))
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("expected error for unknown mode")
	}
	assertRecoveryToolCall(t, result, "configure")
}

func TestUnknownMode_IncludesRecoveryToolCall_Interact(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.toolInteract(req, json.RawMessage(`{"what":"nonexistent_action"}`))
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("expected error for unknown mode")
	}
	assertRecoveryToolCall(t, result, "interact")
}

// assertRecoveryToolCall checks that the error response contains a recovery_tool_call
// pointing to configure/describe_capabilities for the given tool.
func assertRecoveryToolCall(t *testing.T, result mcp.MCPToolResult, toolName string) {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("no content blocks in error response")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "recovery_tool_call") {
		t.Fatalf("error response should contain recovery_tool_call, got: %s", text)
	}
	if !strings.Contains(text, "describe_capabilities") {
		t.Fatalf("recovery_tool_call should reference describe_capabilities, got: %s", text)
	}
	if !strings.Contains(text, `"`+toolName+`"`) {
		t.Fatalf("recovery_tool_call should reference tool %q, got: %s", toolName, text)
	}
}
