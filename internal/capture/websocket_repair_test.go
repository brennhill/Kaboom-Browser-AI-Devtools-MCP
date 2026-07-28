// Purpose: Tests for WebSocket entry wrapper struct integrity.
// Docs: docs/features/feature/backend-log-streaming/index.md

// websocket_repair_test.go — Unit tests for wsEventEntry buffer consistency.

package capture

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"testing"
)

func TestWSEntryBuffer_EqualLength(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	c.Telemetry().AddWebSocketEventsForTest([]types.WebSocketEvent{
		{Event: "message", Data: "hello", ID: "ws1"},
		{Event: "message", Data: "world", ID: "ws1"},
	})

	events, addedAt, _ := c.Telemetry().GetWSLengthsForTest()
	if events != addedAt {
		t.Fatalf("expected equal lengths, got events=%d addedAt=%d", events, addedAt)
	}

	// Add via production path
	c.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{{Event: "message", Data: "new", ID: "ws1"}})

	events, addedAt, _ = c.Telemetry().GetWSLengthsForTest()
	if events != addedAt {
		t.Fatalf("after add: expected equal lengths, got events=%d addedAt=%d", events, addedAt)
	}
	if events != 3 {
		t.Fatalf("expected 3 events, got %d", events)
	}
}

func TestWSEntryBuffer_ExtraEventsViaTestHelper(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Add 2 events, then 3 extra via test helper
	c.Telemetry().AddWebSocketEventsForTest([]types.WebSocketEvent{
		{Event: "message", Data: "matched1", ID: "ws1"},
		{Event: "message", Data: "matched2", ID: "ws1"},
	})
	c.Telemetry().AddExtraWSEventsForTest(3)

	events, addedAt, _ := c.Telemetry().GetWSLengthsForTest()
	// With entry wrappers, events and addedAt are always equal
	if events != addedAt {
		t.Fatalf("expected equal lengths, got events=%d addedAt=%d", events, addedAt)
	}
	if events != 5 {
		t.Fatalf("expected 5 events, got %d", events)
	}

	// Adding another event should work fine
	c.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{{Event: "message", Data: "trigger", ID: "ws1"}})

	events, addedAt, _ = c.Telemetry().GetWSLengthsForTest()
	if events != addedAt {
		t.Fatalf("after add: expected equal lengths, got events=%d addedAt=%d", events, addedAt)
	}
	if events != 6 {
		t.Fatalf("expected 6 events, got %d", events)
	}
}

func TestWSEntryBuffer_MemoryTracked(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Add events with known data sizes
	c.Telemetry().AddWebSocketEventsForTest([]types.WebSocketEvent{
		{Event: "message", Data: "aaaa", ID: "ws1"},   // 4 bytes data
		{Event: "message", Data: "bbbbbb", ID: "ws1"}, // 6 bytes data
	})
	// Add extra events via test helper
	c.Telemetry().AddExtraWSEventsForTest(1)

	// Add via production path to trigger memory accounting
	c.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{{Event: "message", Data: "cc", ID: "ws1"}})

	events, addedAt, mem := c.Telemetry().GetWSLengthsForTest()
	if events != addedAt {
		t.Fatalf("expected equal lengths, got events=%d addedAt=%d", events, addedAt)
	}
	if mem <= 0 {
		t.Fatalf("expected positive memory total, got %d", mem)
	}
}

func TestWSEntryBuffer_BothEmpty(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Buffer is empty
	events, addedAt, _ := c.Telemetry().GetWSLengthsForTest()
	if events != 0 || addedAt != 0 {
		t.Fatalf("expected both empty, got events=%d addedAt=%d", events, addedAt)
	}

	// Adding an event should work fine
	c.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{{Event: "open", ID: "ws1"}})

	events, addedAt, _ = c.Telemetry().GetWSLengthsForTest()
	if events != addedAt {
		t.Fatalf("expected equal lengths, got events=%d addedAt=%d", events, addedAt)
	}
	if events != 1 {
		t.Fatalf("expected 1 event, got %d", events)
	}
}
