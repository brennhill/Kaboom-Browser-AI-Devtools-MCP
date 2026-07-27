// Purpose: Tests for dispatch routing improvements: rename, alias, and recovery_tool_call.
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
	resp := h.toolAnalyze(req, json.RawMessage(`{"what":"navigation_patterns"}`))
	result := parseToolResult(t, resp)
	// Should not be an "unknown mode" error
	if result.IsError && strings.Contains(result.Content[0].Text, "unknown_mode") {
		t.Fatalf("navigation_patterns should be a valid mode, got: %s", result.Content[0].Text)
	}
}

func TestAnalyzeDispatch_HistoryAlias(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.toolAnalyze(req, json.RawMessage(`{"what":"history"}`))
	result := parseToolResult(t, resp)
	// Should not be an "unknown mode" error — history is an alias for navigation_patterns
	if result.IsError && strings.Contains(result.Content[0].Text, "unknown_mode") {
		t.Fatalf("history should be a valid alias, got: %s", result.Content[0].Text)
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
	resp := h.toolAnalyze(req, json.RawMessage(`{"what":"nonexistent_mode"}`))
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
