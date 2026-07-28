// Purpose: Tests for analyze link-health endpoint checking.
// Docs: docs/features/feature/analyze-tool/index.md

// tools_analyze_link_health_test.go — Unit tests for analyze tool link_health mode.
// Tests verify that link health checks create proper pending queries and return
// expected correlation IDs for async tracking.
//
// Run: go test ./cmd/browser-agent -run "TestAnalyzeLinkHealth" -v
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// Test Infrastructure
// ============================================

type analyzeTestEnv struct {
	handler *ToolHandler
	server  *Server
	capture *capture.Capture
}

func newAnalyzeTestEnv(t *testing.T) *analyzeTestEnv {
	t.Helper()
	server, err := NewServer(t.TempDir()+"/test-analyze-link-health.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	handler := mcpHandler.toolHandler.(*ToolHandler)
	return &analyzeTestEnv{handler: handler, server: server, capture: cap}
}

// callAnalyze invokes the analyze tool and returns parsed result
func (e *analyzeTestEnv) callAnalyze(t *testing.T, argsJSON string) (mcp.MCPToolResult, bool) {
	t.Helper()

	args := normalizeAnalyzeArgsForAsync(argsJSON)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := e.handler.analyzeDispatcher.Handle(req, args)

	if resp.Result == nil {
		return mcp.MCPToolResult{}, false
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	return result, true
}

// ============================================
// Behavioral Tests: Dispatcher
// ============================================

// TestAnalyze_Dispatcher_ValidMode verifies analyze routes to correct handler
func TestAnalyze_Dispatcher_ValidMode(t *testing.T) {
	env := newAnalyzeTestEnv(t)

	// Test with a valid mode that exists (dom - moved from configure)
	result, ok := env.callAnalyze(t, `{"what":"dom","selector":"body"}`)
	if !ok {
		t.Fatal("analyze should return result for valid mode")
	}

	// Should not be an error (or should handle gracefully)
	if result.IsError && !strings.Contains(result.Content[0].Text, "extension") {
		t.Errorf("analyze with valid mode should not error (unless extension issue)\nGot: %+v", result)
	}
}

// TestAnalyze_Dispatcher_MissingWhat verifies missing 'what' parameter returns error
func TestAnalyze_Dispatcher_MissingWhat(t *testing.T) {
	env := newAnalyzeTestEnv(t)

	result, ok := env.callAnalyze(t, `{}`)
	if !ok {
		t.Fatal("analyze should return result even for error cases")
	}

	// Should be an error
	if !result.IsError {
		t.Error("analyze without 'what' should return isError")
	}

	// Error should mention the missing parameter
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "what") {
		t.Errorf("error message should mention 'what' parameter\nGot: %s", text)
	}
}

// TestAnalyze_Dispatcher_InvalidMode verifies invalid mode returns error
func TestAnalyze_Dispatcher_InvalidMode(t *testing.T) {
	env := newAnalyzeTestEnv(t)

	result, ok := env.callAnalyze(t, `{"what":"invalid_mode_xyz"}`)
	if !ok {
		t.Fatal("analyze should return result even for invalid mode")
	}

	// Should be an error
	if !result.IsError {
		t.Error("analyze with invalid mode should return isError")
	}

	// Error should mention the invalid mode
	text := result.Content[0].Text
	if !strings.Contains(text, "invalid_mode_xyz") {
		t.Errorf("error message should mention invalid mode\nGot: %s", text)
	}
}

// TestAnalyze_Dispatcher_ValidModes verifies all expected modes are registered
func TestAnalyze_Dispatcher_ValidModes(t *testing.T) {
	expectedModes := []string{
		"dom",
		"api_validation",
		"performance",
		"accessibility",
		"error_clusters",
		"history",
		"security_audit",
		"third_party_audit",
		"link_health",
	}

	// Call with invalid mode to get the error message with valid modes list
	env := newAnalyzeTestEnv(t)
	result, ok := env.callAnalyze(t, `{"what":"invalid"}`)
	if !ok {
		t.Fatal("should return result for error")
	}

	text := result.Content[0].Text
	for _, mode := range expectedModes {
		if !strings.Contains(text, mode) {
			t.Errorf("valid modes list should contain '%s'\nGot: %s", mode, text)
		}
	}
}

// ============================================
// Behavioral Tests: Link Health
// ============================================

// TestAnalyzeLinkHealth_Start_ReturnsCorrelationID verifies link health returns correlation_id
func TestAnalyzeLinkHealth_Start_ReturnsCorrelationID(t *testing.T) {
	env := newAnalyzeTestEnv(t)

	result, ok := env.callAnalyze(t, `{"what":"link_health"}`)
	if !ok {
		t.Fatal("link_health should return result")
	}

	// ASSERTION 1: Not an error
	if result.IsError {
		t.Errorf("link_health should NOT return isError\nGot: %+v", result)
	}

	// ASSERTION 2: Has content
	if len(result.Content) == 0 {
		t.Fatal("link_health should return content block")
	}

	// ASSERTION 3: Content contains correlation_id reference
	text := result.Content[0].Text
	if !strings.Contains(text, "correlation_id") {
		t.Errorf("link_health response should mention correlation_id\nGot: %s", text)
	}
}

// TestAnalyzeLinkHealth_Start_CreatesWatchableQuery verifies query is created for async tracking
func TestAnalyzeLinkHealth_Start_CreatesWatchableQuery(t *testing.T) {
	env := newAnalyzeTestEnv(t)

	// Make the call
	result, ok := env.callAnalyze(t, `{"what":"link_health","timeout_ms":15000}`)
	if !ok {
		t.Fatal("link_health should return result")
	}

	if result.IsError {
		t.Fatalf("link_health should not error: %s", result.Content[0].Text)
	}

	// ASSERTION: Pending queries should have been created
	// (Note: We can't directly inspect pending queries without exposing internals,
	// but we verify the response indicates async operation)
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "queued") &&
		!strings.Contains(strings.ToLower(text), "initiated") {
		t.Errorf("link_health response should indicate async operation\nGot: %s", text)
	}
}

// TestAnalyzeLinkHealth_DomainParamForwarded verifies link_health passes domain through.
func TestAnalyzeLinkHealth_DomainParamForwarded(t *testing.T) {
	env := newAnalyzeTestEnv(t)

	result, ok := env.callAnalyze(t, `{"what":"link_health","domain":"example.com","timeout_ms":15000}`)
	if !ok {
		t.Fatal("link_health should return result")
	}
	if result.IsError {
		t.Fatalf("link_health with domain should not error: %s", result.Content[0].Text)
	}

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("link_health should create a pending query")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("failed to parse pending query params: %v", err)
	}

	if got, ok := params["domain"].(string); !ok || got != "example.com" {
		t.Fatalf("domain not forwarded, got %#v", params["domain"])
	}
}

// TestAnalyzeLinkHealth_InvalidJSON returns error
func TestAnalyzeLinkHealth_InvalidJSON(t *testing.T) {
	env := newAnalyzeTestEnv(t)

	result, ok := env.callAnalyze(t, `{invalid json`)
	if !ok {
		t.Fatal("analyze should return result even for invalid JSON")
	}

	// Should be an error
	if !result.IsError {
		t.Error("analyze with invalid JSON should return isError")
	}

	// Error should mention JSON syntax
	text := strings.ToLower(result.Content[0].Text)
	if !strings.Contains(text, "json") {
		t.Errorf("error should mention JSON issue\nGot: %s", result.Content[0].Text)
	}
}

// ============================================
// Safety Net Tests
// ============================================

// TestAnalyzeLinkHealth_NoPanic verifies link_health doesn't panic
func TestAnalyzeLinkHealth_NoPanic(t *testing.T) {
	env := newAnalyzeTestEnv(t)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("link_health should not panic: %v", r)
		}
	}()

	env.callAnalyze(t, `{"what":"link_health"}`)
	env.callAnalyze(t, `{"what":"link_health","timeout_ms":10000}`)
	env.callAnalyze(t, `{"what":"link_health","max_workers":5}`)
}

// TestAnalyze_AllModes_NoPanic verifies all modes handle their calls without panicking
func TestAnalyze_AllModes_NoPanic(t *testing.T) {
	env := newAnalyzeTestEnv(t)
	modes := []string{
		"dom",
		"api_validation",
		"performance",
		"accessibility",
		"error_clusters",
		"history",
		"security_audit",
		"third_party_audit",
		"link_health",
	}

	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("mode %s should not panic: %v", mode, r)
				}
			}()

			env.callAnalyze(t, `{"what":"`+mode+`"}`)
		})
	}
}
