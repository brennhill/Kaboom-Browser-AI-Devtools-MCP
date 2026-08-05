// websocket_status_test.go — Tests WebSocket connection tracking and status metrics.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestV4WebSocketConnectionTracker(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://chat.example.com/ws", Timestamp: "2024-01-15T10:30:00.000Z"},
	})

	status := capture.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{})

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

	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "uuid-1", Event: "close", URL: "wss://example.com/ws", CloseCode: 1000, CloseReason: "normal closure"},
	})

	status := capture.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{})

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

	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "uuid-1", Event: "error", URL: "wss://example.com/ws"},
	})

	status := capture.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{})

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

	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "uuid-1", Event: "message", Direction: "incoming", Size: 100},
		{ID: "uuid-1", Event: "message", Direction: "incoming", Size: 200},
		{ID: "uuid-1", Event: "message", Direction: "outgoing", Size: 50},
	})

	status := capture.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{})

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

	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "uuid-1", Event: "message", Direction: "incoming", Data: `{"type":"hello"}`, Timestamp: "2024-01-15T10:30:01.000Z"},
		{ID: "uuid-1", Event: "message", Direction: "incoming", Data: `{"type":"world"}`, Timestamp: "2024-01-15T10:30:02.000Z"},
	})

	status := capture.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{})
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
		capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
			{ID: "uuid-" + string(rune('a'+i)), Event: "open", URL: "wss://example.com/ws"},
		})
	}

	status := capture.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{})

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
		capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
			{ID: id, Event: "open", URL: "wss://example.com/ws"},
			{ID: id, Event: "close", URL: "wss://example.com/ws", CloseCode: 1000},
		})
	}

	status := capture.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{})

	if len(status.Closed) > 10 {
		t.Errorf("Expected max 10 closed connections in history, got %d", len(status.Closed))
	}
}

func TestV4WebSocketStatusFilterByURL(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://chat.example.com/ws"},
		{ID: "uuid-2", Event: "open", URL: "wss://feed.example.com/prices"},
	})

	status := capture.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{URLFilter: "chat"})

	if len(status.Connections) != 1 {
		t.Errorf("Expected 1 connection matching 'chat', got %d", len(status.Connections))
	}
}

func TestV4WebSocketStatusFilterByConnectionID(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://a.com"},
		{ID: "uuid-2", Event: "open", URL: "wss://b.com"},
	})

	status := capture.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{ConnectionID: "uuid-2"})

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

	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
		{ID: "uuid-1", Event: "message", Direction: "incoming", Sampled: &types.SamplingInfo{Rate: "48.2/s", Logged: "1/5", Window: "5s"}},
	})

	status := capture.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{})
	conn := status.Connections[0]

	if !conn.Sampling.Active {
		t.Error("Expected sampling to be active")
	}
}

func TestV4ConnectionDurationFormatted(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	openedAt := time.Now().Add(-5*time.Minute - 2*time.Second)
	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{
			Timestamp: openedAt.Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "open",
			URL:       "wss://example.com/ws",
		},
	})

	status := capture.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{})
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
	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{
			Timestamp: openedAt.Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "open",
			URL:       "wss://example.com/ws",
		},
	})

	status := capture.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{})
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
	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{
			Timestamp: openedAt.Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "open",
			URL:       "wss://example.com/ws",
		},
	})

	status := capture.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{})
	conn := status.Connections[0]

	if !strings.Contains(conn.Duration, "h") {
		t.Errorf("Expected duration with 'h' for hours, got: %s", conn.Duration)
	}
}

func TestV4MessageRateCalculation(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Open connection
	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{Timestamp: time.Now().Add(-10 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Send 10 messages over the last 5 seconds (2 per second)
	now := time.Now()
	for i := 0; i < 10; i++ {
		ts := now.Add(-5*time.Second + time.Duration(i)*500*time.Millisecond)
		capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
			{Timestamp: ts.Format(time.RFC3339Nano), ID: "uuid-1", Event: "message", Direction: "incoming", Size: 100},
		})
	}

	status := capture.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{})
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
	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{Timestamp: time.Now().Add(-60 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Send messages long ago (outside 5-second window)
	oldTime := time.Now().Add(-30 * time.Second)
	for i := 0; i < 5; i++ {
		capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
			{Timestamp: oldTime.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "message", Direction: "incoming", Size: 50},
		})
	}

	status := capture.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{})
	conn := status.Connections[0]

	// Rate should be 0 since all messages are outside the 5-second window
	if conn.MessageRate.Incoming.PerSecond != 0.0 {
		t.Errorf("Expected incoming rate 0 for old messages, got %.2f", conn.MessageRate.Incoming.PerSecond)
	}
}

func TestV4MessageRateOutgoing(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{Timestamp: time.Now().Add(-10 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Send 5 outgoing messages in last 5 seconds
	now := time.Now()
	for i := 0; i < 5; i++ {
		ts := now.Add(-4*time.Second + time.Duration(i)*time.Second)
		capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
			{Timestamp: ts.Format(time.RFC3339Nano), ID: "uuid-1", Event: "message", Direction: "outgoing", Size: 200},
		})
	}

	status := capture.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{})
	conn := status.Connections[0]

	if conn.MessageRate.Outgoing.PerSecond < 0.5 {
		t.Errorf("Expected outgoing rate >= 0.5 msg/s, got %.2f", conn.MessageRate.Outgoing.PerSecond)
	}
}

func TestV4LastMessageAgeFormatted(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Open connection
	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{Timestamp: time.Now().Add(-60 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Last message 3 seconds ago
	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{
			Timestamp: time.Now().Add(-3 * time.Second).Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "message",
			Direction: "incoming",
			Data:      `{"type":"ping"}`,
			Size:      15,
		},
	})

	status := capture.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{})
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

	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{Timestamp: time.Now().Add(-600 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Last message 2 minutes 30 seconds ago
	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{
			Timestamp: time.Now().Add(-150 * time.Second).Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "message",
			Direction: "outgoing",
			Data:      `{"type":"update"}`,
			Size:      20,
		},
	})

	status := capture.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{})
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

	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{Timestamp: time.Now().Add(-10 * time.Second).Format(time.RFC3339Nano), ID: "uuid-1", Event: "open", URL: "wss://example.com/ws"},
	})

	// Last message just now (< 1 second ago)
	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{
			Timestamp: time.Now().Add(-200 * time.Millisecond).Format(time.RFC3339Nano),
			ID:        "uuid-1",
			Event:     "message",
			Direction: "incoming",
			Data:      `{"type":"heartbeat"}`,
			Size:      20,
		},
	})

	status := capture.Telemetry().WebSockets().Status(types.WebSocketStatusFilter{})
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
