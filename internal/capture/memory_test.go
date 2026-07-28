// Purpose: Tests for capture memory accounting and limits.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"testing"
	"time"
)

// ============================================
// Per-Buffer Memory Tracking Tests
// ============================================

// Helper: create a types.WebSocketEvent with a specific data size
func makeWSEvent(dataSize int) types.WebSocketEvent {
	data := make([]byte, dataSize)
	for i := range data {
		data[i] = 'x'
	}
	return types.WebSocketEvent{
		ID:        "conn-1",
		Event:     "message",
		Direction: "incoming",
		Data:      string(data),
		Timestamp: time.Now().Format(time.RFC3339Nano),
	}
}

// Helper: create a types.NetworkBody with specific body sizes
func makeNetworkBody(reqSize, respSize int) types.NetworkBody {
	reqBody := make([]byte, reqSize)
	for i := range reqBody {
		reqBody[i] = 'r'
	}
	respBody := make([]byte, respSize)
	for i := range respBody {
		respBody[i] = 'R'
	}
	return types.NetworkBody{
		Method:       "GET",
		URL:          "http://example.com/api",
		Status:       200,
		RequestBody:  string(reqBody),
		ResponseBody: string(respBody),
	}
}

// Helper: create an types.EnhancedAction
func makeAction() types.EnhancedAction {
	return types.EnhancedAction{
		Type:      "click",
		Timestamp: time.Now().UnixMilli(),
		URL:       "http://example.com",
	}
}

// Helper: recalculate running memory totals from current slices.
// Must be called with lock held.
func recalcMemoryTotals(c *Capture) {
	c.telemetry.buffers.wsMemoryTotal = 0
	for i := range c.telemetry.buffers.wsEvents {
		c.telemetry.buffers.wsMemoryTotal += wsEventMemory(&c.telemetry.buffers.wsEvents[i].Event)
	}
	c.telemetry.buffers.networkBodyMemoryTotal = 0
	for i := range c.telemetry.buffers.networkBodies {
		c.telemetry.buffers.networkBodyMemoryTotal += nbEntryMemory(&c.telemetry.buffers.networkBodies[i].Body)
	}
}

// extractWSEvents extracts types.WebSocketEvent values from entry wrappers (test helper).
func extractWSEvents(entries []wsEventEntry) []types.WebSocketEvent {
	out := make([]types.WebSocketEvent, len(entries))
	for i := range entries {
		out[i] = entries[i].Event
	}
	return out
}

// extractNetworkBodies extracts types.NetworkBody values from entry wrappers (test helper).
func extractNetworkBodies(entries []networkBodyEntry) []types.NetworkBody {
	out := make([]types.NetworkBody, len(entries))
	for i := range entries {
		out[i] = entries[i].Body
	}
	return out
}

// bruteForceWSMemory recalculates WS memory by iterating all events (reference implementation).
func bruteForceWSMemory(events []types.WebSocketEvent) int64 {
	var total int64
	for i := range events {
		total += int64(len(events[i].Data)) + wsEventOverhead
	}
	return total
}

// bruteForceNBMemory recalculates NB memory by iterating all bodies (reference implementation).
func bruteForceNBMemory(bodies []types.NetworkBody) int64 {
	var total int64
	for i := range bodies {
		total += int64(len(bodies[i].RequestBody)+len(bodies[i].ResponseBody)) + networkBodyOverhead
	}
	return total
}

// ============================================
// Per-Entry Memory Estimation
// ============================================

func TestMemory_CalcWSMemory_PerEventEstimate(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	dataSize := 1000
	c.telemetry.mu.Lock()
	c.telemetry.buffers.wsEvents = append(c.telemetry.buffers.wsEvents, wsEventEntry{
		Event:   makeWSEvent(dataSize),
		AddedAt: time.Now(),
	})
	recalcMemoryTotals(c)
	c.telemetry.mu.Unlock()

	c.telemetry.mu.RLock()
	mem := c.telemetry.buffers.calcWSMemory()
	c.telemetry.mu.RUnlock()

	expectedMin := int64(dataSize + 100)
	expectedMax := int64(dataSize + 400)

	if mem < expectedMin || mem > expectedMax {
		t.Errorf("calcWSMemory() = %d, expected between %d and %d for %d-byte data",
			mem, expectedMin, expectedMax, dataSize)
	}
}

func TestMemory_CalcNBMemory_PerEntryEstimate(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	reqSize, respSize := 500, 1500
	c.telemetry.mu.Lock()
	c.telemetry.buffers.networkBodies = append(c.telemetry.buffers.networkBodies, networkBodyEntry{
		Body:    makeNetworkBody(reqSize, respSize),
		AddedAt: time.Now(),
	})
	recalcMemoryTotals(c)
	c.telemetry.mu.Unlock()

	c.telemetry.mu.RLock()
	mem := c.telemetry.buffers.calcNBMemory()
	c.telemetry.mu.RUnlock()

	expectedMin := int64(reqSize + respSize + 50)
	expectedMax := int64(reqSize + respSize + 500)

	if mem < expectedMin || mem > expectedMax {
		t.Errorf("calcNBMemory() = %d, expected between %d and %d for %d+%d byte bodies",
			mem, expectedMin, expectedMax, reqSize, respSize)
	}
}

// ============================================
// Running Total Accuracy
// ============================================

func TestMemory_RunningTotal_WSAccurateAfterAdd(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	events := []types.WebSocketEvent{
		makeWSEvent(500),
		makeWSEvent(1000),
		makeWSEvent(2000),
	}
	c.Telemetry().AddWebSocketEvents(events)

	c.telemetry.mu.RLock()
	runningTotal := c.telemetry.buffers.wsMemoryTotal
	expected := bruteForceWSMemory(extractWSEvents(c.telemetry.buffers.wsEvents))
	c.telemetry.mu.RUnlock()

	if runningTotal != expected {
		t.Errorf("wsMemoryTotal = %d, brute force = %d", runningTotal, expected)
	}
}

func TestMemory_RunningTotal_NBAccurateAfterAdd(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	bodies := []types.NetworkBody{
		makeNetworkBody(500, 500),
		makeNetworkBody(1000, 2000),
	}
	c.Telemetry().AddNetworkBodies(bodies)

	c.telemetry.mu.RLock()
	runningTotal := c.telemetry.buffers.networkBodyMemoryTotal
	expected := bruteForceNBMemory(extractNetworkBodies(c.telemetry.buffers.networkBodies))
	c.telemetry.mu.RUnlock()

	if runningTotal != expected {
		t.Errorf("networkBodyMemoryTotal = %d, brute force = %d", runningTotal, expected)
	}
}

func TestMemory_RunningTotal_WSAccurateAfterRotation(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Fill to capacity, then add more to trigger ring buffer rotation
	events := make([]types.WebSocketEvent, MaxWSEvents+10)
	for i := range events {
		events[i] = makeWSEvent(100 + i)
	}
	c.Telemetry().AddWebSocketEvents(events)

	c.telemetry.mu.RLock()
	runningTotal := c.telemetry.buffers.wsMemoryTotal
	expected := bruteForceWSMemory(extractWSEvents(c.telemetry.buffers.wsEvents))
	count := len(c.telemetry.buffers.wsEvents)
	c.telemetry.mu.RUnlock()

	if count > MaxWSEvents {
		t.Errorf("expected at most %d events, got %d", MaxWSEvents, count)
	}
	if runningTotal != expected {
		t.Errorf("after rotation: wsMemoryTotal = %d, brute force = %d", runningTotal, expected)
	}
}

func TestMemory_RunningTotal_NBAccurateAfterRotation(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Fill to capacity, then add more to trigger ring buffer rotation
	bodies := make([]types.NetworkBody, MaxNetworkBodies+5)
	for i := range bodies {
		bodies[i] = makeNetworkBody(100+i, 200+i)
	}
	c.Telemetry().AddNetworkBodies(bodies)

	c.telemetry.mu.RLock()
	runningTotal := c.telemetry.buffers.networkBodyMemoryTotal
	expected := bruteForceNBMemory(extractNetworkBodies(c.telemetry.buffers.networkBodies))
	count := len(c.telemetry.buffers.networkBodies)
	c.telemetry.mu.RUnlock()

	if count > MaxNetworkBodies {
		t.Errorf("expected at most %d bodies, got %d", MaxNetworkBodies, count)
	}
	if runningTotal != expected {
		t.Errorf("after rotation: networkBodyMemoryTotal = %d, brute force = %d", runningTotal, expected)
	}
}

func TestMemory_RunningTotal_WSAccurateAfterPerBufferEviction(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Add events that exceed per-buffer WS memory limit (4MB)
	// Each event with 50KB data = ~50200 bytes; 100 events = ~5MB
	events := make([]types.WebSocketEvent, 100)
	for i := range events {
		events[i] = makeWSEvent(50000)
	}
	c.Telemetry().AddWebSocketEvents(events)

	c.telemetry.mu.RLock()
	runningTotal := c.telemetry.buffers.wsMemoryTotal
	expected := bruteForceWSMemory(extractWSEvents(c.telemetry.buffers.wsEvents))
	c.telemetry.mu.RUnlock()

	if runningTotal != expected {
		t.Errorf("after per-buffer WS eviction: wsMemoryTotal = %d, brute force = %d", runningTotal, expected)
	}
}

func TestMemory_RunningTotal_NBAccurateAfterPerBufferEviction(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Add bodies that exceed per-buffer NB memory limit (8MB)
	bodies := make([]types.NetworkBody, 100)
	for i := range bodies {
		bodies[i] = makeNetworkBody(maxRequestBodySize, maxResponseBodySize)
	}
	c.Telemetry().AddNetworkBodies(bodies)

	c.telemetry.mu.RLock()
	runningTotal := c.telemetry.buffers.networkBodyMemoryTotal
	expected := bruteForceNBMemory(extractNetworkBodies(c.telemetry.buffers.networkBodies))
	c.telemetry.mu.RUnlock()

	if runningTotal != expected {
		t.Errorf("after per-buffer NB eviction: networkBodyMemoryTotal = %d, brute force = %d", runningTotal, expected)
	}
}

func TestMemory_RunningTotal_ZeroAfterClearAll(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	c.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{makeWSEvent(1000), makeWSEvent(2000)})
	c.Telemetry().AddNetworkBodies([]types.NetworkBody{makeNetworkBody(500, 500), makeNetworkBody(1000, 1000)})

	c.telemetry.mu.RLock()
	wsBefore := c.telemetry.buffers.wsMemoryTotal
	nbBefore := c.telemetry.buffers.networkBodyMemoryTotal
	c.telemetry.mu.RUnlock()

	if wsBefore == 0 {
		t.Fatal("expected non-zero wsMemoryTotal before ClearAll")
	}
	if nbBefore == 0 {
		t.Fatal("expected non-zero networkBodyMemoryTotal before ClearAll")
	}

	c.ClearAll()

	c.telemetry.mu.RLock()
	wsAfter := c.telemetry.buffers.wsMemoryTotal
	nbAfter := c.telemetry.buffers.networkBodyMemoryTotal
	c.telemetry.mu.RUnlock()

	if wsAfter != 0 {
		t.Errorf("expected wsMemoryTotal = 0 after ClearAll, got %d", wsAfter)
	}
	if nbAfter != 0 {
		t.Errorf("expected networkBodyMemoryTotal = 0 after ClearAll, got %d", nbAfter)
	}
}

func TestMemory_CalcWSMemory_ReturnsRunningTotal(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	c.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{makeWSEvent(500), makeWSEvent(1000)})

	c.telemetry.mu.RLock()
	calcResult := c.telemetry.buffers.calcWSMemory()
	runningTotal := c.telemetry.buffers.wsMemoryTotal
	c.telemetry.mu.RUnlock()

	if calcResult != runningTotal {
		t.Errorf("calcWSMemory() = %d, wsMemoryTotal = %d; expected equal", calcResult, runningTotal)
	}
}

func TestMemory_CalcNBMemory_ReturnsRunningTotal(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	c.Telemetry().AddNetworkBodies([]types.NetworkBody{makeNetworkBody(500, 500)})

	c.telemetry.mu.RLock()
	calcResult := c.telemetry.buffers.calcNBMemory()
	runningTotal := c.telemetry.buffers.networkBodyMemoryTotal
	c.telemetry.mu.RUnlock()

	if calcResult != runningTotal {
		t.Errorf("calcNBMemory() = %d, networkBodyMemoryTotal = %d; expected equal", calcResult, runningTotal)
	}
}

func TestMemory_RunningTotal_MultipleAddEvictCycles(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Cycle 1: add events
	c.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{makeWSEvent(500), makeWSEvent(1000)})
	c.Telemetry().AddNetworkBodies([]types.NetworkBody{makeNetworkBody(200, 300)})

	// Cycle 2: add more (may trigger rotation if near capacity)
	for i := 0; i < 5; i++ {
		c.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{makeWSEvent(100 * (i + 1))})
		c.Telemetry().AddNetworkBodies([]types.NetworkBody{makeNetworkBody(50*(i+1), 75*(i+1))})
	}

	c.telemetry.mu.RLock()
	wsRunning := c.telemetry.buffers.wsMemoryTotal
	wsExpected := bruteForceWSMemory(extractWSEvents(c.telemetry.buffers.wsEvents))
	nbRunning := c.telemetry.buffers.networkBodyMemoryTotal
	nbExpected := bruteForceNBMemory(extractNetworkBodies(c.telemetry.buffers.networkBodies))
	c.telemetry.mu.RUnlock()

	if wsRunning != wsExpected {
		t.Errorf("after multiple cycles: wsMemoryTotal = %d, brute force = %d", wsRunning, wsExpected)
	}
	if nbRunning != nbExpected {
		t.Errorf("after multiple cycles: networkBodyMemoryTotal = %d, brute force = %d", nbRunning, nbExpected)
	}
}
