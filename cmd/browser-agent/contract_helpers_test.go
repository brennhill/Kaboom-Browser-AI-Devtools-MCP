// Purpose: Tests for contract test helper utilities.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// contract_helpers_test.go — Response shape contract testing utilities.
// Provides scenario builders and JSON shape assertion helpers for MCP tool responses.
// These helpers verify field presence and types without checking exact values,
// catching field renames and type changes without brittleness.
package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// Scenario Builder
// ============================================

// scenario wraps observeTestEnv with data loading methods for contract tests.
type scenario struct {
	*observeTestEnv
}

// newScenario creates a fresh test environment for contract testing.
func newScenario(t *testing.T) *scenario {
	t.Helper()
	return &scenario{observeTestEnv: newObserveTestEnv(t)}
}

// loadConsoleData populates the server with realistic console log entries.
func (s *scenario) loadConsoleData(t *testing.T) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	s.addLogEntry(types.LogEntry{
		"type": "console", "level": "error",
		"message":   "Uncaught TypeError: Cannot read property 'id' of undefined",
		"source":    "https://app.example.com/main.js",
		"url":       "https://app.example.com/main.js",
		"line":      float64(42),
		"column":    float64(15),
		"stack":     "TypeError: Cannot read property 'id' of undefined\n    at render (main.js:42:15)",
		"timestamp": now,
	})
	s.addLogEntry(types.LogEntry{
		"type": "console", "level": "warn",
		"message":   "componentWillMount is deprecated",
		"source":    "https://app.example.com/vendor.js",
		"url":       "https://app.example.com/vendor.js",
		"line":      float64(100),
		"timestamp": now,
	})
	s.addLogEntry(types.LogEntry{
		"type": "console", "level": "log",
		"message":   "App initialized successfully",
		"source":    "https://app.example.com/main.js",
		"timestamp": now,
	})
}

// loadNetworkData populates capture with network waterfall and body entries.
func (s *scenario) loadNetworkData(t *testing.T) {
	t.Helper()
	s.capture.Telemetry().NetworkWaterfall().Add([]types.NetworkWaterfallEntry{
		{
			URL: "https://api.example.com/users", Name: "https://api.example.com/users",
			InitiatorType: "fetch", Duration: 150.5, StartTime: 1000.0,
			FetchStart: 1000.0, ResponseEnd: 1150.5,
			TransferSize: 2048, DecodedBodySize: 4096, EncodedBodySize: 2048,
			PageURL: "https://app.example.com/dashboard",
		},
	}, "https://app.example.com/dashboard")

	s.capture.Telemetry().AddNetworkBodies([]types.NetworkBody{
		{
			URL: "https://api.example.com/users", Method: "GET", Status: 200,
			ResponseBody: `{"users":[{"id":1,"name":"Alice"}]}`,
			Timestamp:    time.Now().Format(time.RFC3339),
		},
	})
}

// loadWebSocketData populates capture with WebSocket events.
func (s *scenario) loadWebSocketData(t *testing.T) {
	t.Helper()
	wsPayload := struct {
		Events []types.WebSocketEvent `json:"events"`
	}{
		Events: []types.WebSocketEvent{
			{
				URL: "wss://realtime.example.com/ws", Type: "websocket",
				Event: "open", ID: "ws-conn-1",
				Timestamp: time.Now().Add(-2 * time.Second).Format(time.RFC3339),
			},
			{
				URL: "wss://realtime.example.com/ws", Type: "websocket",
				Event: "message", ID: "ws-conn-1", Direction: "incoming",
				Data:      `{"type":"ping"}`,
				Timestamp: time.Now().Add(-1 * time.Second).Format(time.RFC3339),
			},
			{
				URL: "wss://realtime.example.com/ws", Type: "websocket",
				Event: "close", ID: "ws-conn-1", CloseCode: 1000, CloseReason: "normal",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		},
	}
	body, _ := json.Marshal(wsPayload)
	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	captureHTTPForTest(s.capture).HandleWebSocketEvents(w, req)
}

// loadActionData populates capture with enhanced actions.
func (s *scenario) loadActionData(t *testing.T) {
	t.Helper()
	actionPayload := struct {
		Actions []types.EnhancedAction `json:"actions"`
	}{
		Actions: []types.EnhancedAction{
			{
				Type: "click", Timestamp: time.Now().Add(-1 * time.Second).UnixMilli(),
				URL: "https://app.example.com/dashboard",
			},
			{
				Type: "navigate", Timestamp: time.Now().UnixMilli(),
				URL:   "https://app.example.com/settings",
				ToURL: "https://app.example.com/settings",
			},
		},
	}
	body, _ := json.Marshal(actionPayload)
	req := httptest.NewRequest("POST", "/enhanced-actions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	captureHTTPForTest(s.capture).HandleEnhancedActions(w, req)
}

// loadExtensionLogs populates capture with extension debug logs.
func (s *scenario) loadExtensionLogs(t *testing.T) {
	t.Helper()
	s.capture.ExtensionLogs().Add([]types.ExtensionLog{
		{
			Level: "debug", Message: "Connection established",
			Source: "background", Category: "CONNECTION",
			Timestamp: time.Now(),
		},
	})
}

// loadTrackingState simulates extension sync with a tracked tab.
func (s *scenario) loadTrackingState(t *testing.T) {
	t.Helper()
	syncReq := httptest.NewRequest("POST", "/sync", bytes.NewReader([]byte(`{
		"ext_session_id": "test-session",
		"settings": {
			"pilot_enabled": true,
			"tracking_enabled": true,
			"tracked_tab_id": 123,
			"tracked_tab_url": "https://app.example.com/dashboard",
			"tracked_tab_title": "Dashboard - App",
			"capture_logs": true,
			"capture_network": true,
			"capture_websocket": true,
			"capture_actions": true
		}
	}`)))
	syncReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	newSyncHandler(s.capture).HandleSync(w, syncReq)
}

// loadFullScenario loads all data types for comprehensive testing.
func (s *scenario) loadFullScenario(t *testing.T) {
	t.Helper()
	s.loadConsoleData(t)
	s.loadNetworkData(t)
	s.loadWebSocketData(t)
	s.loadActionData(t)
	s.loadExtensionLogs(t)
	s.loadTrackingState(t)
}

// ============================================
// Response Shape Assertions
// ============================================

// fieldSpec describes an expected field in a JSON response.
type fieldSpec struct {
	key      string
	kind     string // "string", "number", "array", "object", "bool", "any"
	required bool
}

func required(key, kind string) fieldSpec {
	return fieldSpec{key: key, kind: kind, required: true}
}

func optional(key, kind string) fieldSpec {
	return fieldSpec{key: key, kind: kind, required: false}
}

// assertResponseShape parses an MCP tool result's text content as JSON
// and verifies field presence and types against the given spec.
func assertResponseShape(t *testing.T, mode string, result mcp.MCPToolResult, fields []fieldSpec) {
	t.Helper()
	data := parseResponseJSON(t, result)
	assertObjectShape(t, mode, data, fields)
}

// assertObjectShape checks field presence and types on a JSON object.
func assertObjectShape(t *testing.T, label string, data map[string]any, fields []fieldSpec) {
	t.Helper()

	for _, f := range fields {
		val, exists := data[f.key]

		if f.required && !exists {
			t.Errorf("%s contract VIOLATION: required field %q missing. Present: %v",
				label, f.key, mapKeys(data))
			continue
		}

		if !exists || f.kind == "any" {
			continue
		}

		// Optional fields may be null (handler copies nil from source entry)
		if !f.required && val == nil {
			continue
		}

		actual := jsonKind(val)
		if actual != f.kind {
			t.Errorf("%s contract VIOLATION: field %q expected type %q, got %q (value: %v)",
				label, f.key, f.kind, actual, val)
		}
	}
}

// assertStructuredError verifies the structured error shape on an isError response.
func assertStructuredError(t *testing.T, label string, result mcp.MCPToolResult) {
	t.Helper()
	assertStructuredErrorCode(t, label, result, "")
}

// assertStructuredErrorCode verifies the structured error shape and optionally
// checks the error code matches expectedCode. Pass "" to skip code check.
func assertStructuredErrorCode(t *testing.T, label string, result mcp.MCPToolResult, expectedCode string) {
	t.Helper()

	if !result.IsError {
		t.Fatalf("%s: expected isError:true", label)
	}
	if len(result.Content) == 0 {
		t.Fatalf("%s: no content blocks in error response", label)
	}

	text := result.Content[0].Text
	jsonText := extractJSONFromText(text)
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("%s: structured error is not valid JSON: %v\ntext: %s", label, err, text[:min(len(text), 200)])
	}

	assertObjectShape(t, label+" (structured_error)", data, []fieldSpec{
		required("error_code", "string"),
		required("message", "string"),
		required("recovery_playbook", "string"),
	})

	if expectedCode != "" {
		code, _ := data["error_code"].(string)
		if code != expectedCode {
			t.Errorf("%s: expected error code %q, got %q", label, expectedCode, code)
		}
	}
}

// parseResponseJSON extracts the JSON object from an MCPToolResult's text content.
func parseResponseJSON(t *testing.T, result mcp.MCPToolResult) map[string]any {
	t.Helper()

	if len(result.Content) == 0 {
		t.Fatal("contract: no content blocks in response")
	}
	if result.IsError {
		t.Fatalf("contract: unexpected error response: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("contract: response text is not valid JSON:\n%s\nerror: %v",
			text[:min(len(text), 300)], err)
	}
	return data
}

// jsonKind returns the JSON type name for a Go value from json.Unmarshal.
func jsonKind(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "bool"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}

// mapKeys returns sorted keys of a map for error messages.
func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
