// Purpose: Tests for capture memory accounting and limits.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"testing"
	"time"
)

// ============================================
// Per-Buffer Memory Tracking Tests
// ============================================

// Helper: create a WebSocketEvent with a specific data size
func makeWSEvent(dataSize int) WebSocketEvent {
	data := make([]byte, dataSize)
	for i := range data {
		data[i] = 'x'
	}
	return WebSocketEvent{
		ID:        "conn-1",
		Event:     "message",
		Direction: "incoming",
		Data:      string(data),
		Timestamp: time.Now().Format(time.RFC3339Nano),
	}
}

// Helper: create a NetworkBody with specific body sizes
func makeNetworkBody(reqSize, respSize int) NetworkBody {
	reqBody := make([]byte, reqSize)
	for i := range reqBody {
		reqBody[i] = 'r'
	}
	respBody := make([]byte, respSize)
	for i := range respBody {
		respBody[i] = 'R'
	}
	return NetworkBody{
		Method:       "GET",
		URL:          "http://example.com/api",
		Status:       200,
		RequestBody:  string(reqBody),
		ResponseBody: string(respBody),
	}
}

// Helper: create an EnhancedAction
func makeAction() EnhancedAction {
	return EnhancedAction{
		Type:      "click",
		Timestamp: time.Now().UnixMilli(),
		URL:       "http://example.com",
	}
}

// Helper: recalculate running memory totals from current slices.
// Must be called with lock held.
func recalcMemoryTotals(c *Capture) {
	c.buffers.wsMemoryTotal = 0
	for i := range c.buffers.wsEvents {
		c.buffers.wsMemoryTotal += wsEventMemory(&c.buffers.wsEvents[i].Event)
	}
	c.buffers.networkBodyMemoryTotal = 0
	for i := range c.buffers.networkBodies {
		c.buffers.networkBodyMemoryTotal += nbEntryMemory(&c.buffers.networkBodies[i].Body)
	}
}

// extractWSEvents extracts WebSocketEvent values from entry wrappers (test helper).
func extractWSEvents(entries []wsEventEntry) []WebSocketEvent {
	out := make([]WebSocketEvent, len(entries))
	for i := range entries {
		out[i] = entries[i].Event
	}
	return out
}

// extractNetworkBodies extracts NetworkBody values from entry wrappers (test helper).
func extractNetworkBodies(entries []networkBodyEntry) []NetworkBody {
	out := make([]NetworkBody, len(entries))
	for i := range entries {
		out[i] = entries[i].Body
	}
	return out
}

// bruteForceWSMemory recalculates WS memory by iterating all events (reference implementation).
func bruteForceWSMemory(events []WebSocketEvent) int64 {
	var total int64
	for i := range events {
		total += int64(len(events[i].Data)) + wsEventOverhead
	}
	return total
}

// bruteForceNBMemory recalculates NB memory by iterating all bodies (reference implementation).
func bruteForceNBMemory(bodies []NetworkBody) int64 {
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
	c.mu.Lock()
	c.buffers.wsEvents = append(c.buffers.wsEvents, wsEventEntry{
		Event:   makeWSEvent(dataSize),
		AddedAt: time.Now(),
	})
	recalcMemoryTotals(c)
	c.mu.Unlock()

	c.mu.RLock()
	mem := c.buffers.calcWSMemory()
	c.mu.RUnlock()

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
	c.mu.Lock()
	c.buffers.networkBodies = append(c.buffers.networkBodies, networkBodyEntry{
		Body:    makeNetworkBody(reqSize, respSize),
		AddedAt: time.Now(),
	})
	recalcMemoryTotals(c)
	c.mu.Unlock()

	c.mu.RLock()
	mem := c.buffers.calcNBMemory()
	c.mu.RUnlock()

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

	events := []WebSocketEvent{
		makeWSEvent(500),
		makeWSEvent(1000),
		makeWSEvent(2000),
	}
	c.AddWebSocketEvents(events)

	c.mu.RLock()
	runningTotal := c.buffers.wsMemoryTotal
	expected := bruteForceWSMemory(extractWSEvents(c.buffers.wsEvents))
	c.mu.RUnlock()

	if runningTotal != expected {
		t.Errorf("wsMemoryTotal = %d, brute force = %d", runningTotal, expected)
	}
}

func TestMemory_RunningTotal_NBAccurateAfterAdd(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	bodies := []NetworkBody{
		makeNetworkBody(500, 500),
		makeNetworkBody(1000, 2000),
	}
	c.AddNetworkBodies(bodies)

	c.mu.RLock()
	runningTotal := c.buffers.networkBodyMemoryTotal
	expected := bruteForceNBMemory(extractNetworkBodies(c.buffers.networkBodies))
	c.mu.RUnlock()

	if runningTotal != expected {
		t.Errorf("networkBodyMemoryTotal = %d, brute force = %d", runningTotal, expected)
	}
}

func TestMemory_RunningTotal_WSAccurateAfterRotation(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Fill to capacity, then add more to trigger ring buffer rotation
	events := make([]WebSocketEvent, MaxWSEvents+10)
	for i := range events {
		events[i] = makeWSEvent(100 + i)
	}
	c.AddWebSocketEvents(events)

	c.mu.RLock()
	runningTotal := c.buffers.wsMemoryTotal
	expected := bruteForceWSMemory(extractWSEvents(c.buffers.wsEvents))
	count := len(c.buffers.wsEvents)
	c.mu.RUnlock()

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
	bodies := make([]NetworkBody, MaxNetworkBodies+5)
	for i := range bodies {
		bodies[i] = makeNetworkBody(100+i, 200+i)
	}
	c.AddNetworkBodies(bodies)

	c.mu.RLock()
	runningTotal := c.buffers.networkBodyMemoryTotal
	expected := bruteForceNBMemory(extractNetworkBodies(c.buffers.networkBodies))
	count := len(c.buffers.networkBodies)
	c.mu.RUnlock()

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
	events := make([]WebSocketEvent, 100)
	for i := range events {
		events[i] = makeWSEvent(50000)
	}
	c.AddWebSocketEvents(events)

	c.mu.RLock()
	runningTotal := c.buffers.wsMemoryTotal
	expected := bruteForceWSMemory(extractWSEvents(c.buffers.wsEvents))
	c.mu.RUnlock()

	if runningTotal != expected {
		t.Errorf("after per-buffer WS eviction: wsMemoryTotal = %d, brute force = %d", runningTotal, expected)
	}
}

func TestMemory_RunningTotal_NBAccurateAfterPerBufferEviction(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Add bodies that exceed per-buffer NB memory limit (8MB)
	bodies := make([]NetworkBody, 100)
	for i := range bodies {
		bodies[i] = makeNetworkBody(maxRequestBodySize, maxResponseBodySize)
	}
	c.AddNetworkBodies(bodies)

	c.mu.RLock()
	runningTotal := c.buffers.networkBodyMemoryTotal
	expected := bruteForceNBMemory(extractNetworkBodies(c.buffers.networkBodies))
	c.mu.RUnlock()

	if runningTotal != expected {
		t.Errorf("after per-buffer NB eviction: networkBodyMemoryTotal = %d, brute force = %d", runningTotal, expected)
	}
}

func TestMemory_RunningTotal_ZeroAfterClearAll(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	c.AddWebSocketEvents([]WebSocketEvent{makeWSEvent(1000), makeWSEvent(2000)})
	c.AddNetworkBodies([]NetworkBody{makeNetworkBody(500, 500), makeNetworkBody(1000, 1000)})

	c.mu.RLock()
	wsBefore := c.buffers.wsMemoryTotal
	nbBefore := c.buffers.networkBodyMemoryTotal
	c.mu.RUnlock()

	if wsBefore == 0 {
		t.Fatal("expected non-zero wsMemoryTotal before ClearAll")
	}
	if nbBefore == 0 {
		t.Fatal("expected non-zero networkBodyMemoryTotal before ClearAll")
	}

	c.ClearAll()

	c.mu.RLock()
	wsAfter := c.buffers.wsMemoryTotal
	nbAfter := c.buffers.networkBodyMemoryTotal
	c.mu.RUnlock()

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

	c.AddWebSocketEvents([]WebSocketEvent{makeWSEvent(500), makeWSEvent(1000)})

	c.mu.RLock()
	calcResult := c.buffers.calcWSMemory()
	runningTotal := c.buffers.wsMemoryTotal
	c.mu.RUnlock()

	if calcResult != runningTotal {
		t.Errorf("calcWSMemory() = %d, wsMemoryTotal = %d; expected equal", calcResult, runningTotal)
	}
}

func TestMemory_CalcNBMemory_ReturnsRunningTotal(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	c.AddNetworkBodies([]NetworkBody{makeNetworkBody(500, 500)})

	c.mu.RLock()
	calcResult := c.buffers.calcNBMemory()
	runningTotal := c.buffers.networkBodyMemoryTotal
	c.mu.RUnlock()

	if calcResult != runningTotal {
		t.Errorf("calcNBMemory() = %d, networkBodyMemoryTotal = %d; expected equal", calcResult, runningTotal)
	}
}

func TestMemory_RunningTotal_MultipleAddEvictCycles(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Cycle 1: add events
	c.AddWebSocketEvents([]WebSocketEvent{makeWSEvent(500), makeWSEvent(1000)})
	c.AddNetworkBodies([]NetworkBody{makeNetworkBody(200, 300)})

	// Cycle 2: add more (may trigger rotation if near capacity)
	for i := 0; i < 5; i++ {
		c.AddWebSocketEvents([]WebSocketEvent{makeWSEvent(100 * (i + 1))})
		c.AddNetworkBodies([]NetworkBody{makeNetworkBody(50*(i+1), 75*(i+1))})
	}

	c.mu.RLock()
	wsRunning := c.buffers.wsMemoryTotal
	wsExpected := bruteForceWSMemory(extractWSEvents(c.buffers.wsEvents))
	nbRunning := c.buffers.networkBodyMemoryTotal
	nbExpected := bruteForceNBMemory(extractNetworkBodies(c.buffers.networkBodies))
	c.mu.RUnlock()

	if wsRunning != wsExpected {
		t.Errorf("after multiple cycles: wsMemoryTotal = %d, brute force = %d", wsRunning, wsExpected)
	}
	if nbRunning != nbExpected {
		t.Errorf("after multiple cycles: networkBodyMemoryTotal = %d, brute force = %d", nbRunning, nbExpected)
	}
}
