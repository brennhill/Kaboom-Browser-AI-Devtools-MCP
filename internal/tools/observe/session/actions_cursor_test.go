// actions_cursor_test.go — actions honours after_cursor pagination (kaboom-ziws).
package session

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/composition/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/testsupport"
)

func TestActionsPageWithAfterCursor(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	base := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC).UnixMilli()
	var actions []types.EnhancedAction
	for i := int64(1); i <= 4; i++ {
		actions = append(actions, types.EnhancedAction{Type: "click", URL: "https://example.test/" + string(rune('a'+i-1)), Timestamp: base + i*1000})
	}
	cap.Telemetry().AddEnhancedActions(actions)
	deps := testsupport.Deps(cap)
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}

	first := testsupport.ExtractMCPJSON(t, GetEnhancedActions(deps, req, json.RawMessage(`{"limit":2}`)))
	assertURLs(t, first, "https://example.test/d", "https://example.test/c")
	cursor, _ := first["metadata"].(map[string]any)["cursor"].(string)
	if cursor == "" {
		t.Fatalf("first page has no cursor: %v", first["metadata"])
	}
	second := testsupport.ExtractMCPJSON(t, GetEnhancedActions(deps, req, json.RawMessage(`{"limit":2,"after_cursor":"`+cursor+`"}`)))
	assertURLs(t, second, "https://example.test/c", "https://example.test/b")
}

func assertURLs(t *testing.T, payload map[string]any, want ...string) {
	t.Helper()
	entries, _ := payload["entries"].([]any)
	if len(entries) != len(want) {
		t.Fatalf("entries = %v, want %v", entries, want)
	}
	for i, url := range want {
		if got := entries[i].(map[string]any)["url"]; got != url {
			t.Fatalf("entry %d url = %v, want %s", i, got, url)
		}
	}
}
