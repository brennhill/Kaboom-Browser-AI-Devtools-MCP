// Purpose: End-to-end tests that binary format detection runs on ingest for network bodies and WS messages.
// Docs: docs/features/feature/binary-format-detection/index.md

package capture_test

import (
	"testing"

	cap "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// Integration tests for binary format detection in network/websocket

func TestNetworkBody_BinaryFormatIntegration(t *testing.T) {
	t.Parallel()
	c := cap.NewCapture()

	// Add a network body with MessagePack binary data
	msgpackData := string([]byte{0x81, 0xa3, 0x6b, 0x65, 0x79, 0xa5, 0x76, 0x61, 0x6c, 0x75, 0x65})
	bodies := []cap.NetworkBody{
		{
			URL:          "https://api.example.com/data",
			Method:       "GET",
			Status:       200,
			ResponseBody: msgpackData,
		},
	}
	c.AddNetworkBodies(bodies)

	// Retrieve and verify binary format was detected
	result := c.GetNetworkBodies()
	if len(result) != 1 {
		t.Fatalf("expected 1 body, got %d", len(result))
	}
	if result[0].BinaryFormat != "messagepack" {
		t.Errorf("expected binary_format 'messagepack', got %q", result[0].BinaryFormat)
	}
	if result[0].FormatConfidence < 0.7 {
		t.Errorf("expected format_confidence >= 0.7, got %f", result[0].FormatConfidence)
	}
}

func TestNetworkBody_TextNoFormat(t *testing.T) {
	t.Parallel()
	c := cap.NewCapture()

	// Add a network body with JSON text data
	bodies := []cap.NetworkBody{
		{
			URL:          "https://api.example.com/json",
			Method:       "GET",
			Status:       200,
			ResponseBody: `{"key": "value"}`,
		},
	}
	c.AddNetworkBodies(bodies)

	// Verify no binary format detected for text
	result := c.GetNetworkBodies()
	if len(result) != 1 {
		t.Fatalf("expected 1 body, got %d", len(result))
	}
	if result[0].BinaryFormat != "" {
		t.Errorf("expected empty binary_format for JSON, got %q", result[0].BinaryFormat)
	}
}

func TestWebSocketEvent_BinaryFormatIntegration(t *testing.T) {
	t.Parallel()
	c := cap.NewCapture()

	// Add a WebSocket message with protobuf binary data
	protobufData := string([]byte{0x08, 0x96, 0x01})
	events := []cap.WebSocketEvent{
		{
			Event:     "message",
			ID:        "ws-1",
			URL:       "wss://api.example.com/ws",
			Direction: "incoming",
			Data:      protobufData,
			Size:      len(protobufData),
		},
	}
	c.AddWebSocketEvents(events)

	// Retrieve and verify binary format was detected
	result := c.GetWebSocketEvents(cap.WebSocketEventFilter{Limit: 1})
	if len(result) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result))
	}
	if result[0].BinaryFormat != "protobuf" {
		t.Errorf("expected binary_format 'protobuf', got %q", result[0].BinaryFormat)
	}
	if result[0].FormatConfidence < 0.5 {
		t.Errorf("expected format_confidence >= 0.5, got %f", result[0].FormatConfidence)
	}
}

func TestWebSocketEvent_OpenCloseNoFormat(t *testing.T) {
	t.Parallel()
	c := cap.NewCapture()

	// Add open/close events which shouldn't have binary format detection
	events := []cap.WebSocketEvent{
		{
			Event: "open",
			ID:    "ws-1",
			URL:   "wss://api.example.com/ws",
		},
		{
			Event:       "close",
			ID:          "ws-1",
			CloseCode:   1000,
			CloseReason: "normal",
		},
	}
	c.AddWebSocketEvents(events)

	// Verify no binary format for non-message events
	result := c.GetWebSocketEvents(cap.WebSocketEventFilter{Limit: 10})
	for _, ev := range result {
		if ev.BinaryFormat != "" {
			t.Errorf("expected empty binary_format for %s event, got %q", ev.Event, ev.BinaryFormat)
		}
	}
}

func TestWebSocketEvent_TextMessageNoFormat(t *testing.T) {
	t.Parallel()
	c := cap.NewCapture()

	// Add a text message
	events := []cap.WebSocketEvent{
		{
			Event:     "message",
			ID:        "ws-1",
			URL:       "wss://api.example.com/ws",
			Direction: "outgoing",
			Data:      `{"action": "ping"}`,
			Size:      18,
		},
	}
	c.AddWebSocketEvents(events)

	// Verify no binary format for text message
	result := c.GetWebSocketEvents(cap.WebSocketEventFilter{Limit: 1})
	if len(result) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result))
	}
	if result[0].BinaryFormat != "" {
		t.Errorf("expected empty binary_format for text message, got %q", result[0].BinaryFormat)
	}
}

// Benchmark to ensure detection is fast
func BenchmarkDetectBinaryFormat(b *testing.B) {
	testCases := []struct {
		name string
		data []byte
	}{
		{"messagepack", []byte{0x81, 0xa3, 0x6b, 0x65, 0x79, 0xa5, 0x76, 0x61, 0x6c, 0x75, 0x65}},
		{"protobuf", []byte{0x08, 0x96, 0x01}},
		{"cbor", []byte{0xa1, 0x61, 0x61, 0x01}},
		{"bson", []byte{0x05, 0x00, 0x00, 0x00, 0x00}},
		{"text", []byte(`{"key": "value"}`)},
		{"empty", nil},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				util.DetectBinaryFormat(tc.data)
			}
		})
	}
}
