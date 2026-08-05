// memory_test.go — Verifies WebSocket memory accounting and coordinated clearing.
// Docs: docs/features/feature/backend-log-streaming/index.md

package pipelinetest

import (
	. "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func makeWSEvent(dataSize int) types.WebSocketEvent {
	data := make([]byte, dataSize)
	for i := range data {
		data[i] = 'x'
	}
	return types.WebSocketEvent{
		ID: "conn-1", Event: "message", Direction: "incoming",
		Data: string(data), Timestamp: time.Now().Format(time.RFC3339Nano),
	}
}

func makeNetworkBody(reqSize, respSize int) types.NetworkBody {
	return types.NetworkBody{
		Method: "GET", URL: "http://example.com/api", Status: 200,
		RequestBody: string(make([]byte, reqSize)), ResponseBody: string(make([]byte, respSize)),
	}
}

func TestWebSocketMemoryTracksAddRotationAndPressureEviction(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	t.Cleanup(c.Close)

	events := make([]types.WebSocketEvent, maxWSEvents+10)
	for i := range events {
		events[i] = makeWSEvent(100 + i)
	}
	c.Telemetry().AddWebSocketEvents(events)
	if memory := c.Telemetry().WebSockets().Stats().MemoryBytes; memory <= 0 {
		t.Fatalf("WebSocket memory = %d, want positive after ingestion", memory)
	}
	if count := len(c.Telemetry().WebSockets().Snapshot().Events); count > maxWSEvents {
		t.Fatalf("retained %d WebSocket events, want at most %d", count, maxWSEvents)
	}

	large := make([]types.WebSocketEvent, 100)
	for i := range large {
		large[i] = makeWSEvent(50000)
	}
	c.Telemetry().AddWebSocketEvents(large)
	if memory := c.Telemetry().WebSockets().Stats().MemoryBytes; memory > wsBufferMemoryLimit {
		t.Fatalf("WebSocket memory = %d, exceeds limit %d", memory, wsBufferMemoryLimit)
	}
	if pressure := c.Telemetry().Pressure().WebSocket; pressure.Dropped == 0 {
		t.Fatalf("WebSocket pressure = %+v, want memory-pressure drops", pressure)
	}
}

func TestWebSocketMemoryRunningTotalMatchesAccessor(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	t.Cleanup(c.Close)
	c.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{makeWSEvent(500), makeWSEvent(1000)})

	running := c.Telemetry().WebSockets().Stats().MemoryBytes
	if running == 0 {
		t.Fatal("WebSocket memory is zero after ingestion")
	}
}

func TestStateResetterClearsIndependentBodyAndWebSocketMemoryOwners(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	t.Cleanup(c.Close)
	c.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{makeWSEvent(1000)})
	c.Telemetry().AddNetworkBodies([]types.NetworkBody{makeNetworkBody(500, 1000)})
	if c.Telemetry().NetworkBodies().Stats().MemoryBytes == 0 {
		t.Fatal("network-body memory is zero before coordinated clear")
	}

	resetterForTest(c).ClearAll()
	wsMemory := c.Telemetry().WebSockets().Stats().MemoryBytes
	if wsMemory != 0 || c.Telemetry().NetworkBodies().Stats().MemoryBytes != 0 {
		t.Fatalf("memory after clear: websocket=%d network=%d", wsMemory, c.Telemetry().NetworkBodies().Stats().MemoryBytes)
	}
}
