// Purpose: Regression tests for tool-response post-processing warnings.
// Why: Locks pending-intent warning copy to the Phase 1 audit workflow.

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func makeTextResultResponse(t *testing.T, text string) mcp.JSONRPCResponse {
	t.Helper()
	raw, err := json.Marshal(mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: text}},
		IsError: false,
	})
	if err != nil {
		t.Fatalf("json.Marshal(MCPToolResult): %v", err)
	}
	return mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      1,
		Result:  raw,
	}
}

func TestMaybeAddPendingIntents_UsesAuditWorkflowCopy(t *testing.T) {
	server, err := NewServer(t.TempDir()+"/test.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { server.Close() })

	server.intentStore.Add("https://tracked.example/", terminal.IntentActionQAScan)

	handler := NewMCPHandler(server, "test-version")
	resp := handler.maybeAddPendingIntents(makeTextResultResponse(t, "base response"))
	text := firstText(parseToolResult(t, resp))

	if !strings.Contains(strings.ToLower(text), "audit") {
		t.Fatalf("warning should point to audit workflow, got %q", text)
	}
	if strings.Contains(strings.ToLower(text), "qa skill") {
		t.Fatalf("warning should no longer point to the old QA skill wording, got %q", text)
	}
	if !strings.Contains(text, "/kaboom/audit") && !strings.Contains(text, "/audit") {
		t.Fatalf("warning should mention the audit workflow entrypoint, got %q", text)
	}
}
