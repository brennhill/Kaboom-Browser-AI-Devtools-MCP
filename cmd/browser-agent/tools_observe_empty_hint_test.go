// Purpose: Verify diagnostic hints when network_bodies, websocket_events, and websocket_status return empty.
// Why: Fixes #278 (network_bodies empty despite waterfall data) and #287 (websocket_events empty).
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_observe_empty_hint_test.go — Tests for empty-result diagnostic hints.
// When an observe mode returns 0 entries, the response MUST include a "hint" field
// explaining why the buffer may be empty and suggesting remediation.
package main

import (
	"bytes"
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"
)

// ============================================
// Issue #278: network_bodies empty despite waterfall data
// ============================================

// TestGetNetworkBodies_EmptyWithWaterfallData_ReturnsHint verifies that when
// network_bodies returns 0 entries but the waterfall buffer has entries,
// the response includes a diagnostic hint explaining the prospective-only limitation.
func TestGetNetworkBodies_EmptyWithWaterfallData_ReturnsHint(t *testing.T) {
	t.Parallel()

	env := newToolTestEnv(t)

	// Add waterfall entries (network requests exist)
	waterfallEntries := []types.NetworkWaterfallEntry{
		{
			URL:           "https://api.github.com/repos",
			InitiatorType: "fetch",
			Duration:      120.0,
			StartTime:     500.0,
			TransferSize:  2048,
			PageURL:       "https://github.com/dashboard",
			Timestamp:     time.Now(),
		},
		{
			URL:           "https://api.github.com/notifications",
			InitiatorType: "fetch",
			Duration:      80.0,
			StartTime:     600.0,
			TransferSize:  1024,
			PageURL:       "https://github.com/dashboard",
			Timestamp:     time.Now(),
		},
	}
	env.capture.AddNetworkWaterfallEntries(waterfallEntries, "https://github.com/dashboard")

	// Do NOT add any network bodies — simulates issue #278

	// Call observe network_bodies
	resp := observe.GetNetworkBodies(env.handler, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))

	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	// Verify empty result
	count, _ := data["count"].(float64)
	if count != 0 {
		t.Fatalf("Expected 0 entries, got %v", count)
	}

	// CRITICAL: Must have a hint field explaining why bodies are empty
	hint, ok := data["hint"].(string)
	if !ok {
		t.Fatal("Expected 'hint' field when network_bodies is empty but waterfall has entries")
	}

	// Hint should mention that bodies are captured prospectively
	if !strings.Contains(hint, "after") {
		t.Errorf("Hint should explain bodies are captured only for requests made AFTER tracking. Got: %s", hint)
	}

	// Hint should reference the waterfall count so the user understands data exists
	if !strings.Contains(hint, "waterfall") {
		t.Errorf("Hint should reference the waterfall as a data source. Got: %s", hint)
	}

	t.Logf("hint: %s", hint)
}

// TestGetNetworkBodies_EmptyNoWaterfall_ReturnsHint verifies that when both
// network_bodies and waterfall are empty, a generic hint is still provided.
func TestGetNetworkBodies_EmptyNoWaterfall_ReturnsHint(t *testing.T) {
	t.Parallel()

	env := newToolTestEnv(t)

	// No waterfall entries, no bodies — fresh session

	resp := observe.GetNetworkBodies(env.handler, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))

	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	count, _ := data["count"].(float64)
	if count != 0 {
		t.Fatalf("Expected 0 entries, got %v", count)
	}

	// Should still have a hint
	hint, ok := data["hint"].(string)
	if !ok {
		t.Fatal("Expected 'hint' field when network_bodies is empty")
	}

	if hint == "" {
		t.Fatal("Hint should not be empty")
	}

	t.Logf("hint: %s", hint)
}

// TestGetNetworkBodies_NonEmpty_NoHint verifies that when network_bodies
// has entries, no hint is present.
func TestGetNetworkBodies_NonEmpty_NoHint(t *testing.T) {
	t.Parallel()

	env := newToolTestEnv(t)

	env.capture.AddNetworkBodiesForTest([]types.NetworkBody{
		{
			URL:          "https://api.github.com/repos",
			Method:       "GET",
			Status:       200,
			ResponseBody: `{"repos":[]}`,
			Timestamp:    time.Now().Format(time.RFC3339),
		},
	})

	resp := observe.GetNetworkBodies(env.handler, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))

	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	count, _ := data["count"].(float64)
	if count != 1 {
		t.Fatalf("Expected 1 entry, got %v", count)
	}

	// No hint when results are present
	if _, ok := data["hint"]; ok {
		t.Error("Expected NO hint when network_bodies has entries")
	}
}

// TestGetNetworkBodies_EmptyWithURLFilter_HintMentionsFilter verifies that
// when filtering by URL yields 0 results but unfiltered has data, the hint reflects this.
func TestGetNetworkBodies_EmptyWithURLFilter_HintMentionsFilter(t *testing.T) {
	t.Parallel()

	env := newToolTestEnv(t)

	// Add a body that won't match the filter
	env.capture.AddNetworkBodiesForTest([]types.NetworkBody{
		{
			URL:          "https://api.example.com/users",
			Method:       "GET",
			Status:       200,
			ResponseBody: `{"users":[]}`,
			Timestamp:    time.Now().Format(time.RFC3339),
		},
	})

	// Filter for a URL that doesn't match
	resp := observe.GetNetworkBodies(env.handler, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{"url":"github.com"}`))

	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	count, _ := data["count"].(float64)
	if count != 0 {
		t.Fatalf("Expected 0 entries with filter, got %v", count)
	}

	// Should have a hint about the filter
	hint, ok := data["hint"].(string)
	if !ok {
		t.Fatal("Expected 'hint' field when filtered results are empty")
	}

	if !strings.Contains(hint, "filter") {
		t.Errorf("Hint should mention filtering. Got: %s", hint)
	}

	t.Logf("hint: %s", hint)
}

// ============================================
// Issue #287: websocket_events empty
// ============================================

// TestGetWSEvents_Empty_ReturnsHint verifies that when websocket_events
// returns 0 entries, a diagnostic hint is included.
func TestGetWSEvents_Empty_ReturnsHint(t *testing.T) {
	t.Parallel()

	env := newToolTestEnv(t)

	// No WebSocket events

	resp := observe.GetWSEvents(env.handler, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))

	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	count, _ := data["count"].(float64)
	if count != 0 {
		t.Fatalf("Expected 0 entries, got %v", count)
	}

	// Must have a hint
	hint, ok := data["hint"].(string)
	if !ok {
		t.Fatal("Expected 'hint' field when websocket_events is empty")
	}

	// Hint should explain WebSocket capture is prospective
	if !strings.Contains(strings.ToLower(hint), "websocket") {
		t.Errorf("Hint should mention WebSocket. Got: %s", hint)
	}

	t.Logf("hint: %s", hint)
}

// TestGetWSEvents_NonEmpty_NoHint verifies no hint when events exist.
func TestGetWSEvents_NonEmpty_NoHint(t *testing.T) {
	t.Parallel()

	env := newToolTestEnv(t)

	env.capture.AddWebSocketEventsForTest([]types.WebSocketEvent{
		{
			URL:       "wss://stream.example.com/ws",
			Type:      "message",
			Direction: "received",
			Data:      `{"type":"ping"}`,
			Timestamp: time.Now().Format(time.RFC3339),
		},
	})

	resp := observe.GetWSEvents(env.handler, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))

	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	count, _ := data["count"].(float64)
	if count != 1 {
		t.Fatalf("Expected 1 entry, got %v", count)
	}

	if _, ok := data["hint"]; ok {
		t.Error("Expected NO hint when websocket_events has entries")
	}
}

// TestGetWSStatus_Empty_ReturnsHint verifies that when websocket_status
// returns 0 connections, a diagnostic hint is included.
func TestGetWSStatus_Empty_ReturnsHint(t *testing.T) {
	t.Parallel()

	env := newToolTestEnv(t)

	// No WebSocket connections

	resp := observe.GetWSStatus(env.handler, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))

	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	activeCount, _ := data["active_count"].(float64)
	closedCount, _ := data["closed_count"].(float64)
	if activeCount != 0 || closedCount != 0 {
		t.Fatalf("Expected 0 connections, got active=%v closed=%v", activeCount, closedCount)
	}

	// Must have a hint
	hint, ok := data["hint"].(string)
	if !ok {
		t.Fatal("Expected 'hint' field when websocket_status has no connections")
	}

	if !strings.Contains(strings.ToLower(hint), "websocket") {
		t.Errorf("Hint should mention WebSocket. Got: %s", hint)
	}

	t.Logf("hint: %s", hint)
}

func TestGetWSStatus_SummaryMode_ReturnsCompactShape(t *testing.T) {
	t.Parallel()

	env := newToolTestEnv(t)
	now := time.Now()

	wsPayload := struct {
		Events []types.WebSocketEvent `json:"events"`
	}{
		Events: []types.WebSocketEvent{
			{
				URL:       "wss://realtime.example.com/live",
				Type:      "websocket",
				Event:     "open",
				ID:        "ws-active-1",
				Timestamp: now.Add(-2 * time.Second).Format(time.RFC3339),
			},
			{
				URL:       "wss://realtime.example.com/archive",
				Type:      "websocket",
				Event:     "open",
				ID:        "ws-closed-1",
				Timestamp: now.Add(-4 * time.Second).Format(time.RFC3339),
			},
			{
				URL:         "wss://realtime.example.com/archive",
				Type:        "websocket",
				Event:       "close",
				ID:          "ws-closed-1",
				CloseCode:   1000,
				CloseReason: "normal",
				Timestamp:   now.Add(-1 * time.Second).Format(time.RFC3339),
			},
		},
	}
	body, _ := json.Marshal(wsPayload)
	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.capture.HandleWebSocketEvents(rec, req)
	if rec.Code != 200 {
		t.Fatalf("expected websocket-events ingest 200, got %d", rec.Code)
	}

	resp := observe.GetWSStatus(env.handler, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{"summary":true}`))
	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	if _, ok := data["connections"]; ok {
		t.Fatal("summary mode should omit full connections array")
	}
	if _, ok := data["closed"]; ok {
		t.Fatal("summary mode should omit full closed array")
	}

	if activeCount, _ := data["active_count"].(float64); activeCount != 1 {
		t.Fatalf("active_count = %v, want 1", data["active_count"])
	}
	if closedCount, _ := data["closed_count"].(float64); closedCount != 1 {
		t.Fatalf("closed_count = %v, want 1", data["closed_count"])
	}

	activeURLs, ok := data["active_urls"].([]any)
	if !ok || len(activeURLs) == 0 {
		t.Fatalf("expected active_urls array in summary mode, got %T", data["active_urls"])
	}
	closedURLs, ok := data["closed_urls"].([]any)
	if !ok || len(closedURLs) == 0 {
		t.Fatalf("expected closed_urls array in summary mode, got %T", data["closed_urls"])
	}

	if activeURLs[0] != "wss://realtime.example.com/live" {
		t.Fatalf("active_urls[0] = %v, want wss://realtime.example.com/live", activeURLs[0])
	}
	if closedURLs[0] != "wss://realtime.example.com/archive" {
		t.Fatalf("closed_urls[0] = %v, want wss://realtime.example.com/archive", closedURLs[0])
	}
}
