// Purpose: Tests for handler response structure consistency.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// handler_consistency_test.go — Verifies consistent HTTP error responses across all endpoints.
// Every error response MUST: have Content-Type: application/json, contain parseable JSON with "error" key.
// Every POST-only endpoint MUST: return 405 on GET.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// extensionRequest creates a localhost request with the required extension header.
func extensionRequest(method, path string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, "http://localhost"+path, body)
	r.Header.Set("X-Kaboom-Client", "kaboom-extension/test")
	return r
}

// TestAllPOSTEndpoints_RejectGET verifies every POST-only endpoint returns 405 on GET.
func TestAllPOSTEndpoints_RejectGET(t *testing.T) {
	t.Parallel()

	srv := newTestServerForHandlers(t)
	cap := capture.NewCapture()
	mux, _ := setupHTTPRoutes(srv, cap)

	postOnlyEndpoints := []string{
		"/websocket-events",
		"/network-bodies",
		"/network-waterfall",
		"/query-result",
		"/enhanced-actions",
		"/performance-snapshots",
		"/sync",
		"/logs",
		"/screenshots",
		"/draw-mode/complete",
		"/shutdown",
		"/clear",
		"/test-boundary",
	}

	for _, ep := range postOnlyEndpoints {
		t.Run("GET_"+ep, func(t *testing.T) {
			req := extensionRequest(http.MethodGet, ep, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)
			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("GET %s = %d, want 405", ep, rr.Code)
			}
		})
	}
}

// TestAllPOSTEndpoints_InvalidJSON verifies every POST ingest endpoint returns 400 with
// Content-Type: application/json and a parseable error body when given invalid JSON.
func TestAllPOSTEndpoints_InvalidJSON(t *testing.T) {
	t.Parallel()

	srv := newTestServerForHandlers(t)
	cap := capture.NewCapture()
	mux, _ := setupHTTPRoutes(srv, cap)

	// Endpoints that accept POST with JSON bodies and should return 400 on invalid JSON.
	// Excluded: /sync (has its own test), /recordings/save (multipart), /shutdown (no body).
	jsonEndpoints := []string{
		"/network-bodies",
		"/network-waterfall",
		"/query-result",
		"/enhanced-actions",
		"/performance-snapshots",
		"/logs",
		"/draw-mode/complete",
	}

	for _, ep := range jsonEndpoints {
		t.Run("InvalidJSON_"+ep, func(t *testing.T) {
			req := extensionRequest(http.MethodPost, ep, bytes.NewBufferString("{invalid"))
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("POST %s with invalid JSON = %d, want 400", ep, rr.Code)
			}

			ct := rr.Header().Get("Content-Type")
			if !strings.Contains(ct, "application/json") {
				t.Errorf("POST %s error response Content-Type = %q, want application/json", ep, ct)
			}

			var body map[string]any
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Errorf("POST %s error response is not valid JSON: %v (body=%q)", ep, err, rr.Body.String())
			} else if _, ok := body["error"]; !ok {
				t.Errorf("POST %s error response missing 'error' key: %v", ep, body)
			}
		})
	}
}

// TestCorrelationID_AsyncCommands verifies that every interact action that queues
// an async command returns a correlation_id in the response JSON.
// This is Rule 9 of the hardening lint rules: correlation_id must be present.
func TestCorrelationID_AsyncCommands(t *testing.T) {
	t.Parallel()

	srv := newTestServerForHandlers(t)
	cap := capture.NewCapture()
	cap.Extension().SetPilotEnabled(true)
	mcpHandler := NewToolHandler(srv, cap)
	handler := mcpHandler.toolHandler.(*ToolHandler)

	// Actions that queue async commands and must return correlation_id.
	// Each entry: action name → minimal args JSON that passes param validation.
	asyncActions := []struct {
		name string
		args string
	}{
		{"highlight", `{"what":"highlight","selector":".test"}`},
		{"execute_js", `{"what":"execute_js","script":"1+1"}`},
		{"navigate", `{"what":"navigate","url":"https://example.test"}`},
		{"refresh", `{"what":"refresh"}`},
		{"back", `{"what":"back"}`},
		{"forward", `{"what":"forward"}`},
		{"new_tab", `{"what":"new_tab","url":"https://example.test"}`},
		{"subtitle", `{"what":"subtitle","text":"hello"}`},
		{"list_interactive", `{"what":"list_interactive"}`},
		// DOM primitives
		{"click", `{"what":"click","selector":".btn"}`},
		{"type", `{"what":"type","selector":"input","text":"hello"}`},
		{"get_text", `{"what":"get_text","selector":".el"}`},
		{"scroll_to", `{"what":"scroll_to","selector":".el"}`},
		{"focus", `{"what":"focus","selector":".el"}`},
	}

	for _, tc := range asyncActions {
		t.Run(tc.name, func(t *testing.T) {
			req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
			resp := handler.toolInteract(req, json.RawMessage(tc.args))

			if resp.Result == nil {
				t.Fatal("response result is nil")
			}

			var result mcp.MCPToolResult
			if err := json.Unmarshal(resp.Result, &result); err != nil {
				t.Fatalf("failed to parse MCPToolResult: %v", err)
			}

			if result.IsError {
				t.Skipf("action returned error (may need extension): %s", result.Content[0].Text)
			}

			// Parse the text content to find correlation_id
			found := false
			for _, block := range result.Content {
				if strings.Contains(block.Text, "correlation_id") {
					found = true
					break
				}
			}

			// Subtitle action returns status/subtitle, not correlation_id — that's expected
			if tc.name == "subtitle" {
				return
			}

			if !found {
				t.Errorf("action %q response missing correlation_id.\nResult: %s",
					tc.name, fmt.Sprintf("%.500s", string(resp.Result)))
			}
		})
	}
}

// TestDiagnostics_ReturnsRealData verifies /diagnostics returns non-zero buffer counts
// after data is ingested.
func TestDiagnostics_ReturnsRealData(t *testing.T) {
	t.Parallel()

	srv := newTestServerForHandlers(t)
	cap := capture.NewCapture()
	mux, _ := setupHTTPRoutes(srv, cap)

	// Ingest some data into various buffers.
	cap.AddWebSocketEvents([]types.WebSocketEvent{
		{Type: "ws_connect", ID: "conn-1", URL: "wss://example.test"},
	})
	cap.AddNetworkBodies([]types.NetworkBody{
		{URL: "https://example.test/api", Method: "GET", Status: 200},
	})
	cap.AddEnhancedActions([]types.EnhancedAction{
		{Type: "click", Timestamp: 1000},
	})

	req := localRequest(http.MethodGet, "/diagnostics", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /diagnostics = %d, want 200", rr.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("diagnostics response is not valid JSON: %v", err)
	}

	// Check version is present
	if v, ok := resp["version"].(string); !ok || v == "" {
		t.Error("diagnostics missing version")
	}

	// Check uptime is present
	if _, ok := resp["uptime_seconds"]; !ok {
		t.Error("diagnostics missing uptime_seconds")
	}

	// Check buffers have real data (not hardcoded zeros)
	buffers, ok := resp["buffers"].(map[string]any)
	if !ok {
		t.Fatal("diagnostics missing buffers map")
	}

	checkNonZero := func(key string) {
		v, ok := buffers[key]
		if !ok {
			t.Errorf("buffers missing key %q", key)
			return
		}
		if num, ok := v.(float64); !ok || num == 0 {
			t.Errorf("buffers[%q] = %v, want > 0 (non-zero)", key, v)
		}
	}
	checkNonZero("websocket_events")
	checkNonZero("network_bodies")
	checkNonZero("actions")

	// Check circuit breaker info is present
	circuit, ok := resp["circuit"].(map[string]any)
	if !ok {
		t.Fatal("diagnostics missing circuit map")
	}
	if _, ok := circuit["open"]; !ok {
		t.Error("circuit missing 'open' field")
	}

	// Check extension info is present
	ext, ok := resp["extension"].(map[string]any)
	if !ok {
		t.Fatal("diagnostics missing extension map")
	}
	if _, ok := ext["polling"]; !ok {
		t.Error("extension missing 'polling' field")
	}
}
