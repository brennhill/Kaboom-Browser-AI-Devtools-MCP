// Purpose: Tests for generate tool warning message filtering.
// Docs: docs/features/feature/test-generation/index.md

// tools_generate_warning_filter_test.go — Regression tests for generate dispatch warnings.
package main

import (
	"encoding/json"
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestGenerateTestFromContext_NoWarningsForDispatchParams(t *testing.T) {
	t.Parallel()

	h := newTestToolHandler()
	h.capture.Telemetry().AddEnhancedActionsForTest([]types.EnhancedAction{
		{Type: "navigate", Timestamp: 1000, URL: "https://example.com", ToURL: "https://example.com"},
		{Type: "click", Timestamp: 1200, URL: "https://example.com", Selectors: map[string]any{"text": "Login"}},
	})

	resp := callGenerateRaw(h, `{"what":"test_from_context","context":"interaction","telemetry_mode":"auto","save_to":"tests/generated.spec.ts"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("generate(test_from_context) should succeed, got error: %s", firstText(result))
	}
	if warningsText, ok := warningsBlock(result); ok {
		t.Fatalf("did not expect warnings for dispatch params, got %q", warningsText)
	}
}

func TestGenerateTestClassify_NoWarningsForDispatchParams(t *testing.T) {
	t.Parallel()

	h := newTestToolHandler()

	resp := callGenerateRaw(h, `{"what":"test_classify","action":"failure","telemetry_mode":"auto","save_to":"tmp/classify.json","failure":{"test_name":"login test","error":"Timeout waiting for selector \"#login-btn\""}}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("generate(test_classify) should succeed, got error: %s", firstText(result))
	}
	if warningsText, ok := warningsBlock(result); ok {
		t.Fatalf("did not expect warnings for dispatch params, got %q", warningsText)
	}
}

func TestHandleGenerateTestFromContext_FiltersOnlyDispatchWarnings(t *testing.T) {
	t.Parallel()

	h := newTestToolHandler()
	h.capture.Telemetry().AddEnhancedActionsForTest([]types.EnhancedAction{
		{Type: "navigate", Timestamp: 1000, URL: "https://example.com", ToURL: "https://example.com"},
		{Type: "click", Timestamp: 1200, URL: "https://example.com", Selectors: map[string]any{"text": "Login"}},
	})

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.testGenHandler.HandleGenerateTestFromContext(req, json.RawMessage(`{"what":"test_from_context","context":"interaction","typo_field":true}`))
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("handleGenerateTestFromContext should succeed, got error: %s", firstText(result))
	}

	warningsText, ok := warningsBlock(result)
	if !ok {
		t.Fatal("expected warnings block for typo_field")
	}
	if !strings.Contains(warningsText, "typo_field") {
		t.Fatalf("expected warning to include typo_field, got %q", warningsText)
	}
	if strings.Contains(warningsText, "what") {
		t.Fatalf("warning should not include dispatch param 'what', got %q", warningsText)
	}
}

func TestGenerateTestHeal_NoWarningsForDispatchParams(t *testing.T) {
	t.Parallel()

	h := newTestToolHandler()
	testDir := makeProjectTempDir(t)

	resp := callGenerateRaw(h, fmt.Sprintf(`{"what":"test_heal","action":"batch","test_dir":%q,"telemetry_mode":"auto","save_to":"tmp/heal.json"}`, testDir))
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("generate(test_heal) should succeed, got error: %s", firstText(result))
	}
	if warningsText, ok := warningsBlock(result); ok {
		t.Fatalf("did not expect warnings for dispatch params, got %q", warningsText)
	}
}

func TestHandleGenerateTestHeal_FiltersOnlyDispatchWarnings(t *testing.T) {
	t.Parallel()

	h := newTestToolHandler()
	testDir := makeProjectTempDir(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.testGenHandler.HandleGenerateTestHeal(req, json.RawMessage(fmt.Sprintf(`{"what":"test_heal","action":"batch","test_dir":%q,"typo_field":true}`, testDir)))
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("handleGenerateTestHeal should succeed, got error: %s", firstText(result))
	}

	warningsText, ok := warningsBlock(result)
	if !ok {
		t.Fatal("expected warnings block for typo_field")
	}
	if !strings.Contains(warningsText, "typo_field") {
		t.Fatalf("expected warning to include typo_field, got %q", warningsText)
	}
	if strings.Contains(warningsText, "what") {
		t.Fatalf("warning should not include dispatch param 'what', got %q", warningsText)
	}
}

func TestHandleGenerateTestClassify_FiltersOnlyDispatchWarnings(t *testing.T) {
	t.Parallel()

	h := newTestToolHandler()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.testGenHandler.HandleGenerateTestClassify(req, json.RawMessage(`{"what":"test_classify","action":"failure","failure":{"test_name":"login test","error":"Timeout waiting for selector \"#login-btn\""},"typo_field":true}`))
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("handleGenerateTestClassify should succeed, got error: %s", firstText(result))
	}

	warningsText, ok := warningsBlock(result)
	if !ok {
		t.Fatal("expected warnings block for typo_field")
	}
	if !strings.Contains(warningsText, "typo_field") {
		t.Fatalf("expected warning to include typo_field, got %q", warningsText)
	}
	if strings.Contains(warningsText, "what") {
		t.Fatalf("warning should not include dispatch param 'what', got %q", warningsText)
	}
}

func makeProjectTempDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(".tmp-test-heal", strings.ReplaceAll(t.Name(), "/", "_"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create project temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	return dir
}

func warningsBlock(result mcp.MCPToolResult) (string, bool) {
	for _, block := range result.Content {
		if strings.HasPrefix(block.Text, "_warnings:") {
			return block.Text, true
		}
	}
	return "", false
}
