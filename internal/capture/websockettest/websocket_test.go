// websocket_test.go — Tests WebSocket event buffering and query filters.
// Docs: docs/features/feature/backend-log-streaming/index.md

package websockettest

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestV4WebSocketEventBuffer(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	events := []types.WebSocketEvent{
		{Timestamp: "2024-01-15T10:30:00.000Z", Type: "websocket", Event: "open", ID: "uuid-1", URL: "wss://example.com/ws"},
		{Timestamp: "2024-01-15T10:30:01.000Z", Type: "websocket", Event: "message", ID: "uuid-1", Direction: "incoming", Data: `{"type":"chat","msg":"hello"}`, Size: 32},
	}

	capture.Telemetry().AddWebSocketEvents(events)

	if len(capture.Telemetry().WebSockets().Snapshot().Events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(capture.Telemetry().WebSockets().Snapshot().Events))
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

	capture.Telemetry().AddWebSocketEvents(events)

	if len(capture.Telemetry().WebSockets().Snapshot().Events) != 500 {
		t.Errorf("Expected 500 events after rotation, got %d", len(capture.Telemetry().WebSockets().Snapshot().Events))
	}
}

func TestV4WebSocketEventFilterByConnectionID(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://a.com"},
		{ID: "uuid-2", Event: "open", URL: "wss://b.com"},
		{ID: "uuid-1", Event: "message", Direction: "incoming"},
	})

	filtered := capture.Telemetry().WebSockets().Events(types.WebSocketEventFilter{ConnectionID: "uuid-1"})

	if len(filtered) != 2 {
		t.Errorf("Expected 2 events for uuid-1, got %d", len(filtered))
	}
}

func TestV4WebSocketEventFilterByURL(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://chat.example.com/ws"},
		{ID: "uuid-2", Event: "open", URL: "wss://feed.example.com/prices"},
		{ID: "uuid-1", Event: "message", URL: "wss://chat.example.com/ws"},
	})

	filtered := capture.Telemetry().WebSockets().Events(types.WebSocketEventFilter{URLFilter: "chat"})

	if len(filtered) != 2 {
		t.Errorf("Expected 2 events matching 'chat', got %d", len(filtered))
	}
}

func TestV4WebSocketEventFilterByDirection(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "uuid-1", Event: "message", Direction: "incoming"},
		{ID: "uuid-1", Event: "message", Direction: "outgoing"},
		{ID: "uuid-1", Event: "message", Direction: "incoming"},
	})

	filtered := capture.Telemetry().WebSockets().Events(types.WebSocketEventFilter{Direction: "incoming"})

	if len(filtered) != 2 {
		t.Errorf("Expected 2 incoming events, got %d", len(filtered))
	}
}

func TestV4WebSocketEventFilterWithLimit(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	for i := 0; i < 10; i++ {
		capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
			{ID: "uuid-1", Event: "message", Direction: "incoming"},
		})
	}

	filtered := capture.Telemetry().WebSockets().Events(types.WebSocketEventFilter{Limit: 5})

	if len(filtered) != 5 {
		t.Errorf("Expected 5 events with limit, got %d", len(filtered))
	}
}

func TestV4WebSocketEventDefaultLimit(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	for i := 0; i < 100; i++ {
		capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
			{ID: "uuid-1", Event: "message"},
		})
	}

	// Default limit is 50
	filtered := capture.Telemetry().WebSockets().Events(types.WebSocketEventFilter{})

	if len(filtered) != 50 {
		t.Errorf("Expected 50 events with default limit, got %d", len(filtered))
	}
}

func TestV4WebSocketEventNewestFirst(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{Timestamp: "2024-01-15T10:30:00.000Z", ID: "uuid-1", Event: "open"},
		{Timestamp: "2024-01-15T10:30:05.000Z", ID: "uuid-1", Event: "close"},
	})

	filtered := capture.Telemetry().WebSockets().Events(types.WebSocketEventFilter{})

	if len(filtered) == 0 {
		t.Fatal("Expected events to be returned")
	}
	if filtered[0].Timestamp != "2024-01-15T10:30:05.000Z" {
		t.Errorf("Expected newest first, got %s", filtered[0].Timestamp)
	}
}
