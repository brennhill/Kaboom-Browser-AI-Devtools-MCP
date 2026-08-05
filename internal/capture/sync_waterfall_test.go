// sync_waterfall_test.go — Tests waterfall query and result delivery over sync.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// ============================================
// Waterfall On-Demand Tests via Sync
// ============================================

// TestHandleSync_WaterfallQueryDelivery verifies that waterfall queries
// are delivered to extension via sync response commands.
func TestHandleSync_WaterfallQueryDelivery(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// Create a waterfall query (simulating MCP requesting fresh data)
	queryID, _ := cap.Queries().CreatePendingQuery(queries.PendingQuery{
		Type:   "waterfall",
		Params: json.RawMessage(`{}`),
	})
	if queryID == "" {
		t.Fatal("Failed to create waterfall query")
	}

	// Extension polls /sync and receives the command
	req := syncruntime.SyncRequest{ExtSessionID: "test_session"}
	w := runSyncRequest(t, cap, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	// Parse response and verify waterfall command is present
	resp := decodeSyncResponse(t, w)

	if len(resp.Commands) == 0 {
		t.Fatal("Expected at least one command in sync response")
	}

	found := false
	for _, cmd := range resp.Commands {
		if cmd.Type == "waterfall" && cmd.ID == queryID {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Waterfall command not found in sync response. Commands: %v", resp.Commands)
	}
}

// TestHandleSync_WaterfallResultDelivery verifies that waterfall results
// are stored correctly when extension posts them via sync.
func TestHandleSync_WaterfallResultDelivery(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// Create a waterfall query
	queryID, _ := cap.Queries().CreatePendingQuery(queries.PendingQuery{
		Type:   "waterfall",
		Params: json.RawMessage(`{}`),
	})

	// Simulate extension returning waterfall data via sync
	waterfallResult := map[string]any{
		"entries": []map[string]any{
			{"url": "https://api.example.com/users", "duration": 150.5},
		},
		"page_url": "https://example.com",
	}
	resultBytes, _ := json.Marshal(waterfallResult)

	req := syncruntime.SyncRequest{
		ExtSessionID: "test_session",
		CommandResults: []syncruntime.SyncCommandResult{
			{
				ID:     queryID,
				Status: "complete",
				Result: resultBytes,
			},
		},
	}

	w := runSyncRequest(t, cap, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	// Verify result was stored
	result, found := cap.Queries().TakeQueryResult(queryID)
	if !found {
		t.Fatal("Expected query result to be stored")
	}

	// Verify result content
	var storedResult map[string]any
	if err := json.Unmarshal(result, &storedResult); err != nil {
		t.Fatalf("Failed to unmarshal stored result: %v", err)
	}

	entries, ok := storedResult["entries"].([]any)
	if !ok || len(entries) != 1 {
		t.Errorf("Expected 1 entry in result, got: %v", storedResult)
	}
}
