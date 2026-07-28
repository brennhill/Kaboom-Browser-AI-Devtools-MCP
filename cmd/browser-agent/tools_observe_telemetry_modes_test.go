// tools_observe_telemetry_modes_test.go — Tests observe telemetry response modes.
// Docs: docs/features/feature/observe/index.md
package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestToolsObserveErrors_ResponseFields(t *testing.T) {
	t.Parallel()
	h, server, cap := makeToolHandler(t)
	_ = cap

	ts := time.Now().UTC().Format(time.RFC3339)
	server.logs.SeedEntries([]types.LogEntry{
		types.LogEntry{
			"level":   "error",
			"message": "Test error message",
			"source":  "https://example.com/app.js",
			"url":     "https://example.com/app.js",
			"line":    float64(42),
			"column":  float64(10),
			"stack":   "Error: Test\n    at fn (app.js:42:10)",
			"ts":      ts,
			"tabId":   float64(1),
		},
	}, nil)

	resp := callObserveRaw(h, "errors")
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("errors should not return isError, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)

	// Verify top-level fields
	if _, ok := data["errors"]; !ok {
		t.Error("response missing 'errors' field")
	}
	if _, ok := data["count"]; !ok {
		t.Error("response missing 'count' field")
	}
	if _, ok := data["metadata"]; !ok {
		t.Error("response missing 'metadata' field")
	}

	// Verify count
	count, _ := data["count"].(float64)
	if count != 1 {
		t.Errorf("count = %v, want 1", count)
	}

	// Verify error entry fields
	errors, ok := data["errors"].([]any)
	if !ok || len(errors) == 0 {
		t.Fatal("errors should be non-empty array")
	}
	entry, _ := errors[0].(map[string]any)
	for _, field := range []string{"message", "source", "url", "line", "column", "stack", "timestamp", "tab_id"} {
		if _, ok := entry[field]; !ok {
			t.Errorf("error entry missing field %q", field)
		}
	}
	if entry["message"] != "Test error message" {
		t.Errorf("message = %v, want 'Test error message'", entry["message"])
	}

	// Verify metadata fields
	meta, _ := data["metadata"].(map[string]any)
	if meta == nil {
		t.Fatal("metadata should be a map")
	}
	for _, field := range []string{"retrieved_at", "is_stale", "data_age"} {
		if _, ok := meta[field]; !ok {
			t.Errorf("metadata missing field %q", field)
		}
	}

	// Verify snake_case
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsObserveErrors_EmptyBuffer(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callObserveRaw(h, "errors")
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatal("errors with empty buffer should NOT return isError")
	}

	data := extractResultJSON(t, result)
	count, _ := data["count"].(float64)
	if count != 0 {
		t.Errorf("count = %v, want 0", count)
	}
	errors, _ := data["errors"].([]any)
	if len(errors) != 0 {
		t.Errorf("errors length = %d, want 0", len(errors))
	}
}

func TestToolsObserveErrors_URLFilter(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	ts := time.Now().UTC().Format(time.RFC3339)
	server.logs.SeedEntries([]types.LogEntry{
		types.LogEntry{"level": "error", "message": "Error A", "url": "https://example.com/a.js", "ts": ts},
		types.LogEntry{"level": "error", "message": "Error B", "url": "https://other.com/b.js", "ts": ts},
	}, nil)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.observeDispatcher.Handle(req, json.RawMessage(`{"what":"errors","url":"example.com"}`))
	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	count, _ := data["count"].(float64)
	if count != 1 {
		t.Errorf("filtered count = %v, want 1 (only example.com error)", count)
	}
}

func TestToolsObserveErrors_LimitParam(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	ts := time.Now().UTC().Format(time.RFC3339)
	for i := 0; i < 5; i++ {
		server.logs.SeedEntries([]types.LogEntry{{"level": "error", "message": "err", "ts": ts}}, nil)
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.observeDispatcher.Handle(req, json.RawMessage(`{"what":"errors","limit":2}`))
	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	count, _ := data["count"].(float64)
	if count != 2 {
		t.Errorf("count with limit=2 = %v, want 2", count)
	}
}

// ============================================
// observe(what:"logs") — Response Field Tests
// ============================================

func TestToolsObserveLogs_ResponseFields(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	ts := time.Now().UTC().Format(time.RFC3339)
	server.logs.SeedEntries([]types.LogEntry{
		types.LogEntry{
			"type":    "console",
			"level":   "warn",
			"message": "deprecation warning",
			"source":  "https://example.com/lib.js",
			"url":     "https://example.com/lib.js",
			"line":    float64(10),
			"column":  float64(5),
			"ts":      ts,
			"tabId":   float64(2),
		},
	}, nil)
	server.logs.SeedTotalAdded(1)

	resp := callObserveRaw(h, "logs")
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("logs should not return isError, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)

	// Verify top-level fields
	for _, field := range []string{"logs", "count", "metadata"} {
		if _, ok := data[field]; !ok {
			t.Errorf("response missing field %q", field)
		}
	}

	// Verify log entry fields
	logs, ok := data["logs"].([]any)
	if !ok || len(logs) == 0 {
		t.Fatal("logs should be non-empty array")
	}
	entry, _ := logs[0].(map[string]any)
	for _, field := range []string{"level", "message", "source", "url", "line", "column", "timestamp", "tab_id"} {
		if _, ok := entry[field]; !ok {
			t.Errorf("log entry missing field %q", field)
		}
	}
	if entry["level"] != "warn" {
		t.Errorf("level = %v, want 'warn'", entry["level"])
	}

	// Verify paginated metadata fields
	meta, _ := data["metadata"].(map[string]any)
	if meta == nil {
		t.Fatal("metadata should be a map")
	}
	for _, field := range []string{"retrieved_at", "is_stale", "data_age", "total", "has_more"} {
		if _, ok := meta[field]; !ok {
			t.Errorf("metadata missing field %q", field)
		}
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// observe(what:"extension_logs") — Response Fields
// ============================================

func TestToolsObserveExtensionLogs_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	cap.ExtensionLogs().Add([]types.ExtensionLog{{
		Level:     "info",
		Message:   "Extension started",
		Source:    "background.js",
		Category:  "lifecycle",
		Data:      json.RawMessage(`{"version":"1.0"}`),
		Timestamp: time.Now(),
	}})

	resp := callObserveRaw(h, "extension_logs")
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("extension_logs should not error, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"logs", "count", "metadata"} {
		if _, ok := data[field]; !ok {
			t.Errorf("response missing field %q", field)
		}
	}

	logs, _ := data["logs"].([]any)
	if len(logs) == 0 {
		t.Fatal("logs should be non-empty")
	}
	entry, _ := logs[0].(map[string]any)
	for _, field := range []string{"level", "message", "source", "category", "data", "timestamp"} {
		if _, ok := entry[field]; !ok {
			t.Errorf("extension_log entry missing field %q", field)
		}
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// observe(what:"network_bodies") — Response Fields
// ============================================

func TestToolsObserveNetworkBodies_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	cap.Telemetry().AddNetworkBodies([]types.NetworkBody{
		{
			URL:         "https://api.example.com/users",
			Method:      "GET",
			Status:      200,
			ContentType: "application/json",
			Timestamp:   time.Now().UTC().Format(time.RFC3339),
		},
	})

	resp := callObserveRaw(h, "network_bodies")
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("network_bodies should not error, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"entries", "count", "metadata"} {
		if _, ok := data[field]; !ok {
			t.Errorf("response missing field %q", field)
		}
	}

	count, _ := data["count"].(float64)
	if count != 1 {
		t.Errorf("count = %v, want 1", count)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsObserveNetworkBodies_Filters(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	ts := time.Now().UTC().Format(time.RFC3339)
	cap.Telemetry().AddNetworkBodies([]types.NetworkBody{
		{URL: "https://api.example.com/users", Method: "GET", Status: 200, Timestamp: ts},
		{URL: "https://api.example.com/orders", Method: "POST", Status: 201, Timestamp: ts},
		{URL: "https://other.com/data", Method: "GET", Status: 404, Timestamp: ts},
	})

	// Filter by URL
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.observeDispatcher.Handle(req, json.RawMessage(`{"what":"network_bodies","url":"example.com"}`))
	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)
	count, _ := data["count"].(float64)
	if count != 2 {
		t.Errorf("url filter count = %v, want 2", count)
	}

	// Filter by method
	resp = h.observeDispatcher.Handle(req, json.RawMessage(`{"what":"network_bodies","method":"POST"}`))
	result = parseToolResult(t, resp)
	data = extractResultJSON(t, result)
	count, _ = data["count"].(float64)
	if count != 1 {
		t.Errorf("method filter count = %v, want 1", count)
	}

	// Filter by status_min
	resp = h.observeDispatcher.Handle(req, json.RawMessage(`{"what":"network_bodies","status_min":400}`))
	result = parseToolResult(t, resp)
	data = extractResultJSON(t, result)
	count, _ = data["count"].(float64)
	if count != 1 {
		t.Errorf("status_min filter count = %v, want 1", count)
	}
}

func TestToolsObserveNetworkBodies_BodyPathFilter(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	ts := time.Now().UTC().Format(time.RFC3339)
	cap.Telemetry().AddNetworkBodies([]types.NetworkBody{
		{
			URL:          "https://api.example.com/graphql",
			Method:       "POST",
			Status:       200,
			ResponseBody: `{"data":{"viewer":{"id":"u_123","roles":["admin","editor"]}}}`,
			Timestamp:    ts,
		},
		{
			URL:          "https://api.example.com/other",
			Method:       "GET",
			Status:       200,
			ResponseBody: `{"ok":true}`,
			Timestamp:    ts,
		},
	})

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.observeDispatcher.Handle(req, json.RawMessage(`{"what":"network_bodies","body_path":"data.viewer.roles[0]"}`))
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("network_bodies with body_path should not error, got: %s", result.Content[0].Text)
	}
	data := extractResultJSON(t, result)

	count, _ := data["count"].(float64)
	if count != 1 {
		t.Fatalf("count = %v, want 1", count)
	}

	entries, _ := data["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("entries len = %d, want 1", len(entries))
	}
	entry, _ := entries[0].(map[string]any)
	responseBody, _ := entry["response_body"].(string)
	var extracted any
	if err := json.Unmarshal([]byte(responseBody), &extracted); err != nil {
		t.Fatalf("response_body should be valid JSON, got err: %v", err)
	}
	if extracted != "admin" {
		t.Fatalf("extracted value = %v, want admin", extracted)
	}
}

func TestToolsObserveNetworkBodies_BodyPathValidation(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	ts := time.Now().UTC().Format(time.RFC3339)
	cap.Telemetry().AddNetworkBodies([]types.NetworkBody{
		{
			URL:          "https://api.example.com/data",
			Method:       "GET",
			Status:       200,
			ResponseBody: `{"data":{"id":1}}`,
			Timestamp:    ts,
		},
	})

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.observeDispatcher.Handle(req, json.RawMessage(`{"what":"network_bodies","body_path":"data.items["}`))
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid body_path syntax should return isError:true")
	}
}

// ============================================
// observe(what:"websocket_events") — Response Fields
// ============================================

func TestToolsObserveWSEvents_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	cap.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{
			ID:        "ws-1",
			URL:       "wss://stream.example.com",
			Direction: "incoming",
			Data:      `{"type":"message"}`,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	})

	resp := callObserveRaw(h, "websocket_events")
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("websocket_events should not error, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"entries", "count", "metadata"} {
		if _, ok := data[field]; !ok {
			t.Errorf("response missing field %q", field)
		}
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// observe(what:"actions") — Response Fields
// ============================================

func TestToolsObserveActions_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	cap.Telemetry().AddEnhancedActionsForTest([]types.EnhancedAction{
		{Type: "click", Timestamp: time.Now().UnixMilli(), URL: "https://example.com"},
	})

	resp := callObserveRaw(h, "actions")
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("actions should not error, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"entries", "count", "metadata"} {
		if _, ok := data[field]; !ok {
			t.Errorf("response missing field %q", field)
		}
	}

	count, _ := data["count"].(float64)
	if count != 1 {
		t.Errorf("count = %v, want 1", count)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// observe(what:"pilot") — Response Fields
// ============================================

func TestToolsObserveErrors_DataAgeMs_Present(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	ts := time.Now().UTC().Format(time.RFC3339)
	server.logs.SeedEntries([]types.LogEntry{
		types.LogEntry{
			"level": "error", "message": "Test error", "ts": ts,
		},
	}, nil)

	resp := callObserveRaw(h, "errors")
	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	meta, ok := data["metadata"].(map[string]any)
	if !ok {
		t.Fatal("metadata should be a map")
	}
	if _, ok := meta["data_age_ms"]; !ok {
		t.Error("metadata missing 'data_age_ms' field")
	}
}
