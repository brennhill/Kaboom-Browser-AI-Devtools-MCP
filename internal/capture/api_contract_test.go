// Purpose: Tests for capture API contract compliance.
// Docs: docs/features/feature/backend-log-streaming/index.md

// api_contract_test.go — API contract tests for Extension ↔ Server communication.
// These tests verify that HTTP endpoints accept the correct methods and payloads.
//
// ARCHITECTURAL INVARIANT: These tests MUST pass. If they fail, the extension
// cannot communicate with the server and the product is broken.
//
// Modification of these tests requires principal review.
package capture

import (
	"bytes"
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func postNetworkBodies(t *testing.T, c *Capture, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal payload failed: %v", err)
	}
	req := httptest.NewRequest("POST", "/network-bodies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c.HandleNetworkBodies(w, req)
	return w
}

// ============================================
// Contract: POST Endpoints (Extension → Server)
// ============================================

func TestAPIContract_NetworkBodies_AcceptsPOST(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	payload := map[string]any{
		"bodies": []map[string]any{
			{"url": "https://api.example.com/users", "method": "GET", "status": 200},
		},
	}
	w := postNetworkBodies(t, c, payload)

	if w.Code != http.StatusOK {
		t.Errorf("POST /network-bodies should return 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", resp["status"])
	}
}

func TestAPIContract_NetworkBodies_RejectsGET(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	req := httptest.NewRequest("GET", "/network-bodies", nil)
	w := httptest.NewRecorder()

	c.HandleNetworkBodies(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /network-bodies should return 405, got %d", w.Code)
	}
}

func TestAPIContract_EnhancedActions_AcceptsPOST(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	payload := map[string]any{
		"actions": []map[string]any{
			{"type": "click", "timestamp": time.Now().UnixMilli(), "url": "https://example.com"},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/enhanced-actions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c.HandleEnhancedActions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /enhanced-actions should return 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", resp["status"])
	}
}

func TestAPIContract_EnhancedActions_RejectsGET(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	req := httptest.NewRequest("GET", "/enhanced-actions", nil)
	w := httptest.NewRecorder()

	c.HandleEnhancedActions(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /enhanced-actions should return 405, got %d", w.Code)
	}
}

func TestAPIContract_PerformanceSnapshots_AcceptsPOST(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	payload := map[string]any{
		"snapshots": []map[string]any{
			{"url": "https://example.com", "timestamp": time.Now().Format(time.RFC3339)},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/performance-snapshots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c.HandlePerformanceSnapshots(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /performance-snapshots should return 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", resp["status"])
	}
}

func TestAPIContract_PerformanceSnapshots_RejectsGET(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	req := httptest.NewRequest("GET", "/performance-snapshots", nil)
	w := httptest.NewRecorder()

	c.HandlePerformanceSnapshots(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /performance-snapshots should return 405, got %d", w.Code)
	}
}

func TestAPIContract_AddExtensionLogs(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	c.AddExtensionLogs([]types.ExtensionLog{
		{Level: "debug", Message: "test", Source: "background"},
	})

	logs := c.GetExtensionLogs()
	if len(logs) != 1 {
		t.Errorf("AddExtensionLogs should store 1 log, got %d", len(logs))
	}
}

func TestAPIContract_NetworkWaterfall_AcceptsPOST(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	payload := map[string]any{
		"entries":  []map[string]any{{"url": "https://example.com/script.js"}},
		"page_url": "https://example.com",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c.HandleNetworkWaterfall(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /network-waterfall should return 200, got %d", w.Code)
	}
}

func TestAPIContract_NetworkWaterfall_RejectsGET(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	req := httptest.NewRequest("GET", "/network-waterfall", nil)
	w := httptest.NewRecorder()

	c.HandleNetworkWaterfall(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /network-waterfall should return 405, got %d", w.Code)
	}
}

func TestAPIContract_WebSocketEvents_AcceptsPOST(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	payload := map[string]any{
		"events": []map[string]any{
			{"type": "ws_message", "url": "wss://example.com/socket", "data": "test"},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c.HandleWebSocketEvents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /websocket-events should return 200, got %d", w.Code)
	}
}

func TestAPIContract_WebSocketEvents_RejectsGET(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	req := httptest.NewRequest("GET", "/websocket-events", nil)
	w := httptest.NewRecorder()

	c.HandleWebSocketEvents(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /websocket-events should return 405, got %d", w.Code)
	}
}

// ============================================
// Contract: Unified Query Result Endpoint (Extension → Server)
// ============================================

func TestAPIContract_QueryResult_AcceptsPOST(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	payload := map[string]any{
		"id":     "q-123",
		"result": map[string]any{"elements": []any{}},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/query-result", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c.HandleQueryResult(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /query-result should return 200, got %d", w.Code)
	}
}

func TestAPIContract_QueryResult_RejectsGET(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	req := httptest.NewRequest("GET", "/query-result", nil)
	w := httptest.NewRecorder()

	c.HandleQueryResult(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /query-result should return 405, got %d", w.Code)
	}
}

func TestAPIContract_QueryResult_WithCorrelationID(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	payload := map[string]any{
		"id":             "q-123",
		"correlation_id": "corr-456",
		"status":         "complete",
		"result":         map[string]any{"success": true},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/query-result", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c.HandleQueryResult(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /query-result with correlation_id should return 200, got %d", w.Code)
	}
}

// ============================================
// Contract: Data Flow Verification
// ============================================

func TestAPIContract_EnhancedActions_POSTThenRead(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// POST some actions
	payload := map[string]any{
		"actions": []map[string]any{
			{"type": "click", "timestamp": time.Now().UnixMilli(), "url": "https://example.com"},
			{"type": "input", "timestamp": time.Now().UnixMilli(), "url": "https://example.com"},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/enhanced-actions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c.HandleEnhancedActions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST failed with %d", w.Code)
	}

	// Read back via getter
	actions := c.GetAllEnhancedActions()
	if len(actions) != 2 {
		t.Errorf("Expected 2 actions after POST, got %d", len(actions))
	}
}

func TestAPIContract_PerformanceSnapshots_POSTThenRead(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// POST some snapshots
	payload := map[string]any{
		"snapshots": []map[string]any{
			{"url": "https://example.com/page1", "timestamp": time.Now().Format(time.RFC3339)},
			{"url": "https://example.com/page2", "timestamp": time.Now().Format(time.RFC3339)},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/performance-snapshots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c.HandlePerformanceSnapshots(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST failed with %d", w.Code)
	}

	// Read back via getter
	snapshots := c.GetPerformanceSnapshots()
	if len(snapshots) != 2 {
		t.Errorf("Expected 2 snapshots after POST, got %d", len(snapshots))
	}
}

func TestAPIContract_NetworkBodies_POSTThenRead(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// POST some bodies
	payload := map[string]any{
		"bodies": []map[string]any{
			{"url": "https://api.example.com/users", "method": "GET", "status": 200},
		},
	}
	w := postNetworkBodies(t, c, payload)

	if w.Code != http.StatusOK {
		t.Fatalf("POST failed with %d", w.Code)
	}

	// Read back via getter
	bodies := c.GetNetworkBodies()
	if len(bodies) != 1 {
		t.Errorf("Expected 1 body after POST, got %d", len(bodies))
	}
}
