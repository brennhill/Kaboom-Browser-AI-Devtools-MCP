// Purpose: Tests for security-mode warning injection in responses.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestToolResponsePostProcessing_AddsSecurityModeMetadataAndWarning(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/security-mode-handler.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer server.Close()

	cap := makeTestCapture(t)
	handler := NewToolHandler(server, cap)
	toolHandler := handler.tools.Executor.(*ToolHandler)

	enableReq := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"configure","arguments":{"what":"security_mode","mode":"insecure_proxy","confirm":true}}`),
	}
	enableResp := handler.HandleRequest(enableReq)
	if enableResp == nil || enableResp.Error != nil {
		t.Fatalf("enable security mode failed: %+v", enableResp)
	}

	resp, handled := toolHandler.HandleToolCall(
		mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2, ClientID: "test-client"},
		"configure",
		json.RawMessage(`{"what":"health"}`),
	)
	if !handled {
		t.Fatal("expected configure tool call to be handled")
	}

	post := handler.applyToolResponsePostProcessing(resp, "test-client", "configure", nil)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(post.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content blocks")
	}
	if !strings.Contains(result.Content[0].Text, "[ALTERED ENVIRONMENT]") {
		t.Fatalf("first content block should include altered environment warning, got: %s", result.Content[0].Text)
	}

	if result.Metadata == nil {
		t.Fatal("expected metadata to be present")
	}
	if got, _ := result.Metadata["security_mode"].(string); got != "insecure_proxy" {
		t.Fatalf("metadata.security_mode = %q, want insecure_proxy", got)
	}
	if got, ok := result.Metadata["production_parity"].(bool); !ok || got {
		t.Fatalf("metadata.production_parity = %v, want false", result.Metadata["production_parity"])
	}
	rewriteList, ok := result.Metadata["insecure_rewrites_applied"].([]any)
	if !ok || len(rewriteList) == 0 {
		t.Fatalf("metadata.insecure_rewrites_applied should be non-empty array, got: %#v", result.Metadata["insecure_rewrites_applied"])
	}
}

func makeTestCapture(t *testing.T) *capture.Capture {
	t.Helper()
	cap := capture.NewCapture()
	capturefixture.SetPilot(cap, true)
	return cap
}
