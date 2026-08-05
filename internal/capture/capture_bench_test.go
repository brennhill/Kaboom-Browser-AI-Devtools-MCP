// Purpose: Benchmark capture pipeline throughput and latency.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// BenchmarkAddWebSocketEvents measures WebSocket event buffering performance
func BenchmarkAddWebSocketEvents(b *testing.B) {
	cap := NewCapture()

	events := []types.WebSocketEvent{
		{
			Timestamp: time.Now().Format(time.RFC3339Nano),
			ID:        "ws_123",
			Event:     "message",
			Data:      "test message payload",
			URL:       "wss://example.com/socket",
		},
	}

	for i := 0; i < maxWSEvents; i++ {
		cap.Telemetry().AddWebSocketEvents(events)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cap.Telemetry().AddWebSocketEvents(events)
	}
}

// BenchmarkAddNetworkBodies measures network body capture performance
func BenchmarkAddNetworkBodies(b *testing.B) {
	cap := NewCapture()

	bodies := []types.NetworkBody{
		{
			Timestamp:    time.Now().Format(time.RFC3339Nano),
			Method:       "POST",
			URL:          "https://api.example.com/users",
			Status:       200,
			RequestBody:  `{"name":"test"}`,
			ResponseBody: `{"id":123,"name":"test"}`,
			ContentType:  "application/json",
			Duration:     142,
		},
	}

	for i := 0; i < maxNetworkBodies; i++ {
		cap.Telemetry().AddNetworkBodies(bodies)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cap.Telemetry().AddNetworkBodies(bodies)
	}
}

// BenchmarkAddEnhancedActions measures user action capture performance
func BenchmarkAddEnhancedActions(b *testing.B) {
	cap := NewCapture()

	actions := []types.EnhancedAction{
		{
			Timestamp: time.Now().UnixNano(),
			Type:      "click",
			Selectors: map[string]any{"css": "button.submit"},
			Value:     "",
			URL:       "https://example.com/page",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cap.Telemetry().AddEnhancedActions(actions)
	}
}

// BenchmarkMemoryEnforcement measures memory limit enforcement overhead
func BenchmarkMemoryEnforcement(b *testing.B) {
	cap := NewCapture()

	// Pre-populate with data near soft limit
	for i := 0; i < 1000; i++ {
		cap.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
			{
				Timestamp: time.Now().Format(time.RFC3339Nano),
				ID:        "ws_bench",
				Event:     "message",
				Data:      string(make([]byte, 1000)), // 1KB per event
			},
		})
	}

	event := []types.WebSocketEvent{{
		Timestamp: time.Now().Format(time.RFC3339Nano),
		ID:        "ws_bench",
		Event:     "message",
		Data:      "test",
	}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cap.Telemetry().AddWebSocketEvents(event)
	}
}

// BenchmarkConcurrentCapture measures concurrent capture performance
func BenchmarkConcurrentCapture(b *testing.B) {
	cap := NewCapture()

	wsEvent := []types.WebSocketEvent{{
		Timestamp: time.Now().Format(time.RFC3339Nano),
		ID:        "ws_concurrent",
		Event:     "message",
		Data:      "test",
	}}

	networkBody := []types.NetworkBody{{
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Method:    "GET",
		URL:       "https://example.com/api",
		Status:    200,
	}}

	action := []types.EnhancedAction{{
		Timestamp: time.Now().UnixNano(),
		Type:      "click",
		Selectors: map[string]any{"css": "button"},
	}}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			switch i % 3 {
			case 0:
				cap.Telemetry().AddWebSocketEvents(wsEvent)
			case 1:
				cap.Telemetry().AddNetworkBodies(networkBody)
			case 2:
				cap.Telemetry().AddEnhancedActions(action)
			}
			i++
		}
	})
}
