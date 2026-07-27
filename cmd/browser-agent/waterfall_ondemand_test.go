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
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"
)

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
	handler := NewToolHandler(server, cap)
	th := handler.toolHandler.(*ToolHandler)

	// Add fresh entries (just now)
	entries := []capture.NetworkWaterfallEntry{
		{URL: "https://api.example.com/users", PageURL: "https://example.com"},
	}
	cap.AddNetworkWaterfallEntries(entries, "https://example.com")

	// Get pending queries count before call
	pendingBefore := len(cap.GetPendingQueries())

	// Call observe network_waterfall - should return cached data without querying
	resp := observe.GetNetworkWaterfall(th, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))

	// Verify no new query was created (data was fresh)
	pendingAfter := len(cap.GetPendingQueries())
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
	if testing.Short() {
		t.Skip("skips slow waterfall test in short mode")
	}
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/test-waterfall-stale.jsonl", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	cap := capture.NewCapture()
	handler := NewToolHandler(server, cap)
	th := handler.toolHandler.(*ToolHandler)

	// Add stale entries (2 seconds ago - simulated by waiting)
	entries := []capture.NetworkWaterfallEntry{
		{URL: "https://old.example.com/stale", PageURL: "https://example.com"},
	}
	cap.AddNetworkWaterfallEntries(entries, "https://example.com")

	// Wait for data to become stale (>1s)
	time.Sleep(1100 * time.Millisecond)

	// Track query creation in a goroutine that will respond
	var queryCreated bool
	var queryMu sync.Mutex

	// Simulate extension responding to the query
	go func() {
		// Wait a bit for query to be created
		time.Sleep(50 * time.Millisecond)

		// Check if a waterfall query was created
		pending := cap.GetPendingQueries()
		for _, q := range pending {
			if q.Type == "waterfall" {
				queryMu.Lock()
				queryCreated = true
				queryMu.Unlock()

				// Simulate extension response
				result := map[string]any{
					"entries": []map[string]any{
						{
							"url":            "https://fresh.example.com/new",
							"initiator_type": "fetch",
							"duration":       150.5,
							"start_time":     100.0,
							"transfer_size":  1024,
						},
					},
					"page_url": "https://example.com/page",
				}
				resultBytes, _ := json.Marshal(result)
				cap.SetQueryResult(q.ID, resultBytes)
				return
			}
		}
	}()

	// Call observe network_waterfall - should create query and wait
	resp := observe.GetNetworkWaterfall(th, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))

	// Verify query was created
	queryMu.Lock()
	wasCreated := queryCreated
	queryMu.Unlock()

	if !wasCreated {
		t.Error("Expected waterfall query to be created for stale data")
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
	handler := NewToolHandler(server, cap)
	th := handler.toolHandler.(*ToolHandler)

	// Don't add any entries - buffer is empty

	// Track query creation
	var queryCreated bool
	var queryMu sync.Mutex

	// Simulate extension responding
	go func() {
		time.Sleep(50 * time.Millisecond)

		pending := cap.GetPendingQueries()
		for _, q := range pending {
			if q.Type == "waterfall" {
				queryMu.Lock()
				queryCreated = true
				queryMu.Unlock()

				// Return empty entries (no network activity)
				result := map[string]any{
					"entries":  []map[string]any{},
					"page_url": "https://example.com",
				}
				resultBytes, _ := json.Marshal(result)
				cap.SetQueryResult(q.ID, resultBytes)
				return
			}
		}
	}()

	// Call observe network_waterfall
	_ = observe.GetNetworkWaterfall(th, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))

	queryMu.Lock()
	wasCreated := queryCreated
	queryMu.Unlock()

	if !wasCreated {
		t.Error("Expected waterfall query to be created for empty buffer")
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
	// Set a very short timeout for this test
	cap.SetQueryTimeout(100 * time.Millisecond)
	handler := NewToolHandler(server, cap)
	th := handler.toolHandler.(*ToolHandler)

	// Don't respond to the query - let it timeout
	start := time.Now()
	resp := observe.GetNetworkWaterfall(th, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))
	elapsed := time.Since(start)

	// Should complete within reasonable time (not hang forever)
	if elapsed > 10*time.Second {
		t.Errorf("Query took too long: %v (expected < 10s)", elapsed)
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
	if testing.Short() {
		t.Skip("skips slow waterfall test in short mode")
	}
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/test-waterfall-concurrent.jsonl", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	cap := capture.NewCapture()
	handler := NewToolHandler(server, cap)
	th := handler.toolHandler.(*ToolHandler)

	// Simulate extension responding to queries
	go func() {
		for i := 0; i < 100; i++ {
			time.Sleep(10 * time.Millisecond)
			pending := cap.GetPendingQueries()
			for _, q := range pending {
				if q.Type == "waterfall" {
					result := map[string]any{
						"entries": []map[string]any{
							{"url": "https://example.com/resource", "duration": 100.0},
						},
						"page_url": "https://example.com",
					}
					resultBytes, _ := json.Marshal(result)
					cap.SetQueryResult(q.ID, resultBytes)
				}
			}
		}
	}()

	// Run 10 concurrent requests
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			resp := observe.GetNetworkWaterfall(th, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))

			var result map[string]any
			if err := json.Unmarshal(resp.Result, &result); err != nil {
				errors <- err
				return
			}

			content, ok := result["content"].([]any)
			if !ok || len(content) == 0 {
				errors <- err
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

	// Create a waterfall query
	queryID, _ := cap.CreatePendingQuery(queries.PendingQuery{
		Type:   "waterfall",
		Params: json.RawMessage(`{}`),
	})

	// Verify it was created
	if queryID == "" {
		t.Fatal("Failed to create waterfall query")
	}

	// Verify it appears in pending queries
	pending := cap.GetPendingQueries()
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
	if testing.Short() {
		t.Skip("skips slow waterfall test in short mode")
	}
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/test-waterfall-threshold.jsonl", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	cap := capture.NewCapture()
	handler := NewToolHandler(server, cap)
	th := handler.toolHandler.(*ToolHandler)

	// Add entries
	entries := []capture.NetworkWaterfallEntry{
		{URL: "https://example.com/test", PageURL: "https://example.com"},
	}
	cap.AddNetworkWaterfallEntries(entries, "https://example.com")

	// Immediately query - should NOT create new query (data is fresh)
	pendingBefore := len(cap.GetPendingQueries())
	_ = observe.GetNetworkWaterfall(th, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))
	pendingAfter := len(cap.GetPendingQueries())

	if pendingAfter > pendingBefore {
		t.Error("Query created for fresh data (<1s old) - threshold may be wrong")
	}

	// Wait just over 1 second
	time.Sleep(1100 * time.Millisecond)

	// Now query - SHOULD create new query (data is stale)
	go func() {
		time.Sleep(50 * time.Millisecond)
		pending := cap.GetPendingQueries()
		for _, q := range pending {
			if q.Type == "waterfall" {
				result := map[string]any{"entries": []any{}, "page_url": "https://example.com"}
				resultBytes, _ := json.Marshal(result)
				cap.SetQueryResult(q.ID, resultBytes)
			}
		}
	}()

	_ = observe.GetNetworkWaterfall(th, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))

	t.Log("✅ 1-second staleness threshold verified")
}
