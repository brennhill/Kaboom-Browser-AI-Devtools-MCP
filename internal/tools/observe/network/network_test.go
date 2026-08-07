// Purpose: Tests for the network-stream observe modes and their summary builders.
// Docs: docs/features/feature/observe/index.md

package network

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/core"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/testsupport"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"testing"
	"time"
)

func TestNetworkBodyHandlerFiltersTransformsAndExplainsEmptyResults(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	cap.Telemetry().AddNetworkBodies([]types.NetworkBody{
		{URL: "https://example.test/api/users", Method: "GET", Status: 200, Timestamp: time.Now().Format(time.RFC3339), ResponseBody: `{"data":{"id":7}}`},
		{URL: "https://example.test/api/admin", Method: "POST", Status: 503, Timestamp: time.Now().Format(time.RFC3339), ResponseBody: `{"error":"down"}`},
	})
	deps := testsupport.Deps(cap)
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 7}
	response := testsupport.DecodeToolResult(t, GetNetworkBodies(deps, req, json.RawMessage(`{"url":"users","method":"get","status_min":200,"status_max":299,"body_path":"data.id"}`)))
	if response.IsError || !strings.Contains(response.Content[0].Text, `"response_body":"7"`) {
		t.Fatalf("filtered network bodies = %+v", response)
	}
	response = testsupport.DecodeToolResult(t, GetNetworkBodies(deps, req, json.RawMessage(`{"url":"missing","summary":true}`)))
	if response.IsError || !strings.Contains(response.Content[0].Text, "hint") || !strings.Contains(response.Content[0].Text, "filter") {
		t.Fatalf("empty body summary = %+v", response)
	}
	response = testsupport.DecodeToolResult(t, GetNetworkBodies(deps, req, json.RawMessage(`{"body_path":"data["}`)))
	if !response.IsError {
		t.Fatalf("malformed body filter = %+v", response)
	}
}

func TestNetworkBodyHandlerDistinguishesProspectiveCaptureFromAvailableBodies(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	cap.Telemetry().NetworkWaterfall().Add([]types.NetworkWaterfallEntry{{
		URL: "https://example.test/api/users", Timestamp: time.Now(),
	}}, "https://example.test")
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 71}
	deps := testsupport.Deps(cap)

	empty := testsupport.DecodeToolResult(t, GetNetworkBodies(deps, req, nil))
	if empty.IsError || !strings.Contains(empty.Content[0].Text, "waterfall") || !strings.Contains(empty.Content[0].Text, "after") {
		t.Fatalf("prospective capture hint = %+v", empty)
	}

	cap.Telemetry().AddNetworkBodies([]types.NetworkBody{{
		URL: "https://example.test/api/users", Method: "GET", Status: 200,
		Timestamp: time.Now().Format(time.RFC3339), ResponseBody: `{"users":[]}`,
	}})
	nonEmpty := testsupport.DecodeToolResult(t, GetNetworkBodies(deps, req, nil))
	if nonEmpty.IsError || strings.Contains(nonEmpty.Content[0].Text, `"hint"`) {
		t.Fatalf("non-empty body result = %+v", nonEmpty)
	}
	data := testsupport.ExtractMCPJSON(t, GetNetworkBodies(deps, req, nil))
	metadata := data["metadata"].(map[string]any)
	if age, ok := metadata["data_age_ms"].(float64); !ok || age < 0 {
		t.Fatalf("network body data_age_ms = %#v", metadata["data_age_ms"])
	}
}

func TestWaterfallEntryToMapDistinguishesUnavailableRichFields(t *testing.T) {
	plain := waterfallEntryToMap(types.NetworkWaterfallEntry{URL: "https://app.test/api"})
	if _, exists := plain["ttfb_ms"]; exists {
		t.Fatal("unavailable TTFB must be omitted, not reported as zero")
	}
	if _, exists := plain["status"]; exists {
		t.Fatal("unavailable status must be omitted, not reported as zero")
	}

	rich := waterfallEntryToMap(types.NetworkWaterfallEntry{
		URL: "https://app.test/api", TTFBMs: 80, Status: 200, ContentEncoding: "br",
		DuplicateGroupID: "dup-1", DuplicateCount: 2,
	})
	if rich["ttfb_ms"] != float64(80) || rich["status"] != 200 || rich["content_encoding"] != "br" || rich["duplicate_count"] != 2 {
		t.Fatalf("rich fields missing: %#v", rich)
	}
}

func TestWebSocketHandlerFiltersAndSummarizesTraffic(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	cap.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "one", URL: "wss://example.test/socket", Direction: "incoming", Event: "message", Timestamp: time.Now().Format(time.RFC3339)},
		{ID: "two", URL: "wss://other.test/socket", Direction: "outgoing", Event: "message", Timestamp: time.Now().Format(time.RFC3339)},
	})
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 8}
	deps := testsupport.Deps(cap)
	result := testsupport.DecodeToolResult(t, GetWSEvents(deps, req, json.RawMessage(`{"url":"example","connection_id":"one","direction":"incoming"}`)))
	if result.IsError || !strings.Contains(result.Content[0].Text, `"count":1`) {
		t.Fatalf("filtered websocket events = %+v", result)
	}
	result = testsupport.DecodeToolResult(t, GetWSEvents(deps, req, json.RawMessage(`{"url":"missing","direction":"sideways","summary":true}`)))
	if result.IsError || !strings.Contains(result.Content[0].Text, "param_hint") || !strings.Contains(result.Content[0].Text, "hint") {
		t.Fatalf("websocket summary = %+v", result)
	}
}

func TestWaterfallAndWebSocketStatusHandlersExposeOperationalShapes(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	cap.Telemetry().NetworkWaterfall().Add([]types.NetworkWaterfallEntry{
		{URL: "https://example.test/app.js", InitiatorType: "script", Duration: 12, Timestamp: time.Now()},
		{URL: "https://other.test/image.png", InitiatorType: "img", Duration: 5, Timestamp: time.Now()},
	}, "https://example.test")
	cap.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "active", URL: "wss://example.test/socket", Event: "open", Timestamp: time.Now().Format(time.RFC3339)},
		{ID: "closed", URL: "wss://other.test/socket", Event: "open", Timestamp: time.Now().Add(-time.Second).Format(time.RFC3339)},
		{ID: "closed", URL: "wss://other.test/socket", Event: "close", Timestamp: time.Now().Format(time.RFC3339)},
	})
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 9}
	deps := testsupport.Deps(cap)
	for _, args := range []json.RawMessage{json.RawMessage(`{"url":"example"}`), json.RawMessage(`{"url":"example","summary":true}`)} {
		result := testsupport.DecodeToolResult(t, GetNetworkWaterfall(deps, req, args))
		if result.IsError || !strings.Contains(result.Content[0].Text, `"count":1`) {
			t.Fatalf("waterfall result = %+v", result)
		}
	}
	if result := testsupport.DecodeToolResult(t, GetWSStatus(deps, req, json.RawMessage(`not-json`))); !result.IsError {
		t.Fatal("websocket status accepted invalid JSON")
	}
	result := testsupport.DecodeToolResult(t, GetWSStatus(deps, req, json.RawMessage(`{"summary":true}`)))
	if result.IsError || !strings.Contains(result.Content[0].Text, "active_connection_ids") || !strings.Contains(result.Content[0].Text, "closed_connection_ids") ||
		!strings.Contains(result.Content[0].Text, `"active_urls":["wss://example.test/socket"]`) ||
		!strings.Contains(result.Content[0].Text, `"closed_urls":["wss://other.test/socket"]`) ||
		strings.Contains(result.Content[0].Text, `"connections"`) {
		t.Fatalf("websocket status summary = %+v", result)
	}
	empty := capture.NewCapture()
	t.Cleanup(empty.Close)
	result = testsupport.DecodeToolResult(t, GetWSStatus(testsupport.Deps(empty), req, nil))
	if result.IsError || !strings.Contains(result.Content[0].Text, "hint") {
		t.Fatalf("empty websocket status = %+v", result)
	}
}

func TestRefreshWaterfallFreshnessUsesInjectedClock(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	cap.Telemetry().NetworkWaterfall().Add([]types.NetworkWaterfallEntry{{URL: "https://example.test/app.js"}}, "https://example.test")
	addedAt := cap.Telemetry().NetworkWaterfall().Entries()[0].Timestamp

	deps := testsupport.Deps(cap)
	deps.Now = func() time.Time { return addedAt.Add(time.Second) }
	deps.WaterfallRefreshTimeout = time.Nanosecond
	_ = GetNetworkWaterfall(deps, mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 10}, nil)

	if depth := cap.Queries().QueueDepth(); depth != 1 {
		t.Fatalf("queue depth = %d, want one waterfall refresh at exact staleness threshold", depth)
	}
}

func TestWaterfallSummaryEntry_CompactFields(t *testing.T) {
	t.Parallel()
	entry := types.NetworkWaterfallEntry{
		URL:             "https://example.com/api/data",
		InitiatorType:   "fetch",
		Duration:        123.45,
		StartTime:       100.0,
		TransferSize:    5000,
		DecodedBodySize: 10000,
		EncodedBodySize: 5000,
		Timestamp:       time.Now(),
		PageURL:         "https://example.com",
	}

	result := waterfallSummaryEntry(entry)

	// Should have exactly 3 fields: url, ms, type
	if len(result) != 3 {
		t.Errorf("expected 3 fields, got %d: %v", len(result), result)
	}
	if result["url"] != "https://example.com/api/data" {
		t.Errorf("url = %v, want https://example.com/api/data", result["url"])
	}
	if result["ms"] != 123.45 {
		t.Errorf("ms = %v, want 123.45", result["ms"])
	}
	if result["type"] != "fetch" {
		t.Errorf("type = %v, want fetch", result["type"])
	}
}

func TestWaterfallSummaryEntry_URLTruncation(t *testing.T) {
	t.Parallel()
	longURL := "https://example.com/" + string(make([]byte, 100)) // > 80 chars
	for i := range longURL {
		if i >= 20 && longURL[i] == 0 {
			// Fill with 'a' chars after the prefix
		}
	}
	// Build a URL that's definitely > 80 chars
	longURL = "https://example.com/api/v1/very/long/path/that/exceeds/eighty/characters/limit/and/keeps/going/further"

	entry := types.NetworkWaterfallEntry{
		URL:           longURL,
		InitiatorType: "xmlhttprequest",
		Duration:      50.0,
	}

	result := waterfallSummaryEntry(entry)

	url := result["url"].(string)
	if len(url) > 83 { // 80 + "..."
		t.Errorf("URL should be truncated, len=%d: %s", len(url), url)
	}
	if url[len(url)-3:] != "..." {
		t.Errorf("truncated URL should end with ..., got: %s", url)
	}
}

func TestWaterfallSummaryEntry_ShortURL(t *testing.T) {
	t.Parallel()
	entry := types.NetworkWaterfallEntry{
		URL:           "https://a.co/x",
		InitiatorType: "script",
		Duration:      10.0,
	}

	result := waterfallSummaryEntry(entry)
	if result["url"] != "https://a.co/x" {
		t.Errorf("short URL should not be truncated: %v", result["url"])
	}
}

func TestFilterWaterfallSummaryEntries(t *testing.T) {
	t.Parallel()
	entries := []types.NetworkWaterfallEntry{
		{URL: "https://example.com/a", InitiatorType: "fetch", Duration: 10.0},
		{URL: "https://example.com/b", InitiatorType: "script", Duration: 20.0},
		{URL: "https://other.com/c", InitiatorType: "img", Duration: 30.0},
	}

	result := filterWaterfallSummaryEntries(entries, "", 10)
	if len(result) != 3 {
		t.Errorf("expected 3 entries, got %d", len(result))
	}
	// Each entry should have exactly 3 fields
	for i, entry := range result {
		if len(entry) != 3 {
			t.Errorf("entry %d: expected 3 fields, got %d", i, len(entry))
		}
	}
}

func TestFilterWaterfallSummaryEntries_WithFilter(t *testing.T) {
	t.Parallel()
	entries := []types.NetworkWaterfallEntry{
		{URL: "https://example.com/a", InitiatorType: "fetch", Duration: 10.0},
		{URL: "https://other.com/b", InitiatorType: "script", Duration: 20.0},
	}

	result := filterWaterfallSummaryEntries(entries, "example", 10)
	if len(result) != 1 {
		t.Errorf("expected 1 filtered entry, got %d", len(result))
	}
}

func TestFilterWaterfallSummaryEntries_WithLimit(t *testing.T) {
	t.Parallel()
	entries := []types.NetworkWaterfallEntry{
		{URL: "https://example.com/a", InitiatorType: "fetch", Duration: 10.0},
		{URL: "https://example.com/b", InitiatorType: "script", Duration: 20.0},
		{URL: "https://example.com/c", InitiatorType: "img", Duration: 30.0},
	}

	result := filterWaterfallSummaryEntries(entries, "", 2)
	if len(result) != 2 {
		t.Errorf("expected 2 entries with limit, got %d", len(result))
	}
}

// ============================================
// Timeline Summary Tests
// ============================================

func TestBuildNetworkBodiesSummary_StatusGrouping(t *testing.T) {
	t.Parallel()
	bodies := []types.NetworkBody{
		{URL: "http://a.com/api", Method: "GET", Status: 200},
		{URL: "http://a.com/api2", Method: "GET", Status: 201},
		{URL: "http://a.com/api3", Method: "POST", Status: 404},
		{URL: "http://a.com/api4", Method: "GET", Status: 500},
	}
	result := buildNetworkBodiesSummary(bodies, core.ResponseMetadata{})

	byStatus, ok := result["by_status_group"].(map[string]int)
	if !ok {
		t.Fatal("by_status_group not a map[string]int")
	}
	if byStatus["2xx"] != 2 {
		t.Errorf("2xx count = %d, want 2", byStatus["2xx"])
	}
	if byStatus["4xx"] != 1 {
		t.Errorf("4xx count = %d, want 1", byStatus["4xx"])
	}
	if byStatus["5xx"] != 1 {
		t.Errorf("5xx count = %d, want 1", byStatus["5xx"])
	}
}

func TestBuildNetworkBodiesSummary_RecentURLs(t *testing.T) {
	t.Parallel()
	longURL := "http://example.com/" + string(make([]byte, 100))
	bodies := []types.NetworkBody{
		{URL: longURL, Method: "GET", Status: 200},
		{URL: "http://short.com", Method: "GET", Status: 200},
	}
	result := buildNetworkBodiesSummary(bodies, core.ResponseMetadata{})

	recentURLs, ok := result["recent_urls"].([]string)
	if !ok {
		t.Fatal("recent_urls not a []string")
	}
	if len(recentURLs) != 2 {
		t.Fatalf("recent_urls len = %d, want 2", len(recentURLs))
	}
	// Long URL should be truncated to 80 runes + "..."
	if len([]rune(recentURLs[0])) > 84 {
		t.Errorf("long URL not truncated: rune len=%d", len([]rune(recentURLs[0])))
	}
}

func TestBuildWSEventsSummary_ByDirection(t *testing.T) {
	t.Parallel()
	events := []types.WebSocketEvent{
		{Direction: "incoming", ID: "conn1", Event: "message"},
		{Direction: "incoming", ID: "conn1", Event: "message"},
		{Direction: "outgoing", ID: "conn1", Event: "message"},
	}
	result := buildWSEventsSummary(events, core.ResponseMetadata{})

	byDir, ok := result["by_direction"].(map[string]int)
	if !ok {
		t.Fatal("by_direction not a map[string]int")
	}
	if byDir["incoming"] != 2 {
		t.Errorf("incoming = %d, want 2", byDir["incoming"])
	}
	if byDir["outgoing"] != 1 {
		t.Errorf("outgoing = %d, want 1", byDir["outgoing"])
	}
}

func TestBuildWSEventsSummary_UniqueConnections(t *testing.T) {
	t.Parallel()
	events := []types.WebSocketEvent{
		{Direction: "incoming", ID: "conn1", Event: "message"},
		{Direction: "incoming", ID: "conn2", Event: "message"},
		{Direction: "incoming", ID: "conn1", Event: "message"},
	}
	result := buildWSEventsSummary(events, core.ResponseMetadata{})

	connCount, _ := result["connection_count"].(int)
	if connCount != 2 {
		t.Errorf("connection_count = %d, want 2", connCount)
	}
}
