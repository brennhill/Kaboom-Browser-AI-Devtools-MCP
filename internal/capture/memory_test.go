// memory_test.go — Verifies WebSocket memory accounting and coordinated clearing.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
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

func extractWSEvents(entries []wsEventEntry) []types.WebSocketEvent {
	out := make([]types.WebSocketEvent, len(entries))
	for i := range entries {
		out[i] = entries[i].Event
	}
	return out
}

func bruteForceWSMemory(events []types.WebSocketEvent) int64 {
	var total int64
	for i := range events {
		total += int64(len(events[i].Data)) + wsEventOverhead
	}
	return total
}

func assertWSMemoryConsistent(t *testing.T, c *Capture) {
	t.Helper()
	c.telemetry.mu.RLock()
	running := c.telemetry.buffers.wsMemoryTotal
	expected := bruteForceWSMemory(extractWSEvents(c.telemetry.buffers.wsEvents.Snapshot()))
	c.telemetry.mu.RUnlock()
	if running != expected {
		t.Fatalf("ws memory total = %d, retained-entry estimate = %d", running, expected)
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
	assertWSMemoryConsistent(t, c)
	if count := len(c.Telemetry().GetAllWebSocketEvents()); count > maxWSEvents {
		t.Fatalf("retained %d WebSocket events, want at most %d", count, maxWSEvents)
	}

	large := make([]types.WebSocketEvent, 100)
	for i := range large {
		large[i] = makeWSEvent(50000)
	}
	c.Telemetry().AddWebSocketEvents(large)
	assertWSMemoryConsistent(t, c)
	if pressure := c.Telemetry().Pressure().WebSocket; pressure.Dropped == 0 {
		t.Fatalf("WebSocket pressure = %+v, want memory-pressure drops", pressure)
	}
}

func TestWebSocketMemoryRunningTotalMatchesAccessor(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	t.Cleanup(c.Close)
	c.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{makeWSEvent(500), makeWSEvent(1000)})

	c.telemetry.mu.RLock()
	calculated := c.telemetry.buffers.calcWSMemory()
	running := c.telemetry.buffers.wsMemoryTotal
	c.telemetry.mu.RUnlock()
	if calculated != running || running == 0 {
		t.Fatalf("calculated=%d running=%d, want equal positive totals", calculated, running)
	}
	assertWSMemoryConsistent(t, c)
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

	NewStateResetter(c).ClearAll()
	c.telemetry.mu.RLock()
	wsMemory := c.telemetry.buffers.wsMemoryTotal
	c.telemetry.mu.RUnlock()
	if wsMemory != 0 || c.Telemetry().NetworkBodies().Stats().MemoryBytes != 0 {
		t.Fatalf("memory after clear: websocket=%d network=%d", wsMemory, c.Telemetry().NetworkBodies().Stats().MemoryBytes)
	}
}
