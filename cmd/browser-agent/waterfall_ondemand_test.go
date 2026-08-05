// Purpose: Tests for on-demand waterfall data retrieval.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// waterfall_ondemand_test.go — Tests for on-demand network waterfall fetching.
// These tests ensure the on-demand waterfall feature never regresses.
//
// ARCHITECTURAL INVARIANT: When buffer is stale (>1s), toolGetNetworkWaterfall
// MUST create a "waterfall" query and wait for extension response.
package main

import (
	"encoding/json"
	"fmt"
	observenetwork "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/network"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func respondToNextWaterfallQuery(cap *capture.Capture, result any) <-chan error {
	done := make(chan error, 1)
	go func() {
		cap.Queries().WaitForPendingQueries(time.Second)
		for _, query := range cap.Queries().GetPendingQueries() {
			if query.Type != "waterfall" {
				continue
			}
			resultBytes, err := json.Marshal(result)
			if err == nil {
				cap.Queries().SetQueryResult(query.ID, resultBytes)
			}
			done <- err
			return
		}
		done <- fmt.Errorf("waterfall query was not enqueued")
	}()
	return done
}

// ============================================
// On-Demand Waterfall Tests
// ============================================

// TestWaterfallOnDemand_FreshDataNoQuery verifies that fresh data (<1s old)
// is returned immediately without creating a query.
func TestWaterfallOnDemand_FreshDataNoQuery(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/test-waterfall-fresh.jsonl", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	handler := NewToolHandler(server, cap)
	th := handler.tools.Executor.(*ToolHandler)

	// Add fresh entries (just now)
	entries := []types.NetworkWaterfallEntry{
		{URL: "https://api.example.com/users", PageURL: "https://example.com"},
	}
	cap.Telemetry().NetworkWaterfall().Add(entries, "https://example.com")

	// Get pending queries count before call
	pendingBefore := len(cap.Queries().GetPendingQueries())

	// Call observe network_waterfall - should return cached data without querying
	resp := observenetwork.GetNetworkWaterfall(buildObserveReadDeps(th), mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))

	// Verify no new query was created (data was fresh)
	pendingAfter := len(cap.Queries().GetPendingQueries())
	if pendingAfter > pendingBefore {
		t.Errorf("Expected no new queries for fresh data, but query count changed from %d to %d", pendingBefore, pendingAfter)
	}

	// Verify data was returned
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	content := result["content"].([]any)
	textBlock := content[0].(map[string]any)
	var data map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(textBlock["text"].(string))), &data); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	resultEntries := data["entries"].([]any)
	if len(resultEntries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(resultEntries))
	}

	t.Log("✅ Fresh data returned without creating query")
}

// TestWaterfallOnDemand_StaleDataCreatesQuery verifies that stale data (>1s old)
// triggers a waterfall query to the extension.
func TestWaterfallOnDemand_StaleDataCreatesQuery(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/test-waterfall-stale.jsonl", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	handler := NewToolHandler(server, cap)
	th := handler.tools.Executor.(*ToolHandler)

	// Add stale entries (2 seconds ago - simulated by waiting)
	entries := []types.NetworkWaterfallEntry{
		{URL: "https://old.example.com/stale", PageURL: "https://example.com"},
	}
	cap.Telemetry().NetworkWaterfall().Add(entries, "https://example.com")
	addedAt := cap.Telemetry().NetworkWaterfall().Entries()[0].Timestamp
	deps := buildObserveReadDeps(th)
	deps.Now = func() time.Time { return addedAt.Add(time.Second) }
	responded := respondToNextWaterfallQuery(cap, map[string]any{
		"entries": []map[string]any{{
			"url":            "https://fresh.example.com/new",
			"initiator_type": "fetch",
			"duration":       150.5,
			"start_time":     100.0,
			"transfer_size":  1024,
		}},
		"page_url": "https://example.com/page",
	})

	// Call observe network_waterfall - should create query and wait
	resp := observenetwork.GetNetworkWaterfall(deps, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))
	if err := <-responded; err != nil {
		t.Fatal(err)
	}

	// Verify fresh data was returned
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	content := result["content"].([]any)
	textBlock := content[0].(map[string]any)
	var data map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(textBlock["text"].(string))), &data); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	resultEntries := data["entries"].([]any)
	// Should have both old and new entries (buffer accumulates)
	if len(resultEntries) < 1 {
		t.Errorf("Expected at least 1 entry, got %d", len(resultEntries))
	}

	t.Logf("✅ Stale data triggered query, returned %d entries", len(resultEntries))
}

// TestWaterfallOnDemand_EmptyBufferCreatesQuery verifies that an empty buffer
// triggers a waterfall query.
func TestWaterfallOnDemand_EmptyBufferCreatesQuery(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/test-waterfall-empty.jsonl", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	handler := NewToolHandler(server, cap)
	th := handler.tools.Executor.(*ToolHandler)

	// Don't add any entries - buffer is empty

	responded := respondToNextWaterfallQuery(cap, map[string]any{
		"entries":  []map[string]any{},
		"page_url": "https://example.com",
	})

	// Call observe network_waterfall
	_ = observenetwork.GetNetworkWaterfall(buildObserveReadDeps(th), mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))
	if err := <-responded; err != nil {
		t.Fatal(err)
	}

	t.Log("✅ Empty buffer triggered query")
}

// TestWaterfallOnDemand_TimeoutHandling verifies graceful handling when
// extension doesn't respond within timeout.
func TestWaterfallOnDemand_TimeoutHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("skips slow waterfall test in short mode")
	}
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/test-waterfall-timeout.jsonl", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	handler := NewToolHandler(server, cap)
	th := handler.tools.Executor.(*ToolHandler)
	deps := buildObserveReadDeps(th)
	deps.WaterfallRefreshTimeout = 10 * time.Millisecond

	// Don't respond to the query - let it timeout
	start := time.Now()
	resp := observenetwork.GetNetworkWaterfall(deps, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))
	elapsed := time.Since(start)

	// Should complete within reasonable time (not hang forever)
	if elapsed > time.Second {
		t.Errorf("Query took too long: %v (expected < 1s)", elapsed)
	}

	// Should still return a valid response (empty entries)
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	content := result["content"].([]any)
	if len(content) == 0 {
		t.Error("Expected at least one content block")
	}

	t.Logf("✅ Timeout handled gracefully in %v", elapsed)
}

// TestWaterfallOnDemand_ConcurrentRequests verifies that concurrent requests
// don't cause data races or deadlocks.
func TestWaterfallOnDemand_ConcurrentRequests(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/test-waterfall-concurrent.jsonl", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	handler := NewToolHandler(server, cap)
	th := handler.tools.Executor.(*ToolHandler)
	deps := buildObserveReadDeps(th)
	deps.WaterfallRefreshTimeout = time.Nanosecond

	// Run 10 concurrent requests
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			resp := observenetwork.GetNetworkWaterfall(deps, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))

			var result map[string]any
			if err := json.Unmarshal(resp.Result, &result); err != nil {
				errors <- err
				return
			}

			content, ok := result["content"].([]any)
			if !ok || len(content) == 0 {
				errors <- fmt.Errorf("response content is missing: %#v", result["content"])
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		if err != nil {
			t.Errorf("Concurrent request failed: %v", err)
		}
	}

	t.Log("✅ Concurrent requests handled without race conditions")
}

// ============================================
// Architecture Invariant Tests
// ============================================

// TestWaterfallQueryType_ExistsInPendingQueries verifies that "waterfall"
// is a valid query type that can be created and retrieved.
func TestWaterfallQueryType_ExistsInPendingQueries(t *testing.T) {
	t.Parallel()

	cap := capture.NewCapture()
	t.Cleanup(cap.Close)

	// Create a waterfall query
	queryID, _ := cap.Queries().CreatePendingQuery(queries.PendingQuery{
		Type:   "waterfall",
		Params: json.RawMessage(`{}`),
	})

	// Verify it was created
	if queryID == "" {
		t.Fatal("Failed to create waterfall query")
	}

	// Verify it appears in pending queries
	pending := cap.Queries().GetPendingQueries()
	found := false
	for _, q := range pending {
		if q.ID == queryID && q.Type == "waterfall" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Waterfall query not found in pending queries")
	}

	t.Log("✅ Waterfall query type works correctly")
}

// TestWaterfallStalenessThreshold verifies the 1-second staleness threshold.
func TestWaterfallStalenessThreshold(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/test-waterfall-threshold.jsonl", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	handler := NewToolHandler(server, cap)
	th := handler.tools.Executor.(*ToolHandler)

	// Add entries
	entries := []types.NetworkWaterfallEntry{
		{URL: "https://example.com/test", PageURL: "https://example.com"},
	}
	cap.Telemetry().NetworkWaterfall().Add(entries, "https://example.com")
	addedAt := cap.Telemetry().NetworkWaterfall().Entries()[0].Timestamp
	deps := buildObserveReadDeps(th)
	deps.Now = func() time.Time { return addedAt.Add(time.Second - time.Nanosecond) }

	// Immediately query - should NOT create new query (data is fresh)
	pendingBefore := len(cap.Queries().GetPendingQueries())
	_ = observenetwork.GetNetworkWaterfall(deps, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))
	pendingAfter := len(cap.Queries().GetPendingQueries())

	if pendingAfter > pendingBefore {
		t.Error("Query created for fresh data (<1s old) - threshold may be wrong")
	}

	// At exactly one second the entry is stale and must trigger a refresh.
	deps.Now = func() time.Time { return addedAt.Add(time.Second) }
	responded := respondToNextWaterfallQuery(cap, map[string]any{"entries": []any{}, "page_url": "https://example.com"})
	_ = observenetwork.GetNetworkWaterfall(deps, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))
	if err := <-responded; err != nil {
		t.Fatal(err)
	}

	t.Log("✅ 1-second staleness threshold verified")
}
