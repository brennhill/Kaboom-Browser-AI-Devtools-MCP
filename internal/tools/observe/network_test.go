// Purpose: Tests for the network-stream observe modes and their summary builders.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

func TestWaterfallSummaryEntry_CompactFields(t *testing.T) {
	t.Parallel()
	entry := capture.NetworkWaterfallEntry{
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

	entry := capture.NetworkWaterfallEntry{
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
	entry := capture.NetworkWaterfallEntry{
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
	entries := []capture.NetworkWaterfallEntry{
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
	entries := []capture.NetworkWaterfallEntry{
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
	entries := []capture.NetworkWaterfallEntry{
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
	bodies := []capture.NetworkBody{
		{URL: "http://a.com/api", Method: "GET", Status: 200},
		{URL: "http://a.com/api2", Method: "GET", Status: 201},
		{URL: "http://a.com/api3", Method: "POST", Status: 404},
		{URL: "http://a.com/api4", Method: "GET", Status: 500},
	}
	result := buildNetworkBodiesSummary(bodies, ResponseMetadata{})

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
	bodies := []capture.NetworkBody{
		{URL: longURL, Method: "GET", Status: 200},
		{URL: "http://short.com", Method: "GET", Status: 200},
	}
	result := buildNetworkBodiesSummary(bodies, ResponseMetadata{})

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
	events := []capture.WebSocketEvent{
		{Direction: "incoming", ID: "conn1", Event: "message"},
		{Direction: "incoming", ID: "conn1", Event: "message"},
		{Direction: "outgoing", ID: "conn1", Event: "message"},
	}
	result := buildWSEventsSummary(events, ResponseMetadata{})

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
	events := []capture.WebSocketEvent{
		{Direction: "incoming", ID: "conn1", Event: "message"},
		{Direction: "incoming", ID: "conn2", Event: "message"},
		{Direction: "incoming", ID: "conn1", Event: "message"},
	}
	result := buildWSEventsSummary(events, ResponseMetadata{})

	connCount, _ := result["connection_count"].(int)
	if connCount != 2 {
		t.Errorf("connection_count = %d, want 2", connCount)
	}
}
