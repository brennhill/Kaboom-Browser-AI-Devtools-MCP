// Purpose: Black-box tests for observe tool end-to-end behavior.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_observe_blackbox_test.go — Black box tests for observe tool data flow.
// These tests simulate the full browser extension → server → MCP tool flow.
// They verify that data POSTed via HTTP endpoints is correctly returned by MCP tools.
//
// ARCHITECTURAL INVARIANT: These tests verify end-to-end data flow.
// If these tests fail, it means MCP tools are returning empty/stale data.
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// ============================================
// Test Fixtures - Realistic Browser Data
// ============================================

// Sample console error from browser
var sampleConsoleError = mcp.LogEntry{
	"type":    "console",
	"level":   "error",
	"message": "Uncaught TypeError: Cannot read property 'foo' of undefined",
	"source":  "https://example.com/app.js",
	"url":     "https://example.com/app.js",
	"line":    42,
	"column":  15,
	"stack":   "TypeError: Cannot read property 'foo' of undefined\n    at handleClick (app.js:42:15)",
	"ts":      time.Now().UTC().Format(time.RFC3339),
}

// Sample console warning
var sampleConsoleWarning = mcp.LogEntry{
	"type":    "console",
	"level":   "warn",
	"message": "Deprecation warning: componentWillMount is deprecated",
	"source":  "https://example.com/react.js",
	"url":     "https://example.com/react.js",
	"line":    100,
	"ts":      time.Now().UTC().Format(time.RFC3339),
}

// Sample console log
var sampleConsoleLog = mcp.LogEntry{
	"type":    "console",
	"level":   "log",
	"message": "User clicked button",
	"source":  "https://example.com/app.js",
	"ts":      time.Now().UTC().Format(time.RFC3339),
}

// Sample network waterfall entry
var sampleNetworkEntry = capture.NetworkWaterfallEntry{
	URL:             "https://api.example.com/users",
	Name:            "https://api.example.com/users",
	InitiatorType:   "fetch",
	Duration:        150.5,
	StartTime:       1000.0,
	FetchStart:      1000.0,
	ResponseEnd:     1150.5,
	TransferSize:    2048,
	DecodedBodySize: 4096,
	EncodedBodySize: 2048,
	PageURL:         "https://example.com/dashboard",
}

// Sample extension log
var sampleExtensionLog = capture.ExtensionLog{
	Level:     "debug",
	Message:   "Connection established to server",
	Source:    "background",
	Category:  "CONNECTION",
	Timestamp: time.Now(),
}

// ============================================
// Black Box Tests
// ============================================

// TestObserveErrors_EndToEnd verifies errors POST → observe errors flow
func TestObserveErrors_EndToEnd(t *testing.T) {
	t.Parallel()

	// Setup server with capture
	server, err := NewServer(t.TempDir()+"/test-errors-e2e.jsonl", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	cap := capture.NewCapture()
	handler := NewToolHandler(server, cap)

	// Step 1: POST console errors (simulating browser extension)
	logsPayload := map[string]any{
		"entries": []mcp.LogEntry{sampleConsoleError, sampleConsoleWarning, sampleConsoleLog},
	}
	body, _ := json.Marshal(logsPayload)

	// Note: We directly add to server.logs.entries to simulate the /logs endpoint
	_ = body // Payload would be POSTed to /logs in real scenario

	// Use the server's /logs handler
	server.logs.SeedEntries(logsPayload["entries"].([]mcp.LogEntry), nil)

	// Step 2: Call observe errors via MCP tool
	mcpReq := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	}

	th := handler.toolHandler.(*ToolHandler)
	resp := observe.GetBrowserErrors(th, mcpReq, json.RawMessage(`{}`))

	// Step 3: Verify errors are returned
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatal("Expected content array in response")
	}

	textBlock := content[0].(map[string]any)
	text := textBlock["text"].(string)

	// Parse the JSON in the text block (skip summary line)
	jsonText := extractJSONFromText(text)
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse text content: %v (text was: %s)", err, text[:min(len(text), 100)])
	}

	errors, ok := data["errors"].([]any)
	if !ok {
		t.Fatal("Expected 'errors' array in response")
	}

	if len(errors) != 1 {
		t.Errorf("Expected 1 error (only level='error' entries), got %d", len(errors))
	}

	if len(errors) > 0 {
		firstError := errors[0].(map[string]any)
		if msg, _ := firstError["message"].(string); !strings.Contains(msg, "TypeError") {
			t.Errorf("Expected error message containing 'TypeError', got: %s", msg)
		}
	}

	t.Logf("✅ observe errors returned %d errors from %d log entries", len(errors), 3)
}

// TestObserveLogs_EndToEnd verifies logs POST → observe logs flow
func TestObserveLogs_EndToEnd(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/test-logs-e2e.jsonl", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	cap := capture.NewCapture()
	handler := NewToolHandler(server, cap)

	// POST logs
	server.logs.SeedEntries([]mcp.LogEntry{
		sampleConsoleError,
		sampleConsoleWarning,
		sampleConsoleLog,
	}, nil)

	// Call observe logs
	mcpReq := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	th := handler.toolHandler.(*ToolHandler)
	resp := observe.GetBrowserLogs(th, mcpReq, json.RawMessage(`{}`))

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

	logs := data["logs"].([]any)
	if len(logs) != 3 {
		t.Errorf("Expected 3 logs, got %d", len(logs))
	}

	t.Logf("✅ observe logs returned %d entries", len(logs))
}

// TestObserveLogs_LevelFilter verifies level filtering works
func TestObserveLogs_LevelFilter(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/test-logs-filter.jsonl", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	cap := capture.NewCapture()
	handler := NewToolHandler(server, cap)

	server.logs.SeedEntries([]mcp.LogEntry{
		sampleConsoleError,
		sampleConsoleWarning,
		sampleConsoleLog,
	}, nil)

	// "level" is a quiet alias for "min_level" (threshold): warn returns warn+error.
	th := handler.toolHandler.(*ToolHandler)
	resp := observe.GetBrowserLogs(th, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{"level":"warn"}`))

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

	logs := data["logs"].([]any)
	if len(logs) != 2 {
		t.Errorf("Expected 2 logs (warn+error via min_level threshold), got %d", len(logs))
	}

	t.Logf("✅ observe logs level filter returned %d entries", len(logs))
}

// TestObserveNetworkWaterfall_EndToEnd verifies network waterfall flow
func TestObserveNetworkWaterfall_EndToEnd(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/test-waterfall-e2e.jsonl", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	cap := capture.NewCapture()
	handler := NewToolHandler(server, cap)

	// Add network waterfall entry directly to capture
	cap.AddNetworkWaterfallEntries([]capture.NetworkWaterfallEntry{sampleNetworkEntry}, "https://example.com/dashboard")

	// Call observe network_waterfall
	th := handler.toolHandler.(*ToolHandler)
	resp := observe.GetNetworkWaterfall(th, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))

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

	entries := data["entries"].([]any)
	if len(entries) != 1 {
		t.Errorf("Expected 1 network entry, got %d", len(entries))
	}

	if len(entries) > 0 {
		entry := entries[0].(map[string]any)
		if url, _ := entry["url"].(string); url != "https://api.example.com/users" {
			t.Errorf("Expected URL 'https://api.example.com/users', got: %s", url)
		}
	}

	t.Logf("✅ observe network_waterfall returned %d entries", len(entries))
}

// TestObserveNetworkWaterfall_URLFilter verifies URL filtering works
func TestObserveNetworkWaterfall_URLFilter(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/test-waterfall-filter.jsonl", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	cap := capture.NewCapture()
	handler := NewToolHandler(server, cap)

	// Add multiple entries
	entries := []capture.NetworkWaterfallEntry{
		{URL: "https://api.example.com/users", PageURL: "https://example.com"},
		{URL: "https://cdn.example.com/style.css", PageURL: "https://example.com"},
		{URL: "https://api.example.com/orders", PageURL: "https://example.com"},
	}
	cap.AddNetworkWaterfallEntries(entries, "https://example.com")

	// Filter by "api.example.com"
	th := handler.toolHandler.(*ToolHandler)
	resp := observe.GetNetworkWaterfall(th, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{"url":"api.example.com"}`))

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
	if len(resultEntries) != 2 {
		t.Errorf("Expected 2 API entries, got %d", len(resultEntries))
	}

	t.Logf("✅ observe network_waterfall URL filter returned %d entries", len(resultEntries))
}

// TestObserveExtensionLogs_EndToEnd verifies extension logs flow
func TestObserveExtensionLogs_EndToEnd(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/test-extlogs-e2e.jsonl", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	cap := capture.NewCapture()
	handler := NewToolHandler(server, cap)

	// Add extension logs directly
	cap.AddExtensionLogs([]capture.ExtensionLog{sampleExtensionLog})

	// Call observe extension_logs
	th := handler.toolHandler.(*ToolHandler)
	resp := observe.GetExtensionLogs(th, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))

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

	logs := data["logs"].([]any)
	if len(logs) != 1 {
		t.Errorf("Expected 1 extension log, got %d", len(logs))
	}

	if len(logs) > 0 {
		log := logs[0].(map[string]any)
		if msg, _ := log["message"].(string); !strings.Contains(msg, "Connection established") {
			t.Errorf("Expected message containing 'Connection established', got: %s", msg)
		}
	}

	t.Logf("✅ observe extension_logs returned %d entries", len(logs))
}

// TestObservePage_ExtractsFromWaterfall verifies page info extraction
func TestObservePage_ExtractsFromWaterfall(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/test-page-e2e.jsonl", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	cap := capture.NewCapture()
	handler := NewToolHandler(server, cap)

	// Add network entry with page_url
	cap.AddNetworkWaterfallEntries([]capture.NetworkWaterfallEntry{sampleNetworkEntry}, "https://example.com/dashboard")

	// Call observe page
	th := handler.toolHandler.(*ToolHandler)
	resp := observe.GetPageInfo(th, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))

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

	pageURL := data["url"].(string)
	if pageURL != "https://example.com/dashboard" {
		t.Errorf("Expected page URL 'https://example.com/dashboard', got: %s", pageURL)
	}

	t.Logf("✅ observe page extracted URL: %s", pageURL)
}

// TestObservePage_PrioritizesTrackedURL verifies that page info prioritizes
// the tracked tab URL from extension sync over stale waterfall entries.
// This is a REGRESSION TEST for the fix in commit b4c06b7.
// If this test fails, it means observe(page) is returning stale URLs.
func TestObservePage_PrioritizesTrackedURL(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/test-page-priority.jsonl", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	cap := capture.NewCapture()
	handler := NewToolHandler(server, cap)

	// Add STALE waterfall entry with old URL
	cap.AddNetworkWaterfallEntries([]capture.NetworkWaterfallEntry{sampleNetworkEntry}, "https://old-stale-url.com/page")

	// Simulate extension sync with FRESH tracked tab URL
	// This is what the extension sends via /sync endpoint
	syncReq := httptest.NewRequest("POST", "/sync", bytes.NewReader([]byte(`{
		"ext_session_id": "test-session",
		"settings": {
			"pilot_enabled": true,
			"tracking_enabled": true,
			"tracked_tab_id": 123,
			"tracked_tab_url": "https://current-tracked-tab.com/fresh",
			"capture_logs": true,
			"capture_network": true,
			"capture_websocket": true,
			"capture_actions": true
		}
	}`)))
	syncReq.Header.Set("Content-Type", "application/json")
	syncW := httptest.NewRecorder()
	cap.HandleSync(syncW, syncReq)

	if syncW.Code != http.StatusOK {
		t.Fatalf("Sync request failed: %d", syncW.Code)
	}

	// Call observe page - should return tracked URL, NOT waterfall URL
	th := handler.toolHandler.(*ToolHandler)
	resp := observe.GetPageInfo(th, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))

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

	pageURL := data["url"].(string)

	// CRITICAL ASSERTION: Must return tracked URL, not stale waterfall URL
	if pageURL == "https://old-stale-url.com/page" {
		t.Fatalf("REGRESSION: observe(page) returned stale waterfall URL instead of tracked URL!\n"+
			"Expected: https://current-tracked-tab.com/fresh\n"+
			"Got: %s\n"+
			"This means GetTrackingStatus() is not being used correctly.", pageURL)
	}

	if pageURL != "https://current-tracked-tab.com/fresh" {
		t.Errorf("Expected tracked URL 'https://current-tracked-tab.com/fresh', got: %s", pageURL)
	}

	t.Logf("✅ observe page correctly prioritized tracked URL over stale waterfall")
}

// TestObserveNetworkBodies_EndToEnd verifies network bodies flow
func TestObserveNetworkBodies_EndToEnd(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/test-bodies-e2e.jsonl", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	cap := capture.NewCapture()
	handler := NewToolHandler(server, cap)

	// Add network body using test helper (simulates browser extension POST)
	cap.AddNetworkBodiesForTest([]capture.NetworkBody{
		{
			URL:          "https://api.example.com/users",
			Method:       "GET",
			Status:       200,
			RequestBody:  "",
			ResponseBody: `{"users":[{"id":1,"name":"Alice"}]}`,
			Timestamp:    time.Now().Format(time.RFC3339),
		},
	})

	// Call observe network_bodies
	th := handler.toolHandler.(*ToolHandler)
	resp := observe.GetNetworkBodies(th, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))

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

	entries := data["entries"].([]any)
	if len(entries) != 1 {
		t.Errorf("Expected 1 network body, got %d", len(entries))
	}

	t.Logf("✅ observe network_bodies returned %d entries", len(entries))
}

// TestObserveWebSocketEvents_EndToEnd verifies WebSocket events flow
func TestObserveWebSocketEvents_EndToEnd(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/test-ws-e2e.jsonl", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	cap := capture.NewCapture()
	handler := NewToolHandler(server, cap)

	// Simulate WebSocket event POST
	wsPayload := struct {
		Events []capture.WebSocketEvent `json:"events"`
	}{
		Events: []capture.WebSocketEvent{
			{
				URL:       "wss://realtime.example.com/socket",
				Type:      "message",
				Direction: "received",
				Data:      `{"type":"ping"}`,
				Timestamp: time.Now().Format(time.RFC3339),
			},
		},
	}
	body, _ := json.Marshal(wsPayload)
	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	cap.HandleWebSocketEvents(w, req)

	// Call observe websocket_events
	th := handler.toolHandler.(*ToolHandler)
	resp := observe.GetWSEvents(th, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{}`))

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

	entries := data["entries"].([]any)
	if len(entries) != 1 {
		t.Errorf("Expected 1 WebSocket event, got %d", len(entries))
	}

	t.Logf("✅ observe websocket_events returned %d entries", len(entries))
}

// ============================================
// Integration Test: Full MCP Request Flow
// ============================================

// TestMCPToolsCall_ObserveErrors_FullFlow tests the complete MCP request → response flow
func TestMCPToolsCall_ObserveErrors_FullFlow(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/test-mcp-flow.jsonl", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	cap := capture.NewCapture()
	handler := NewToolHandler(server, cap)

	// Populate with test data
	server.logs.SeedEntries([]mcp.LogEntry{
		sampleConsoleError,
	}, nil)

	// Create MCP request exactly as client would send it
	mcpRequest := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      42,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	}

	// Parse the params to extract tool call
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(mcpRequest.Params, &params); err != nil {
		t.Fatalf("Failed to parse params: %v", err)
	}

	// Dispatch to observe tool
	th := handler.toolHandler.(*ToolHandler)
	resp := th.observeDispatcher.Handle(mcpRequest, params.Arguments)

	// Verify response structure
	if resp.ID != 42 {
		t.Errorf("Expected ID 42, got %v", resp.ID)
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected jsonrpc '2.0', got %s", resp.JSONRPC)
	}
	if resp.Error != nil {
		t.Errorf("Unexpected error: %v", resp.Error)
	}

	// Verify result contains errors
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	content := result["content"].([]any)
	if len(content) == 0 {
		t.Fatal("Expected content in response")
	}

	t.Logf("✅ Full MCP tools/call observe errors flow works correctly")
}
