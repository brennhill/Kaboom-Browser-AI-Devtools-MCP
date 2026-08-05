// websocket_handlers_test.go — Tests WebSocket HTTP ingestion and MCP responses.
// Docs: docs/features/feature/backend-log-streaming/index.md

package websockettest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/circuit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestV4PostWebSocketEventsEndpoint(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	body := `{"events":[{"ts":"2024-01-15T10:30:00.000Z","type":"websocket","event":"open","id":"uuid-1","url":"wss://example.com/ws"}]}`
	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	httpIngestForTest(capture).HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	if len(capture.Telemetry().WebSockets().Snapshot().Events) != 1 {
		t.Errorf("Expected 1 event stored, got %d", len(capture.Telemetry().WebSockets().Snapshot().Events))
	}
}

func TestV4PostWebSocketEventsInvalidJSON(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	httpIngestForTest(capture).HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rec.Code)
	}
}

func TestMCPGetWebSocketEvents(t *testing.T) {
	t.Parallel()
	cap := setupTestCapture(t)

	// Seed events that the MCP observe(websocket_events) layer would return.
	cap.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "ws-1", Event: "open", URL: "wss://chat.example.com/ws", Timestamp: "2024-01-15T10:30:00.000Z"},
		{ID: "ws-1", Event: "message", Direction: "incoming", Data: `{"msg":"hello"}`, Size: 15, URL: "wss://chat.example.com/ws", Timestamp: "2024-01-15T10:30:01.000Z"},
		{ID: "ws-1", Event: "message", Direction: "outgoing", Data: `{"msg":"world"}`, Size: 15, URL: "wss://chat.example.com/ws", Timestamp: "2024-01-15T10:30:02.000Z"},
	})

	// GetAllWebSocketEvents is the accessor the MCP layer calls.
	all := cap.Telemetry().WebSockets().Snapshot().Events
	if len(all) != 3 {
		t.Fatalf("expected 3 events, got %d", len(all))
	}

	// Verify events retain all fields the MCP response includes.
	if all[0].Event != "open" {
		t.Errorf("expected first event to be 'open', got %s", all[0].Event)
	}
	if all[2].Direction != "outgoing" {
		t.Errorf("expected last event direction 'outgoing', got %s", all[2].Direction)
	}
}

func TestMCPGetWebSocketEventsWithFilter(t *testing.T) {
	t.Parallel()
	cap := setupTestCapture(t)

	cap.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "ws-1", Event: "open", URL: "wss://chat.example.com/ws"},
		{ID: "ws-2", Event: "open", URL: "wss://feed.example.com/prices"},
		{ID: "ws-1", Event: "message", Direction: "incoming", URL: "wss://chat.example.com/ws"},
		{ID: "ws-2", Event: "message", Direction: "outgoing", URL: "wss://feed.example.com/prices"},
	})

	// Filter by connection_id (mirrors MCP args.connection_id).
	filtered := cap.Telemetry().WebSockets().Events(types.WebSocketEventFilter{ConnectionID: "ws-1"})
	if len(filtered) != 2 {
		t.Errorf("connection_id filter: expected 2 events, got %d", len(filtered))
	}

	// Filter by URL substring (mirrors MCP args.url).
	filtered = cap.Telemetry().WebSockets().Events(types.WebSocketEventFilter{URLFilter: "feed"})
	if len(filtered) != 2 {
		t.Errorf("url filter: expected 2 events, got %d", len(filtered))
	}

	// Filter by direction (mirrors MCP args.direction).
	filtered = cap.Telemetry().WebSockets().Events(types.WebSocketEventFilter{Direction: "outgoing"})
	if len(filtered) != 1 {
		t.Errorf("direction filter: expected 1 event, got %d", len(filtered))
	}

	// Combined filters: connection_id + direction.
	filtered = cap.Telemetry().WebSockets().Events(types.WebSocketEventFilter{ConnectionID: "ws-1", Direction: "incoming"})
	if len(filtered) != 1 {
		t.Errorf("combined filter: expected 1 event, got %d", len(filtered))
	}
}

func TestMCPGetWebSocketStatus(t *testing.T) {
	t.Parallel()
	cap := setupTestCapture(t)

	cap.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "ws-1", Event: "open", URL: "wss://chat.example.com/ws", Timestamp: "2024-01-15T10:30:00.000Z"},
		{ID: "ws-2", Event: "open", URL: "wss://feed.example.com/prices", Timestamp: "2024-01-15T10:30:01.000Z"},
		{ID: "ws-1", Event: "message", Direction: "incoming", Size: 100, Timestamp: "2024-01-15T10:30:02.000Z"},
	})

	// GetWebSocketStatus is the accessor the MCP observe(websocket_status) layer calls.
	status := cap.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{})

	if len(status.Connections) != 2 {
		t.Fatalf("expected 2 active connections, got %d", len(status.Connections))
	}
	if len(status.Closed) != 0 {
		t.Errorf("expected 0 closed connections, got %d", len(status.Closed))
	}

	// Verify connection details match MCP response shape.
	found := false
	for _, conn := range status.Connections {
		if conn.ID == "ws-1" {
			found = true
			if conn.URL != "wss://chat.example.com/ws" {
				t.Errorf("expected URL wss://chat.example.com/ws, got %s", conn.URL)
			}
			if conn.MessageRate.Incoming.Total != 1 {
				t.Errorf("expected 1 incoming message, got %d", conn.MessageRate.Incoming.Total)
			}
		}
	}
	if !found {
		t.Error("expected to find connection ws-1")
	}
}

func TestMCPGetWebSocketEventsEmpty(t *testing.T) {
	t.Parallel()
	cap := setupTestCapture(t)

	// No events added — mirrors MCP observe(websocket_events) on fresh capture.
	all := cap.Telemetry().WebSockets().Snapshot().Events
	if len(all) != 0 {
		t.Errorf("expected 0 events on fresh capture, got %d", len(all))
	}

	filtered := cap.Telemetry().WebSockets().Events(types.WebSocketEventFilter{})
	if len(filtered) != 0 {
		t.Errorf("expected 0 filtered events on fresh capture, got %d", len(filtered))
	}

	status := cap.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{})
	if len(status.Connections) != 0 {
		t.Errorf("expected 0 connections on fresh capture, got %d", len(status.Connections))
	}
	if len(status.Closed) != 0 {
		t.Errorf("expected 0 closed on fresh capture, got %d", len(status.Closed))
	}
}

// ============================================
// HandleWebSocketStatus: HTTP GET handler
// ============================================

// Test: HandleWebSocketStatus returns JSON with connections and closed arrays.
func TestV4HandleWebSocketStatus_EmptyState(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	req := httptest.NewRequest("GET", "/websocket-status", nil)
	rec := httptest.NewRecorder()

	httpIngestForTest(capture).HandleWebSocketStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	var status types.WebSocketStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	if status.Connections == nil {
		t.Error("expected non-nil Connections slice")
	}
	if status.Closed == nil {
		t.Error("expected non-nil Closed slice")
	}
}

// Test: HandleWebSocketStatus returns open connections.
func TestV4HandleWebSocketStatus_WithConnections(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "ws-1", Event: "open", URL: "wss://chat.example.com/ws", Timestamp: time.Now().Format(time.RFC3339Nano)},
		{ID: "ws-2", Event: "open", URL: "wss://feed.example.com/prices", Timestamp: time.Now().Format(time.RFC3339Nano)},
	})

	req := httptest.NewRequest("GET", "/websocket-status", nil)
	rec := httptest.NewRecorder()

	httpIngestForTest(capture).HandleWebSocketStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var status types.WebSocketStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}

	if len(status.Connections) != 2 {
		t.Errorf("expected 2 connections, got %d", len(status.Connections))
	}
}

// Test: HandleWebSocketStatus returns closed connections.
func TestV4HandleWebSocketStatus_WithClosedConnections(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "ws-closed", Event: "open", URL: "wss://example.com/ws"},
		{ID: "ws-closed", Event: "close", URL: "wss://example.com/ws", CloseCode: 1001, CloseReason: "going away"},
	})

	req := httptest.NewRequest("GET", "/websocket-status", nil)
	rec := httptest.NewRecorder()

	httpIngestForTest(capture).HandleWebSocketStatus(rec, req)

	var status types.WebSocketStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if len(status.Connections) != 0 {
		t.Errorf("expected 0 open connections, got %d", len(status.Connections))
	}
	if len(status.Closed) != 1 {
		t.Errorf("expected 1 closed connection, got %d", len(status.Closed))
	}
	if status.Closed[0].CloseCode != 1001 {
		t.Errorf("expected close code 1001, got %d", status.Closed[0].CloseCode)
	}
}

// ============================================
// toolGetWSStatus: connection_id and url filters
// ============================================

// Test: toolGetWSStatus with connection_id filter.
func TestV4ToolGetWSStatus_ConnectionIDFilter(t *testing.T) {
	t.Parallel()
	cap := setupTestCapture(t)

	cap.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "ws-a", Event: "open", URL: "wss://a.example.com/ws"},
		{ID: "ws-b", Event: "open", URL: "wss://b.example.com/ws"},
		{ID: "ws-c", Event: "open", URL: "wss://c.example.com/ws"},
	})

	status := cap.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{ConnectionID: "ws-b"})

	if len(status.Connections) != 1 {
		t.Fatalf("expected 1 connection with connection_id filter, got %d", len(status.Connections))
	}
	if status.Connections[0].ID != "ws-b" {
		t.Errorf("expected connection ws-b, got %s", status.Connections[0].ID)
	}
}

// Test: toolGetWSStatus with url filter.
func TestV4ToolGetWSStatus_URLFilter(t *testing.T) {
	t.Parallel()
	cap := setupTestCapture(t)

	cap.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "ws-1", Event: "open", URL: "wss://chat.example.com/ws"},
		{ID: "ws-2", Event: "open", URL: "wss://feed.example.com/prices"},
		{ID: "ws-3", Event: "open", URL: "wss://chat.example.com/live"},
	})

	status := cap.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{URLFilter: "chat"})

	if len(status.Connections) != 2 {
		t.Fatalf("expected 2 connections matching 'chat', got %d", len(status.Connections))
	}
	for _, conn := range status.Connections {
		if !strings.Contains(conn.URL, "chat") {
			t.Errorf("expected URL containing 'chat', got %s", conn.URL)
		}
	}
}

// Test: toolGetWSStatus with both connection_id and url filter (connection_id takes precedence).
func TestV4ToolGetWSStatus_BothFilters(t *testing.T) {
	t.Parallel()
	cap := setupTestCapture(t)

	cap.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "ws-1", Event: "open", URL: "wss://chat.example.com/ws"},
		{ID: "ws-2", Event: "open", URL: "wss://chat.example.com/live"},
		{ID: "ws-3", Event: "open", URL: "wss://feed.example.com/prices"},
	})

	// When both filters are set, both should apply (connection_id narrows first).
	status := cap.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{
		ConnectionID: "ws-1",
		URLFilter:    "chat",
	})

	if len(status.Connections) != 1 {
		t.Fatalf("expected 1 connection with both filters, got %d", len(status.Connections))
	}
	if status.Connections[0].ID != "ws-1" {
		t.Errorf("expected connection ws-1, got %s", status.Connections[0].ID)
	}
}

// ============================================
// HandleWebSocketEvents: GET handler (returning events)
// ============================================

// Test: HandleWebSocketEvents rejects GET (reads go through /telemetry).
func TestV4HandleWebSocketEvents_RejectsGET(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	req := httptest.NewRequest("GET", "/websocket-events", nil)
	rec := httptest.NewRecorder()

	httpIngestForTest(capture).HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

// ============================================
// HandleWebSocketEvents: POST rate limiting and body size
// ============================================

// Test: HandleWebSocketEvents POST rejected when rate limited.
func TestV4HandleWebSocketEvents_POST_RateLimited(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.Circuit().ForceOpen("rate_exceeded")

	body := `{"events":[{"event":"open","id":"ws-1","url":"wss://example.com/ws"}]}`
	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	httpIngestForTest(capture).HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
}

// Test: HandleWebSocketEvents POST rejected when body too large.
func TestV4HandleWebSocketEvents_POST_BodyTooLarge(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	largePayload := strings.Repeat("x", 6*1024*1024)
	req := httptest.NewRequest("POST", "/websocket-events", strings.NewReader(largePayload))
	rec := httptest.NewRecorder()

	httpIngestForTest(capture).HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", rec.Code)
	}
}

// Test: HandleWebSocketEvents POST rejected when bad JSON.
func TestV4HandleWebSocketEvents_POST_BadJSON(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewBufferString("not json!"))
	rec := httptest.NewRecorder()

	httpIngestForTest(capture).HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad JSON, got %d", rec.Code)
	}
}

// Test: HandleWebSocketEvents POST re-check rate limit after recording.
func TestV4HandleWebSocketEvents_POST_RateLimitAfterRecording(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.Circuit().SetWindowState(time.Now(), circuit.RateLimitThreshold-1)

	// 10 events pushes count over threshold
	events := make([]map[string]any, 10)
	for i := range events {
		events[i] = map[string]any{
			"event":     "message",
			"id":        "ws-1",
			"direction": "incoming",
		}
	}
	payload, _ := json.Marshal(map[string]any{"events": events})

	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewReader(payload))
	rec := httptest.NewRecorder()

	httpIngestForTest(capture).HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 after recording pushes over threshold, got %d", rec.Code)
	}
}

// ============================================
// Additional coverage: toolGetWSStatus parse error
// ============================================

// Test: toolGetWSStatus with invalid arguments — GetWebSocketStatus gracefully
// handles empty/default filters (the MCP layer handles JSON parse errors itself,
// so at the capture level we verify that zero-value filters return valid results).
func TestV4ToolGetWSStatus_InvalidArgs(t *testing.T) {
	t.Parallel()
	cap := setupTestCapture(t)

	// Seed a connection so we can verify default filter returns it.
	cap.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "ws-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Zero-value filter (what the MCP layer falls back to on parse error).
	status := cap.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{})
	if len(status.Connections) != 1 {
		t.Fatalf("expected 1 connection with zero-value filter, got %d", len(status.Connections))
	}

	// Filter with non-matching connection_id returns empty (not an error).
	status = cap.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{ConnectionID: "nonexistent"})
	if len(status.Connections) != 0 {
		t.Errorf("expected 0 connections for non-matching ID, got %d", len(status.Connections))
	}

	// Filter with non-matching URL returns empty (not an error).
	status = cap.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{URLFilter: "nonexistent"})
	if len(status.Connections) != 0 {
		t.Errorf("expected 0 connections for non-matching URL, got %d", len(status.Connections))
	}
}
