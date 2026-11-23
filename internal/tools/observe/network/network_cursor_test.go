// network_cursor_test.go — network_bodies and network_waterfall honour after_cursor pagination (kaboom-ziws).
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

func TestNetworkBodiesPageWithAfterCursor(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	base := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	var bodies []types.NetworkBody
	for i := 1; i <= 5; i++ {
		bodies = append(bodies, types.NetworkBody{URL: fmt.Sprintf("https://api.test/%d", i), Method: "GET", Status: 200,
			Timestamp: base.Add(time.Duration(i) * time.Second).Format(time.RFC3339)})
	}
	cap.Telemetry().AddNetworkBodies(bodies)
	deps := testsupport.Deps(cap)
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}

	first := testsupport.ExtractMCPJSON(t, GetNetworkBodies(deps, req, json.RawMessage(`{"limit":2}`)))
	assertField(t, first, "url", "https://api.test/5", "https://api.test/4")
	cursor := metaCursor(t, first)
	second := testsupport.ExtractMCPJSON(t, GetNetworkBodies(deps, req, json.RawMessage(`{"limit":2,"after_cursor":"`+cursor+`"}`)))
	assertField(t, second, "url", "https://api.test/4", "https://api.test/3")
	third := testsupport.ExtractMCPJSON(t, GetNetworkBodies(deps, req, json.RawMessage(`{"limit":2,"after_cursor":"`+metaCursor(t, second)+`"}`)))
	assertField(t, third, "url", "https://api.test/2", "https://api.test/1")
}

func TestNetworkWaterfallPageWithAfterCursor(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	var entries []types.NetworkWaterfallEntry
	for i := 1; i <= 4; i++ {
		entries = append(entries, types.NetworkWaterfallEntry{Name: fmt.Sprintf("r%d", i), URL: fmt.Sprintf("https://cdn.test/%d.js", i), StartTime: float64(i)})
	}
	cap.Telemetry().NetworkWaterfall().Add(entries, "https://page.test/")
	deps := testsupport.Deps(cap)
	deps.Now = func() time.Time { return time.Now() } // entries are fresh: no extension refresh
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 2}

	first := testsupport.ExtractMCPJSON(t, GetNetworkWaterfall(deps, req, json.RawMessage(`{"limit":2}`)))
	assertField(t, first, "url", "https://cdn.test/4.js", "https://cdn.test/3.js")
	second := testsupport.ExtractMCPJSON(t, GetNetworkWaterfall(deps, req, json.RawMessage(`{"limit":2,"after_cursor":"`+metaCursor(t, first)+`"}`)))
	assertField(t, second, "url", "https://cdn.test/3.js", "https://cdn.test/2.js")
}

func metaCursor(t *testing.T, payload map[string]any) string {
	t.Helper()
	cursor, _ := payload["metadata"].(map[string]any)["cursor"].(string)
	if cursor == "" {
		t.Fatalf("no cursor in metadata: %v", payload["metadata"])
	}
	return cursor
}

func assertField(t *testing.T, payload map[string]any, field string, want ...string) {
	t.Helper()
	entries, _ := payload["entries"].([]any)
	if len(entries) != len(want) {
		t.Fatalf("entries = %v, want %s %v", entries, field, want)
	}
	for i, value := range want {
		if got := entries[i].(map[string]any)[field]; got != value {
			t.Fatalf("entry %d %s = %v, want %s (entries %v)", i, field, got, value, entries)
		}
	}
}
