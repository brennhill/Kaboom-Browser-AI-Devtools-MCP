// ws_cursor_test.go — websocket_events honours after_cursor pagination (kaboom-ziws).
package network

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/composition/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/testsupport"
)

func TestWebSocketEventsPageWithAfterCursor(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	base := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	var events []types.WebSocketEvent
	for i := 1; i <= 5; i++ {
		events = append(events, types.WebSocketEvent{ID: fmt.Sprintf("m%d", i), URL: "wss://example.test/s", Event: "message",
			Direction: "incoming", Timestamp: base.Add(time.Duration(i) * time.Second).Format(time.RFC3339)})
	}
	cap.Telemetry().AddWebSocketEvents(events)
	deps := testsupport.Deps(cap)
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}

	first := testsupport.ExtractMCPJSON(t, GetWSEvents(deps, req, json.RawMessage(`{"limit":2}`)))
	assertIDs(t, first, "m5", "m4")
	meta := first["metadata"].(map[string]any)
	cursor, _ := meta["cursor"].(string)
	if cursor == "" || meta["has_more"] != true {
		t.Fatalf("first page metadata = %v, want cursor and has_more", meta)
	}
	// The shared first-page cursor marks the newest entry (as for logs), so
	// after_cursor walks older from there; later pages continue from the
	// oldest returned entry without overlap.
	second := testsupport.ExtractMCPJSON(t, GetWSEvents(deps, req, json.RawMessage(`{"limit":2,"after_cursor":"`+cursor+`"}`)))
	assertIDs(t, second, "m4", "m3")
	next, _ := second["metadata"].(map[string]any)["cursor"].(string)
	third := testsupport.ExtractMCPJSON(t, GetWSEvents(deps, req, json.RawMessage(`{"limit":2,"after_cursor":"`+next+`"}`)))
	assertIDs(t, third, "m2", "m1")
}

func TestWebSocketEventsRejectMalformedCursor(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	cap.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{{ID: "a", Event: "message", Timestamp: time.Now().Format(time.RFC3339)}})
	result := testsupport.DecodeToolResult(t, GetWSEvents(testsupport.Deps(cap), mcp.JSONRPCRequest{ID: 2}, json.RawMessage(`{"after_cursor":"garbage"}`)))
	if !result.IsError {
		t.Fatalf("malformed cursor must be rejected, got %+v", result)
	}
}

func assertIDs(t *testing.T, payload map[string]any, want ...string) {
	t.Helper()
	entries, _ := payload["entries"].([]any)
	if len(entries) != len(want) {
		t.Fatalf("entries = %v, want ids %v", entries, want)
	}
	for i, id := range want {
		if got := entries[i].(map[string]any)["id"]; got != id {
			t.Fatalf("entry %d id = %v, want %s (entries %v)", i, got, id, entries)
		}
	}
}
