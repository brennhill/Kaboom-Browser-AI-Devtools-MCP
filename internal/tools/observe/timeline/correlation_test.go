// Purpose: Tests for the cross-stream observe modes (error bundles, timeline) and their summaries.
// Docs: docs/features/feature/observe/index.md

package timeline

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/core"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/testsupport"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestBuildTimelineSummary_CountsByType(t *testing.T) {
	t.Parallel()
	entries := []timelineEntry{
		{Timestamp: "2024-01-01T00:00:01Z", Type: "action", Summary: "click"},
		{Timestamp: "2024-01-01T00:00:02Z", Type: "action", Summary: "type"},
		{Timestamp: "2024-01-01T00:00:03Z", Type: "error", Summary: "ReferenceError"},
		{Timestamp: "2024-01-01T00:00:04Z", Type: "network", Summary: "GET /api"},
		{Timestamp: "2024-01-01T00:00:05Z", Type: "network", Summary: "POST /api"},
		{Timestamp: "2024-01-01T00:00:06Z", Type: "network", Summary: "GET /img"},
		{Timestamp: "2024-01-01T00:00:07Z", Type: "websocket", Summary: "message"},
	}

	result := buildTimelineSummary(entries)

	counts, ok := result["counts_by_type"].(map[string]int)
	if !ok {
		t.Fatalf("counts_by_type wrong type: %T", result["counts_by_type"])
	}
	if counts["action"] != 2 {
		t.Errorf("action count = %d, want 2", counts["action"])
	}
	if counts["error"] != 1 {
		t.Errorf("error count = %d, want 1", counts["error"])
	}
	if counts["network"] != 3 {
		t.Errorf("network count = %d, want 3", counts["network"])
	}
	if counts["websocket"] != 1 {
		t.Errorf("websocket count = %d, want 1", counts["websocket"])
	}

	if result["total"] != 7 {
		t.Errorf("total = %v, want 7", result["total"])
	}

	timeRange, ok := result["time_range"].(map[string]string)
	if !ok {
		t.Fatalf("time_range wrong type: %T", result["time_range"])
	}
	if timeRange["first"] != "2024-01-01T00:00:01Z" {
		t.Errorf("first = %v, want 2024-01-01T00:00:01Z", timeRange["first"])
	}
	if timeRange["last"] != "2024-01-01T00:00:07Z" {
		t.Errorf("last = %v, want 2024-01-01T00:00:07Z", timeRange["last"])
	}
}

func TestBuildTimelineSummary_Empty(t *testing.T) {
	t.Parallel()
	result := buildTimelineSummary(nil)
	if result["total"] != 0 {
		t.Errorf("total = %v, want 0", result["total"])
	}
}

// ============================================
// History Limit Tests
// ============================================

func TestBuildErrorBundlesSummary_Counts(t *testing.T) {
	t.Parallel()
	bundles := []map[string]any{
		{"error": map[string]any{"message": "err1"}},
		{"error": map[string]any{"message": "err2"}},
		{"error": map[string]any{"message": "err1"}},
	}
	meta := core.ResponseMetadata{RetrievedAt: "2024-01-01T00:00:00Z"}
	result := buildErrorBundlesSummary(bundles, time.Now(), meta)

	total, _ := result["total_bundles"].(int)
	if total != 3 {
		t.Errorf("total_bundles = %d, want 3", total)
	}

	messages, ok := result["unique_error_messages"].([]string)
	if !ok {
		t.Fatal("unique_error_messages not a []string")
	}
	if len(messages) != 2 {
		t.Errorf("unique_error_messages len = %d, want 2", len(messages))
	}

	// Verify metadata is included
	if _, ok := result["metadata"]; !ok {
		t.Error("expected metadata key in error bundles summary")
	}
}

func TestCorrelationWindowJoinsOnlyEntriesInsideWindow(t *testing.T) {
	t.Parallel()
	end := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	start := end.Add(-3 * time.Second)
	inside := end.Add(-time.Second)
	bodies := []types.NetworkBody{
		{Timestamp: inside.Format(time.RFC3339Nano), Method: "GET", URL: "/inside", Status: 200},
		{Timestamp: start.Format(time.RFC3339Nano), URL: "/boundary"},
		{Timestamp: "bad", URL: "/bad"},
	}
	waterfall := []types.NetworkWaterfallEntry{
		{Timestamp: inside, URL: "/inside", PageURL: "https://example.test"},
		{Timestamp: end.Add(time.Second), URL: "/late"},
	}
	actions := []types.EnhancedAction{
		{Timestamp: inside.UnixMilli(), Type: "click", URL: "/inside", Value: "v", Selectors: map[string]any{"css": "#go"}},
		{Timestamp: start.UnixMilli(), Type: "click"},
	}
	logs := []timedEntry{
		{ts: inside, data: map[string]any{"level": "info", "message": "inside", "timestamp": inside.Format(time.RFC3339)}},
		{ts: end.Add(time.Second), data: map[string]any{"message": "late"}},
	}
	if got := matchNetworkBodies(bodies, start, end); len(got) != 1 || got[0]["url"] != "/inside" {
		t.Fatalf("network matches = %#v", got)
	}
	if got := matchWaterfall(waterfall, start, end); len(got) != 1 || got[0]["url"] != "/inside" {
		t.Fatalf("waterfall matches = %#v", got)
	}
	if got := matchActions(actions, start, end); len(got) != 1 || got[0]["selector"] != "#go" || got[0]["value"] != "v" {
		t.Fatalf("action matches = %#v", got)
	}
	if got := matchLogs(logs, start, end); len(got) != 1 || got[0]["message"] != "inside" {
		t.Fatalf("log matches = %#v", got)
	}

	errors := []timedEntry{{ts: end, data: map[string]any{"message": "boom", "timestamp": end.Format(time.RFC3339)}}}
	bundles := buildBundles(errors, bundleContext{
		networkBodies: bodies, waterfallEntries: waterfall, actions: actions, logs: logs, windowSeconds: 3,
	})
	if len(bundles) != 1 || bundles[0]["context_window_seconds"] != 3 {
		t.Fatalf("bundles = %#v", bundles)
	}
}

func TestCorrelationTabFiltersAndTimelineIncludes(t *testing.T) {
	t.Parallel()
	bodies := []types.NetworkBody{{TabID: 1}, {TabID: 2}}
	if got := filterNetworkBodiesByTab(bodies, 2); len(got) != 1 || got[0].TabID != 2 {
		t.Fatalf("network tab filter = %#v", got)
	}
	actions := []types.EnhancedAction{{TabID: 1}, {TabID: 2}}
	if got := filterActionsByTab(actions, 1); len(got) != 1 || got[0].TabID != 1 {
		t.Fatalf("action tab filter = %#v", got)
	}

	cap := capture.NewCapture()
	entries := []types.NetworkWaterfallEntry{{PageURL: "https://example.test/page"}, {PageURL: "https://other.test"}, {}}
	if got := filterWaterfallByTab(entries, 7, cap); len(got) != len(entries) {
		t.Fatalf("untracked waterfall = %#v", got)
	}
	cap.Extension().UpdateTrackedTab(7, "https://example.test", "Example")
	got := filterWaterfallByTab(entries, 7, cap)
	if len(got) != 2 || got[0].PageURL != "https://example.test/page" {
		t.Fatalf("tracked waterfall = %#v", got)
	}

	all := parseTimelineIncludes(nil)
	if !all.actions || !all.errors || !all.network || !all.ws {
		t.Fatalf("default includes = %+v", all)
	}
	selected := parseTimelineIncludes([]string{"actions", "errors", "network", "websocket", "unknown"})
	if !selected.actions || !selected.errors || !selected.network || !selected.ws {
		t.Fatalf("selected includes = %+v", selected)
	}
}

func TestErrorEntryAndTimestampFallback(t *testing.T) {
	t.Parallel()
	ts := "2026-01-02T03:04:05Z"
	entry := map[string]any{"message": "boom", "source": "app", "ts": ts}
	if got := parseEntryTimestamp(entry); got.IsZero() {
		t.Fatal("ts fallback was not parsed")
	}
	mapped := errorEntryToMap(entry)
	if mapped["message"] != "boom" || mapped["source"] != "app" {
		t.Fatalf("error map = %#v", mapped)
	}
}

func TestErrorBundlesHandlerExplainsEmptyStateAndAcceptsExtensionTimestamp(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	deps := testsupport.Deps(cap)
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 21}

	empty := testsupport.ExtractMCPJSON(t, GetErrorBundles(deps, req, nil))
	if bundles, ok := empty["bundles"].([]any); !ok || len(bundles) != 0 || empty["hint"] == nil {
		t.Fatalf("empty bundles = %#v", empty)
	}
	timeline := testsupport.ExtractMCPJSON(t, GetSessionTimeline(deps, req, nil))
	if entries, ok := timeline["entries"].([]any); !ok || len(entries) != 0 {
		t.Fatalf("empty timeline = %#v", timeline)
	}

	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	deps.LogEntries = func() ([]types.LogEntry, []time.Time) {
		return []types.LogEntry{{
			"type": "console", "level": "error", "message": "extension failure",
			"source": "app.js", "ts": now.Format(time.RFC3339),
		}}, nil
	}
	result := testsupport.ExtractMCPJSON(t, GetErrorBundles(deps, req, nil))
	bundles, ok := result["bundles"].([]any)
	if !ok || len(bundles) != 1 || !strings.Contains(resultText(t, bundles[0]), "extension failure") {
		t.Fatalf("extension timestamp bundles = %#v", result)
	}
}

func TestErrorBundlesHandlerAppliesLimitAndCustomContextWindow(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	cap.Telemetry().AddNetworkBodies([]types.NetworkBody{{
		URL: "/within-five-seconds", Method: "GET", Status: 500,
		Timestamp: now.Add(-4 * time.Second).Format(time.RFC3339),
	}})
	deps := testsupport.Deps(cap)
	deps.LogEntries = func() ([]types.LogEntry, []time.Time) {
		return []types.LogEntry{
			{"type": "console", "level": "error", "message": "older", "timestamp": now.Add(-time.Second).Format(time.RFC3339)},
			{"type": "console", "level": "error", "message": "newer", "timestamp": now.Format(time.RFC3339)},
		}, nil
	}
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 22}
	result := testsupport.ExtractMCPJSON(t, GetErrorBundles(deps, req, json.RawMessage(`{"limit":1,"window_seconds":5}`)))
	bundles, ok := result["bundles"].([]any)
	if !ok || len(bundles) != 1 {
		t.Fatalf("limited bundles = %#v", result)
	}
	bundle, ok := bundles[0].(map[string]any)
	if !ok || bundle["context_window_seconds"] != float64(5) {
		t.Fatalf("custom-window bundle = %#v", bundles[0])
	}
	network, ok := bundle["network"].([]any)
	if !ok || len(network) != 1 {
		t.Fatalf("network context = %#v", bundle["network"])
	}
}

func resultText(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	return string(encoded)
}
