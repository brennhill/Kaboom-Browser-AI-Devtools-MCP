// Purpose: Tests for WebSocket frame capture and event storage.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"bytes"
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestV4WebSocketEventBuffer(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	events := []types.WebSocketEvent{
		{Timestamp: "2024-01-15T10:30:00.000Z", Type: "websocket", Event: "open", ID: "uuid-1", URL: "wss://example.com/ws"},
		{Timestamp: "2024-01-15T10:30:01.000Z", Type: "websocket", Event: "message", ID: "uuid-1", Direction: "incoming", Data: `{"type":"chat","msg":"hello"}`, Size: 32},
	}

	capture.AddWebSocketEvents(events)

	if capture.GetWebSocketEventCount() != 2 {
		t.Errorf("Expected 2 events, got %d", capture.GetWebSocketEventCount())
	}
}

func TestV4WebSocketEventBufferRotation(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Add more than max (500) events
	events := make([]types.WebSocketEvent, 550)
	for i := range events {
		events[i] = types.WebSocketEvent{
			Timestamp: "2024-01-15T10:30:00.000Z",
			Type:      "websocket",
			Event:     "message",
			ID:        "uuid-1",
			Data:      `{"i":` + string(rune(i)) + `}`,
		}
	}

	capture.AddWebSocketEvents(events)

	if capture.GetWebSocketEventCount() != 500 {
		t.Errorf("Expected 500 events after rotation, got %d", capture.GetWebSocketEventCount())
	}
}

func TestV4WebSocketEventFilterByConnectionID(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://a.com"},
		{ID: "uuid-2", Event: "open", URL: "wss://b.com"},
		{ID: "uuid-1", Event: "message", Direction: "incoming"},
	})

	filtered := capture.GetWebSocketEvents(types.WebSocketEventFilter{ConnectionID: "uuid-1"})

	if len(filtered) != 2 {
		t.Errorf("Expected 2 events for uuid-1, got %d", len(filtered))
	}
}

func TestV4WebSocketEventFilterByURL(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://chat.example.com/ws"},
		{ID: "uuid-2", Event: "open", URL: "wss://feed.example.com/prices"},
		{ID: "uuid-1", Event: "message", URL: "wss://chat.example.com/ws"},
	})

	filtered := capture.GetWebSocketEvents(types.WebSocketEventFilter{URLFilter: "chat"})

	if len(filtered) != 2 {
		t.Errorf("Expected 2 events matching 'chat', got %d", len(filtered))
	}
}

func TestV4WebSocketEventFilterByDirection(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "uuid-1", Event: "message", Direction: "incoming"},
		{ID: "uuid-1", Event: "message", Direction: "outgoing"},
		{ID: "uuid-1", Event: "message", Direction: "incoming"},
	})

	filtered := capture.GetWebSocketEvents(types.WebSocketEventFilter{Direction: "incoming"})

	if len(filtered) != 2 {
		t.Errorf("Expected 2 incoming events, got %d", len(filtered))
	}
}

func TestV4WebSocketEventFilterWithLimit(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	for i := 0; i < 10; i++ {
		capture.AddWebSocketEvents([]types.WebSocketEvent{
			{ID: "uuid-1", Event: "message", Direction: "incoming"},
		})
	}

	filtered := capture.GetWebSocketEvents(types.WebSocketEventFilter{Limit: 5})

	if len(filtered) != 5 {
		t.Errorf("Expected 5 events with limit, got %d", len(filtered))
	}
}

func TestV4WebSocketEventDefaultLimit(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	for i := 0; i < 100; i++ {
		capture.AddWebSocketEvents([]types.WebSocketEvent{
			{ID: "uuid-1", Event: "message"},
		})
	}

	// Default limit is 50
	filtered := capture.GetWebSocketEvents(types.WebSocketEventFilter{})

	if len(filtered) != 50 {
		t.Errorf("Expected 50 events with default limit, got %d", len(filtered))
	}
}

func TestV4WebSocketEventNewestFirst(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{Timestamp: "2024-01-15T10:30:00.000Z", ID: "uuid-1", Event: "open"},
		{Timestamp: "2024-01-15T10:30:05.000Z", ID: "uuid-1", Event: "close"},
	})

	filtered := capture.GetWebSocketEvents(types.WebSocketEventFilter{})

	if len(filtered) == 0 {
		t.Fatal("Expected events to be returned")
	}
	if filtered[0].Timestamp != "2024-01-15T10:30:05.000Z" {
		t.Errorf("Expected newest first, got %s", filtered[0].Timestamp)
	}
}

func TestV4WebSocketConnectionTracker(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://chat.example.com/ws", Timestamp: "2024-01-15T10:30:00.000Z"},
	})

	status := capture.GetWebSocketStatus(types.WebSocketStatusFilter{})

	if len(status.Connections) != 1 {
		t.Fatalf("Expected 1 open connection, got %d", len(status.Connections))
	}

	if status.Connections[0].State != "open" {
		t.Errorf("Expected state 'open', got %s", status.Connections[0].State)
	}

	if status.Connections[0].URL != "wss://chat.example.com/ws" {
		t.Errorf("Expected URL 'wss://chat.example.com/ws', got %s", status.Connections[0].URL)
	}
}

func TestV4WebSocketConnectionClose(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "uuid-1", Event: "close", URL: "wss://example.com/ws", CloseCode: 1000, CloseReason: "normal closure"},
	})

	status := capture.GetWebSocketStatus(types.WebSocketStatusFilter{})

	if len(status.Connections) != 0 {
		t.Errorf("Expected 0 open connections, got %d", len(status.Connections))
	}

	if len(status.Closed) != 1 {
		t.Fatalf("Expected 1 closed connection, got %d", len(status.Closed))
	}

	if status.Closed[0].CloseCode != 1000 {
		t.Errorf("Expected close code 1000, got %d", status.Closed[0].CloseCode)
	}
}

func TestV4WebSocketConnectionError(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "uuid-1", Event: "error", URL: "wss://example.com/ws"},
	})

	status := capture.GetWebSocketStatus(types.WebSocketStatusFilter{})

	if len(status.Connections) != 1 {
		t.Fatalf("Expected 1 connection (in error state), got %d", len(status.Connections))
	}

	if status.Connections[0].State != "error" {
		t.Errorf("Expected state 'error', got %s", status.Connections[0].State)
	}
}

func TestV4WebSocketConnectionMessageStats(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "uuid-1", Event: "message", Direction: "incoming", Size: 100},
		{ID: "uuid-1", Event: "message", Direction: "incoming", Size: 200},
		{ID: "uuid-1", Event: "message", Direction: "outgoing", Size: 50},
	})

	status := capture.GetWebSocketStatus(types.WebSocketStatusFilter{})

	if len(status.Connections) != 1 {
		t.Fatalf("Expected 1 connection, got %d", len(status.Connections))
	}

	conn := status.Connections[0]
	if conn.MessageRate.Incoming.Total != 2 {
		t.Errorf("Expected 2 incoming messages, got %d", conn.MessageRate.Incoming.Total)
	}

	if conn.MessageRate.Incoming.Bytes != 300 {
		t.Errorf("Expected 300 incoming bytes, got %d", conn.MessageRate.Incoming.Bytes)
	}

	if conn.MessageRate.Outgoing.Total != 1 {
		t.Errorf("Expected 1 outgoing message, got %d", conn.MessageRate.Outgoing.Total)
	}

	if conn.MessageRate.Outgoing.Bytes != 50 {
		t.Errorf("Expected 50 outgoing bytes, got %d", conn.MessageRate.Outgoing.Bytes)
	}
}

func TestV4WebSocketConnectionLastMessage(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "uuid-1", Event: "message", Direction: "incoming", Data: `{"type":"hello"}`, Timestamp: "2024-01-15T10:30:01.000Z"},
		{ID: "uuid-1", Event: "message", Direction: "incoming", Data: `{"type":"world"}`, Timestamp: "2024-01-15T10:30:02.000Z"},
	})

	status := capture.GetWebSocketStatus(types.WebSocketStatusFilter{})
	conn := status.Connections[0]

	if conn.LastMessage.Incoming.Preview != `{"type":"world"}` {
		t.Errorf("Expected last incoming preview to be world message, got %s", conn.LastMessage.Incoming.Preview)
	}
}

func TestV4WebSocketMaxTrackedConnections(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Open 25 connections (max is 20 active)
	for i := 0; i < 25; i++ {
		capture.AddWebSocketEvents([]types.WebSocketEvent{
			{ID: "uuid-" + string(rune('a'+i)), Event: "open", URL: "wss://example.com/ws"},
		})
	}

	status := capture.GetWebSocketStatus(types.WebSocketStatusFilter{})

	if len(status.Connections) > 20 {
		t.Errorf("Expected max 20 active connections, got %d", len(status.Connections))
	}
}

func TestV4WebSocketClosedConnectionHistory(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Open and close 15 connections (max closed history is 10)
	for i := 0; i < 15; i++ {
		id := "uuid-" + strings.Repeat("x", i+1) // unique IDs
		capture.AddWebSocketEvents([]types.WebSocketEvent{
			{ID: id, Event: "open", URL: "wss://example.com/ws"},
			{ID: id, Event: "close", URL: "wss://example.com/ws", CloseCode: 1000},
		})
	}

	status := capture.GetWebSocketStatus(types.WebSocketStatusFilter{})

	if len(status.Closed) > 10 {
		t.Errorf("Expected max 10 closed connections in history, got %d", len(status.Closed))
	}
}

func TestV4WebSocketStatusFilterByURL(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://chat.example.com/ws"},
		{ID: "uuid-2", Event: "open", URL: "wss://feed.example.com/prices"},
	})

	status := capture.GetWebSocketStatus(types.WebSocketStatusFilter{URLFilter: "chat"})

	if len(status.Connections) != 1 {
		t.Errorf("Expected 1 connection matching 'chat', got %d", len(status.Connections))
	}
}

func TestV4WebSocketStatusFilterByConnectionID(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://a.com"},
		{ID: "uuid-2", Event: "open", URL: "wss://b.com"},
	})

	status := capture.GetWebSocketStatus(types.WebSocketStatusFilter{ConnectionID: "uuid-2"})

	if len(status.Connections) != 1 {
		t.Errorf("Expected 1 connection, got %d", len(status.Connections))
	}

	if status.Connections[0].ID != "uuid-2" {
		t.Errorf("Expected connection uuid-2, got %s", status.Connections[0].ID)
	}
}

func TestV4WebSocketSamplingInfo(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "uuid-1", Event: "message", Direction: "incoming", Sampled: &types.SamplingInfo{Rate: "48.2/s", Logged: "1/5", Window: "5s"}},
	})

	status := capture.GetWebSocketStatus(types.WebSocketStatusFilter{})
	conn := status.Connections[0]

	if !conn.Sampling.Active {
		t.Error("Expected sampling to be active")
	}
}

func TestV4PostWebSocketEventsEndpoint(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	body := `{"events":[{"ts":"2024-01-15T10:30:00.000Z","type":"websocket","event":"open","id":"uuid-1","url":"wss://example.com/ws"}]}`
	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	if capture.GetWebSocketEventCount() != 1 {
		t.Errorf("Expected 1 event stored, got %d", capture.GetWebSocketEventCount())
	}
}

func TestV4PostWebSocketEventsInvalidJSON(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rec.Code)
	}
}

func TestMCPGetWebSocketEvents(t *testing.T) {
	t.Parallel()
	cap := setupTestCapture(t)

	// Seed events that the MCP observe(websocket_events) layer would return.
	cap.AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "ws-1", Event: "open", URL: "wss://chat.example.com/ws", Timestamp: "2024-01-15T10:30:00.000Z"},
		{ID: "ws-1", Event: "message", Direction: "incoming", Data: `{"msg":"hello"}`, Size: 15, URL: "wss://chat.example.com/ws", Timestamp: "2024-01-15T10:30:01.000Z"},
		{ID: "ws-1", Event: "message", Direction: "outgoing", Data: `{"msg":"world"}`, Size: 15, URL: "wss://chat.example.com/ws", Timestamp: "2024-01-15T10:30:02.000Z"},
	})

	// GetAllWebSocketEvents is the accessor the MCP layer calls.
	all := cap.GetAllWebSocketEvents()
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

	cap.AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "ws-1", Event: "open", URL: "wss://chat.example.com/ws"},
		{ID: "ws-2", Event: "open", URL: "wss://feed.example.com/prices"},
		{ID: "ws-1", Event: "message", Direction: "incoming", URL: "wss://chat.example.com/ws"},
		{ID: "ws-2", Event: "message", Direction: "outgoing", URL: "wss://feed.example.com/prices"},
	})

	// Filter by connection_id (mirrors MCP args.connection_id).
	filtered := cap.GetWebSocketEvents(types.WebSocketEventFilter{ConnectionID: "ws-1"})
	if len(filtered) != 2 {
		t.Errorf("connection_id filter: expected 2 events, got %d", len(filtered))
	}

	// Filter by URL substring (mirrors MCP args.url).
	filtered = cap.GetWebSocketEvents(types.WebSocketEventFilter{URLFilter: "feed"})
	if len(filtered) != 2 {
		t.Errorf("url filter: expected 2 events, got %d", len(filtered))
	}

	// Filter by direction (mirrors MCP args.direction).
	filtered = cap.GetWebSocketEvents(types.WebSocketEventFilter{Direction: "outgoing"})
	if len(filtered) != 1 {
		t.Errorf("direction filter: expected 1 event, got %d", len(filtered))
	}

	// Combined filters: connection_id + direction.
	filtered = cap.GetWebSocketEvents(types.WebSocketEventFilter{ConnectionID: "ws-1", Direction: "incoming"})
	if len(filtered) != 1 {
		t.Errorf("combined filter: expected 1 event, got %d", len(filtered))
	}
}

func TestMCPGetWebSocketStatus(t *testing.T) {
	t.Parallel()
	cap := setupTestCapture(t)

	cap.AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "ws-1", Event: "open", URL: "wss://chat.example.com/ws", Timestamp: "2024-01-15T10:30:00.000Z"},
		{ID: "ws-2", Event: "open", URL: "wss://feed.example.com/prices", Timestamp: "2024-01-15T10:30:01.000Z"},
		{ID: "ws-1", Event: "message", Direction: "incoming", Size: 100, Timestamp: "2024-01-15T10:30:02.000Z"},
	})

	// GetWebSocketStatus is the accessor the MCP observe(websocket_status) layer calls.
	status := cap.GetWebSocketStatus(types.WebSocketStatusFilter{})

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
	all := cap.GetAllWebSocketEvents()
	if len(all) != 0 {
		t.Errorf("expected 0 events on fresh capture, got %d", len(all))
	}

	filtered := cap.GetWebSocketEvents(types.WebSocketEventFilter{})
	if len(filtered) != 0 {
		t.Errorf("expected 0 filtered events on fresh capture, got %d", len(filtered))
	}

	status := cap.GetWebSocketStatus(types.WebSocketStatusFilter{})
	if len(status.Connections) != 0 {
		t.Errorf("expected 0 connections on fresh capture, got %d", len(status.Connections))
	}
	if len(status.Closed) != 0 {
		t.Errorf("expected 0 closed on fresh capture, got %d", len(status.Closed))
	}
}

func TestV4ConnectionDurationFormatted(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	openedAt := time.Now().Add(-5*time.Minute - 2*time.Second)
	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{
			Timestamp: openedAt.Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "open",
			URL:       "wss://example.com/ws",
		},
	})

	status := capture.GetWebSocketStatus(types.WebSocketStatusFilter{})
	if len(status.Connections) != 1 {
		t.Fatalf("Expected 1 connection, got %d", len(status.Connections))
	}

	conn := status.Connections[0]
	if conn.Duration == "" {
		t.Fatal("Expected Duration to be set for active connection")
	}

	// Duration should be approximately "5m02s" (give or take a second)
	if !strings.Contains(conn.Duration, "m") {
		t.Errorf("Expected duration to contain 'm' for minutes, got: %s", conn.Duration)
	}
}

func TestV4ConnectionDurationShortFormat(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	openedAt := time.Now().Add(-3 * time.Second)
	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{
			Timestamp: openedAt.Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "open",
			URL:       "wss://example.com/ws",
		},
	})

	status := capture.GetWebSocketStatus(types.WebSocketStatusFilter{})
	conn := status.Connections[0]

	// Should be "3s" or "4s" (within test timing tolerance)
	if !strings.HasSuffix(conn.Duration, "s") {
		t.Errorf("Expected short duration ending in 's', got: %s", conn.Duration)
	}
}

func TestV4ConnectionDurationHourFormat(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	openedAt := time.Now().Add(-1*time.Hour - 15*time.Minute)
	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{
			Timestamp: openedAt.Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "open",
			URL:       "wss://example.com/ws",
		},
	})

	status := capture.GetWebSocketStatus(types.WebSocketStatusFilter{})
	conn := status.Connections[0]

	if !strings.Contains(conn.Duration, "h") {
		t.Errorf("Expected duration with 'h' for hours, got: %s", conn.Duration)
	}
}

func TestV4MessageRateCalculation(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Open connection
	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{Timestamp: time.Now().Add(-10 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Send 10 messages over the last 5 seconds (2 per second)
	now := time.Now()
	for i := 0; i < 10; i++ {
		ts := now.Add(-5*time.Second + time.Duration(i)*500*time.Millisecond)
		capture.AddWebSocketEvents([]types.WebSocketEvent{
			{Timestamp: ts.Format(time.RFC3339Nano), ID: "uuid-1", Event: "message", Direction: "incoming", Size: 100},
		})
	}

	status := capture.GetWebSocketStatus(types.WebSocketStatusFilter{})
	if len(status.Connections) != 1 {
		t.Fatalf("Expected 1 connection, got %d", len(status.Connections))
	}

	conn := status.Connections[0]
	// Rate should be approximately 2.0 msg/s (10 messages in 5 seconds)
	if conn.MessageRate.Incoming.PerSecond < 1.0 {
		t.Errorf("Expected incoming rate >= 1.0 msg/s, got %.2f", conn.MessageRate.Incoming.PerSecond)
	}
	if conn.MessageRate.Incoming.PerSecond > 5.0 {
		t.Errorf("Expected incoming rate <= 5.0 msg/s, got %.2f", conn.MessageRate.Incoming.PerSecond)
	}
}

func TestV4MessageRateZeroWhenNoRecentMessages(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Open connection long ago
	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{Timestamp: time.Now().Add(-60 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Send messages long ago (outside 5-second window)
	oldTime := time.Now().Add(-30 * time.Second)
	for i := 0; i < 5; i++ {
		capture.AddWebSocketEvents([]types.WebSocketEvent{
			{Timestamp: oldTime.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "message", Direction: "incoming", Size: 50},
		})
	}

	status := capture.GetWebSocketStatus(types.WebSocketStatusFilter{})
	conn := status.Connections[0]

	// Rate should be 0 since all messages are outside the 5-second window
	if conn.MessageRate.Incoming.PerSecond != 0.0 {
		t.Errorf("Expected incoming rate 0 for old messages, got %.2f", conn.MessageRate.Incoming.PerSecond)
	}
}

func TestV4MessageRateOutgoing(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{Timestamp: time.Now().Add(-10 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Send 5 outgoing messages in last 5 seconds
	now := time.Now()
	for i := 0; i < 5; i++ {
		ts := now.Add(-4*time.Second + time.Duration(i)*time.Second)
		capture.AddWebSocketEvents([]types.WebSocketEvent{
			{Timestamp: ts.Format(time.RFC3339Nano), ID: "uuid-1", Event: "message", Direction: "outgoing", Size: 200},
		})
	}

	status := capture.GetWebSocketStatus(types.WebSocketStatusFilter{})
	conn := status.Connections[0]

	if conn.MessageRate.Outgoing.PerSecond < 0.5 {
		t.Errorf("Expected outgoing rate >= 0.5 msg/s, got %.2f", conn.MessageRate.Outgoing.PerSecond)
	}
}

func TestV4LastMessageAgeFormatted(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Open connection
	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{Timestamp: time.Now().Add(-60 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Last message 3 seconds ago
	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{
			Timestamp: time.Now().Add(-3 * time.Second).Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "message",
			Direction: "incoming",
			Data:      `{"type":"ping"}`,
			Size:      15,
		},
	})

	status := capture.GetWebSocketStatus(types.WebSocketStatusFilter{})
	conn := status.Connections[0]

	if conn.LastMessage.Incoming == nil {
		t.Fatal("Expected incoming last message to be set")
	}

	age := conn.LastMessage.Incoming.Age
	if age == "" {
		t.Fatal("Expected Age to be set on last message preview")
	}

	// Should be approximately "3s" or "3.Xs"
	if !strings.HasSuffix(age, "s") {
		t.Errorf("Expected age ending in 's', got: %s", age)
	}
}

func TestV4LastMessageAgeMinutesFormat(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{Timestamp: time.Now().Add(-600 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Last message 2 minutes 30 seconds ago
	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{
			Timestamp: time.Now().Add(-150 * time.Second).Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "message",
			Direction: "outgoing",
			Data:      `{"type":"update"}`,
			Size:      20,
		},
	})

	status := capture.GetWebSocketStatus(types.WebSocketStatusFilter{})
	conn := status.Connections[0]

	if conn.LastMessage.Outgoing == nil {
		t.Fatal("Expected outgoing last message to be set")
	}

	age := conn.LastMessage.Outgoing.Age
	if !strings.Contains(age, "m") {
		t.Errorf("Expected age with 'm' for minutes, got: %s", age)
	}
}

func TestV4LastMessageAgeSubSecond(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{Timestamp: time.Now().Add(-10 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Last message just now (< 1 second ago)
	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{
			Timestamp: time.Now().Add(-200 * time.Millisecond).Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "message",
			Direction: "incoming",
			Data:      `{"type":"heartbeat"}`,
			Size:      20,
		},
	})

	status := capture.GetWebSocketStatus(types.WebSocketStatusFilter{})
	conn := status.Connections[0]

	age := conn.LastMessage.Incoming.Age
	if age == "" {
		t.Fatal("Expected age to be set for sub-second message")
	}

	// Should show fractional seconds like "0.2s"
	if !strings.HasSuffix(age, "s") {
		t.Errorf("Expected sub-second age ending in 's', got: %s", age)
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

	capture.HandleWebSocketStatus(rec, req)

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

	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "ws-1", Event: "open", URL: "wss://chat.example.com/ws", Timestamp: time.Now().Format(time.RFC3339Nano)},
		{ID: "ws-2", Event: "open", URL: "wss://feed.example.com/prices", Timestamp: time.Now().Format(time.RFC3339Nano)},
	})

	req := httptest.NewRequest("GET", "/websocket-status", nil)
	rec := httptest.NewRecorder()

	capture.HandleWebSocketStatus(rec, req)

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

	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "ws-closed", Event: "open", URL: "wss://example.com/ws"},
		{ID: "ws-closed", Event: "close", URL: "wss://example.com/ws", CloseCode: 1001, CloseReason: "going away"},
	})

	req := httptest.NewRequest("GET", "/websocket-status", nil)
	rec := httptest.NewRecorder()

	capture.HandleWebSocketStatus(rec, req)

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

	cap.AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "ws-a", Event: "open", URL: "wss://a.example.com/ws"},
		{ID: "ws-b", Event: "open", URL: "wss://b.example.com/ws"},
		{ID: "ws-c", Event: "open", URL: "wss://c.example.com/ws"},
	})

	status := cap.GetWebSocketStatus(types.WebSocketStatusFilter{ConnectionID: "ws-b"})

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

	cap.AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "ws-1", Event: "open", URL: "wss://chat.example.com/ws"},
		{ID: "ws-2", Event: "open", URL: "wss://feed.example.com/prices"},
		{ID: "ws-3", Event: "open", URL: "wss://chat.example.com/live"},
	})

	status := cap.GetWebSocketStatus(types.WebSocketStatusFilter{URLFilter: "chat"})

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

	cap.AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "ws-1", Event: "open", URL: "wss://chat.example.com/ws"},
		{ID: "ws-2", Event: "open", URL: "wss://chat.example.com/live"},
		{ID: "ws-3", Event: "open", URL: "wss://feed.example.com/prices"},
	})

	// When both filters are set, both should apply (connection_id narrows first).
	status := cap.GetWebSocketStatus(types.WebSocketStatusFilter{
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

	capture.HandleWebSocketEvents(rec, req)

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

	capture.circuit.ForceOpen("rate_exceeded")

	body := `{"events":[{"event":"open","id":"ws-1","url":"wss://example.com/ws"}]}`
	req := httptest.NewRequest("POST", "/websocket-events", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	capture.HandleWebSocketEvents(rec, req)

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

	capture.HandleWebSocketEvents(rec, req)

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

	capture.HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad JSON, got %d", rec.Code)
	}
}

// Test: HandleWebSocketEvents POST re-check rate limit after recording.
func TestV4HandleWebSocketEvents_POST_RateLimitAfterRecording(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.circuit.SetWindowState(time.Now(), RateLimitThreshold-1)

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

	capture.HandleWebSocketEvents(rec, req)

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
	cap.AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "ws-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Zero-value filter (what the MCP layer falls back to on parse error).
	status := cap.GetWebSocketStatus(types.WebSocketStatusFilter{})
	if len(status.Connections) != 1 {
		t.Fatalf("expected 1 connection with zero-value filter, got %d", len(status.Connections))
	}

	// Filter with non-matching connection_id returns empty (not an error).
	status = cap.GetWebSocketStatus(types.WebSocketStatusFilter{ConnectionID: "nonexistent"})
	if len(status.Connections) != 0 {
		t.Errorf("expected 0 connections for non-matching ID, got %d", len(status.Connections))
	}

	// Filter with non-matching URL returns empty (not an error).
	status = cap.GetWebSocketStatus(types.WebSocketStatusFilter{URLFilter: "nonexistent"})
	if len(status.Connections) != 0 {
		t.Errorf("expected 0 connections for non-matching URL, got %d", len(status.Connections))
	}
}
