// contract_test.go — Shared batch/sequence replay contract tests.
// Docs: docs/features/feature/batch-sequences/index.md

package replay

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestForceAsyncDisablesStepWaiting(t *testing.T) {
	got := ForceAsync(json.RawMessage(`{"what":"click","sync":true,"wait":true}`))
	var fields map[string]any
	if err := json.Unmarshal(got, &fields); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if fields["sync"] != false || fields["wait"] != false {
		t.Fatalf("expected async replay fields, got %#v", fields)
	}
}

func TestCorrelationIDReadsMCPJSONContent(t *testing.T) {
	resp := mcp.Succeed(mcp.JSONRPCRequest{ID: 1}, "queued", map[string]any{"correlation_id": "batch_123"})
	if got := CorrelationID(resp); got != "batch_123" {
		t.Fatalf("CorrelationID = %q", got)
	}
}

func TestErrorMessageHandlesStructuredPlainAndMalformedResults(t *testing.T) {
	t.Parallel()
	request := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}
	structured := mcp.Succeed(request, "failed", map[string]any{"message": "boom"})
	if got := ErrorMessage(structured); got != "boom" {
		t.Fatalf("structured error = %q", got)
	}
	plainResult, err := json.Marshal(mcp.MCPToolResult{Content: []mcp.MCPContentBlock{{Type: "text", Text: "plain failure"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := ErrorMessage(mcp.JSONRPCResponse{Result: plainResult}); got != "plain failure" {
		t.Fatalf("plain error = %q", got)
	}
	for _, result := range []json.RawMessage{nil, json.RawMessage(`{bad`), json.RawMessage(`{"content":[]}`)} {
		if got := ErrorMessage(mcp.JSONRPCResponse{Result: result}); got != "" {
			t.Fatalf("malformed/empty error = %q", got)
		}
	}
}
