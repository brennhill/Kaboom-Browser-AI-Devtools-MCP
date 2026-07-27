// Purpose: Tests for observe tool handler dispatch.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// TODO: Split into tools_observe_page_test.go for page-specific tests when this file approaches 1000 LOC.

// tools_observe_handler_test.go — Comprehensive unit tests for observe tool dispatch and response fields.
// Validates all response fields, snake_case JSON convention, and dispatch logic.
package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolobserve"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// Dispatch Tests
// ============================================

func TestToolsObserveDispatch_InvalidJSON(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.observeDispatcher.Handle(req, json.RawMessage(`{bad json`))

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid JSON should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "invalid_json") {
		t.Errorf("error code should be 'invalid_json', got: %s", result.Content[0].Text)
	}
}

func TestToolsObserveDispatch_MissingWhat(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.observeDispatcher.Handle(req, json.RawMessage(`{}`))

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("missing 'what' should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "missing_param") {
		t.Errorf("error code should be 'missing_param', got: %s", result.Content[0].Text)
	}
	// Verify hint contains valid modes
	if !strings.Contains(result.Content[0].Text, "errors") {
		t.Error("hint should list valid modes including 'errors'")
	}
}

func TestToolsObserveDispatch_UnknownMode(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callObserveRaw(h, "nonexistent_mode")
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("unknown mode should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "unknown_mode") {
		t.Errorf("error code should be 'unknown_mode', got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "nonexistent_mode") {
		t.Error("error should mention the invalid mode name")
	}
}

func TestToolsObserveDispatch_UnknownModeAliasAddsCanonicalWhatWarning(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.observeDispatcher.Handle(req, json.RawMessage(`{"mode":"nonexistent_mode"}`))
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("unknown mode alias should return isError:true")
	}
	foundCanonicalWarning := false
	for _, block := range result.Content {
		if strings.Contains(block.Text, "deprecated") {
			foundCanonicalWarning = true
			break
		}
	}
	if !foundCanonicalWarning {
		t.Fatalf("expected canonical what warning block on error path, got %d content blocks", len(result.Content))
	}
}

func TestToolsObserveDispatch_EmptyArgs(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.observeDispatcher.Handle(req, nil)

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("nil args (no 'what') should return isError:true")
	}
}

func TestToolsObserveDispatch_ModeAliasAddsCanonicalWhatWarning(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.observeDispatcher.Handle(req, json.RawMessage(`{"mode":"pilot"}`))
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("mode alias should be accepted, got: %s", result.Content[0].Text)
	}
	foundCanonicalWarning := false
	for _, block := range result.Content {
		if strings.Contains(block.Text, "deprecated") {
			foundCanonicalWarning = true
			break
		}
	}
	if !foundCanonicalWarning {
		t.Fatalf("expected canonical what warning block, got %d content blocks", len(result.Content))
	}
}

func TestToolsObserveDispatch_ConflictingWhatAndMode(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.observeDispatcher.Handle(req, json.RawMessage(`{"what":"pilot","mode":"errors"}`))
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("conflicting what/mode should return isError:true")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "invalid_param") {
		t.Fatalf("expected invalid_param, got: %s", text)
	}
	if !strings.Contains(text, "Conflicting parameters") {
		t.Fatalf("expected conflict explanation, got: %s", text)
	}
}

// ============================================
// observe(what:"errors") — Response Field Tests
// ============================================

func TestToolsObserveErrors_ResponseFields(t *testing.T) {
	t.Parallel()
	h, server, cap := makeToolHandler(t)
	_ = cap

	ts := time.Now().UTC().Format(time.RFC3339)
	server.logs.SeedEntries([]mcp.LogEntry{
		mcp.LogEntry{
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
	server.logs.SeedEntries([]mcp.LogEntry{
		mcp.LogEntry{"level": "error", "message": "Error A", "url": "https://example.com/a.js", "ts": ts},
		mcp.LogEntry{"level": "error", "message": "Error B", "url": "https://other.com/b.js", "ts": ts},
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
		server.logs.SeedEntries([]mcp.LogEntry{{"level": "error", "message": "err", "ts": ts}}, nil)
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
	server.logs.SeedEntries([]mcp.LogEntry{
		mcp.LogEntry{
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

	cap.AddExtensionLogs([]capture.ExtensionLog{{
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

	cap.AddNetworkBodies([]capture.NetworkBody{
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
	cap.AddNetworkBodies([]capture.NetworkBody{
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
	cap.AddNetworkBodies([]capture.NetworkBody{
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
	cap.AddNetworkBodies([]capture.NetworkBody{
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

	cap.AddWebSocketEvents([]capture.WebSocketEvent{
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

	cap.AddEnhancedActionsForTest([]capture.EnhancedAction{
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

func TestToolsObservePilot_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callObserveRaw(h, "pilot")
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("pilot should not error, got: %s", result.Content[0].Text)
	}

	// Pilot is a server-side mode, should not get disconnect warning
	text := result.Content[0].Text
	if strings.Contains(text, "Extension is not connected") {
		t.Error("pilot is server-side mode, should NOT get disconnect warning")
	}
}

// ============================================
// observe(what:"page") — page_ready_for_commands Tests
// ============================================

func TestToolsObservePage_PageReadyForCommands_AllConditionsMet(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	// Set up all conditions for page_ready_for_commands=true
	cap.SimulateExtensionConnectForTest()
	cap.SetPilotEnabled(true)
	cap.SetTrackingStatusForTest(42, "https://example.com")
	cap.SetTabStatusForTest("complete")

	resp := callObserveRaw(h, "page")
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("page should not error, got: %s", result.Content[0].Text)
	}
	data := extractResultJSON(t, result)

	ready, ok := data["page_ready_for_commands"]
	if !ok {
		t.Fatal("response missing 'page_ready_for_commands' field")
	}
	if ready != true {
		t.Errorf("page_ready_for_commands = %v, want true", ready)
	}

	tabStatus, ok := data["tab_status"]
	if !ok {
		t.Fatal("response missing 'tab_status' field")
	}
	if tabStatus != "complete" {
		t.Errorf("tab_status = %v, want 'complete'", tabStatus)
	}
}

func TestToolsObservePage_PageReadyForCommands_ExtensionDisconnected(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	cap.SimulateExtensionDisconnectForTest()
	cap.SetPilotEnabled(true)
	cap.SetTrackingStatusForTest(42, "https://example.com")
	cap.SetTabStatusForTest("complete")

	resp := callObserveRaw(h, "page")
	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	if data["page_ready_for_commands"] != false {
		t.Errorf("page_ready_for_commands = %v, want false (extension disconnected)", data["page_ready_for_commands"])
	}
}

func TestToolsObservePage_PageReadyForCommands_PilotDisabled(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	cap.SimulateExtensionConnectForTest()
	cap.SetPilotEnabled(false)
	cap.SetTrackingStatusForTest(42, "https://example.com")
	cap.SetTabStatusForTest("complete")

	resp := callObserveRaw(h, "page")
	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	if data["page_ready_for_commands"] != false {
		t.Errorf("page_ready_for_commands = %v, want false (pilot disabled)", data["page_ready_for_commands"])
	}
}

func TestToolsObservePage_PageReadyForCommands_NoTrackedTab(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	cap.SimulateExtensionConnectForTest()
	cap.SetPilotEnabled(true)
	// No tracked tab set
	cap.SetTabStatusForTest("complete")

	resp := callObserveRaw(h, "page")
	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	if data["page_ready_for_commands"] != false {
		t.Errorf("page_ready_for_commands = %v, want false (no tracked tab)", data["page_ready_for_commands"])
	}
}

func TestToolsObservePage_PageReadyForCommands_TabLoading(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	cap.SimulateExtensionConnectForTest()
	cap.SetPilotEnabled(true)
	cap.SetTrackingStatusForTest(42, "https://example.com")
	cap.SetTabStatusForTest("loading")

	resp := callObserveRaw(h, "page")
	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	if data["page_ready_for_commands"] != false {
		t.Errorf("page_ready_for_commands = %v, want false (tab loading)", data["page_ready_for_commands"])
	}
	if data["tab_status"] != "loading" {
		t.Errorf("tab_status = %v, want 'loading'", data["tab_status"])
	}
}

func TestToolsObservePage_DataAgeMs_Present(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SimulateExtensionConnectForTest()

	resp := callObserveRaw(h, "page")
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

func TestToolsObserveErrors_DataAgeMs_Present(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	ts := time.Now().UTC().Format(time.RFC3339)
	server.logs.SeedEntries([]mcp.LogEntry{
		mcp.LogEntry{
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

// ============================================
// isServerSideObserveMode Tests
// ============================================

func TestToolsObserve_IsServerSideObserveMode(t *testing.T) {
	t.Parallel()

	serverSide := []string{
		"command_result", "pending_commands", "failed_commands",
		"saved_videos", "recordings", "recording_actions",
		"playback_results", "log_diff_report", "pilot",
	}
	for _, mode := range serverSide {
		if !toolobserve.ServerSideObserveModes[mode] {
			t.Errorf("toolobserve.ServerSideObserveModes[%q] = false, want true", mode)
		}
	}

	clientSide := []string{"errors", "logs", "network_bodies", "actions", "vitals"}
	for _, mode := range clientSide {
		if toolobserve.ServerSideObserveModes[mode] {
			t.Errorf("toolobserve.ServerSideObserveModes[%q] = true, want false", mode)
		}
	}
}

// ============================================
// getValidObserveModes Tests
// ============================================

func TestToolsObserve_GetValidObserveModes(t *testing.T) {
	t.Parallel()

	h, _, _ := makeToolHandler(t)
	modes := strings.Join(h.observeDispatcher.ValidModes(), ", ")
	// Should be sorted
	modeList := strings.Split(modes, ", ")
	for i := 1; i < len(modeList); i++ {
		if modeList[i-1] > modeList[i] {
			t.Errorf("modes not sorted: %q > %q", modeList[i-1], modeList[i])
		}
	}

	// Should contain key modes
	for _, required := range []string{"errors", "logs", "network_bodies", "actions"} {
		if !strings.Contains(modes, required) {
			t.Errorf("valid modes missing %q: %s", required, modes)
		}
	}
}

// ============================================
// Structured Error Field Validation
// ============================================

func TestToolsObserve_StructuredErrorFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callObserveRaw(h, "nonexistent")
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("expected isError:true")
	}

	text := result.Content[0].Text
	errorJSON := extractJSONFromText(text)

	var se mcp.StructuredError
	if err := json.Unmarshal([]byte(errorJSON), &se); err != nil {
		t.Fatalf("structured error JSON parse failed: %v\nraw: %s", err, text)
	}
	if se.ErrorCode == "" {
		t.Error("mcp.StructuredError.ErrorCode should not be empty")
	}
	if se.Message == "" {
		t.Error("mcp.StructuredError.Message should not be empty")
	}
	if se.RecoveryPlaybook == "" {
		t.Error("mcp.StructuredError.RecoveryPlaybook should not be empty")
	}

	// Verify JSON fields are snake_case
	assertSnakeCaseFields(t, errorJSON)
}
