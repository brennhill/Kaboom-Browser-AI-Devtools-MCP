// Purpose: Unit tests for browser-agent tools observe logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_observe_unit_test.go — Unit tests for observe tool helpers.
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	wiretypes "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestAppendAlertsToResponse(t *testing.T) {
	t.Parallel()

	cap := capture.NewCapture()
	server, err := NewServer(t.TempDir()+"/test-observe-alerts.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	mcpHandler := NewToolHandler(server, cap)
	h := mcpHandler.toolHandler.(*ToolHandler)

	t.Run("appends alert block", func(t *testing.T) {
		// Build a valid MCP response with one content block
		original := MCPToolResult{
			Content: []MCPContentBlock{
				{Type: "text", Text: "original data"},
			},
		}
		resultJSON, _ := json.Marshal(original)
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(resultJSON),
		}

		alerts := []wiretypes.Alert{
			{Severity: "error", Category: "regression", Title: "Something broke"},
		}

		got := h.appendAlertsToResponse(resp, alerts)

		var result MCPToolResult
		if err := json.Unmarshal(got.Result, &result); err != nil {
			t.Fatalf("unmarshal result: %v", err)
		}

		if len(result.Content) != 2 {
			t.Fatalf("expected 2 content blocks, got %d", len(result.Content))
		}
		if result.Content[0].Text != "original data" {
			t.Fatalf("original content modified: %q", result.Content[0].Text)
		}
		if !strings.Contains(result.Content[1].Text, "Something broke") {
			t.Fatalf("alert block should contain alert title, got: %q", result.Content[1].Text)
		}
	})

	t.Run("empty alerts still appends block", func(t *testing.T) {
		original := MCPToolResult{
			Content: []MCPContentBlock{
				{Type: "text", Text: "data"},
			},
		}
		resultJSON, _ := json.Marshal(original)
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(resultJSON),
		}

		got := h.appendAlertsToResponse(resp, []wiretypes.Alert{})

		var result MCPToolResult
		if err := json.Unmarshal(got.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		// Should still have 2 blocks (original + alert summary)
		if len(result.Content) != 2 {
			t.Fatalf("expected 2 content blocks, got %d", len(result.Content))
		}
	})

	t.Run("malformed result returns unchanged", func(t *testing.T) {
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(`"not an object"`),
		}

		got := h.appendAlertsToResponse(resp, []wiretypes.Alert{{Title: "alert"}})
		// Should return the original response unchanged
		if string(got.Result) != `"not an object"` {
			t.Fatalf("malformed result should be returned unchanged, got: %s", got.Result)
		}
	})
}
