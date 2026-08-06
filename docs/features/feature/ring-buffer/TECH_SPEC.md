---
status: draft
priority: tier-1
phase: v6.0-sprint-1
relates-to: [browser-extension-enhancement, normalized-event-schema]
blocks: [query-service, checkpoint-system]
last-updated: 2026-01-31
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Ring Buffer Implementation — Technical Specification

**Goal:** Implement bounded, circular in-memory event buffers with O(1) operations and TTL-based cleanup for v6.0.

---

## Problem Statement

The query service (v6.0 sprint 2) needs queryable event history, but:

1. **Memory is bounded:** Can't store infinite events on laptops
2. **Events arrive continuously:** Browser logs, backend logs, tests all stream in
3. **Queries need speed:** LLM can't wait for disk I/O
4. **No events should be lost:** During normal operation, buffer must never drop data

**Solution:** Ring buffers per event stream with:
- Circular design (O(1) push/pop)
- Bounded capacity (10K-50K events per stream)
- TTL cleanup (24-hour retention)
- Async writer (non-blocking)
- Snapshot capability (for checkpoints)

---

## Architecture

### Multiple Ring Buffers Per Stream

```
RingBufferManager
├─ Browser Logs (capacity: 10K, TTL: 24h)
│  ├─ console.log events
│  ├─ console.warn events
│  └─ console.error events
│
├─ Browser Network (capacity: 5K, TTL: 24h)
│  ├─ HTTP requests
│  ├─ HTTP responses
│  └─ Failed requests (4xx, 5xx)
│
├─ Browser DOM (capacity: 2K, TTL: 24h)
│  └─ DOM snapshots
│
├─ Browser Actions (capacity: 5K, TTL: 24h)
│  ├─ Click events
│  ├─ Type events
│  └─ Navigation events
│
├─ Backend Logs (capacity: 50K, TTL: 24h)
│  ├─ stdout logs
│  ├─ stderr logs
│  └─ Structured logs
│
├─ Backend Network (capacity: 10K, TTL: 24h)
│  ├─ Incoming requests
│  └─ Outgoing requests
│
├─ Test Events (capacity: 5K, TTL: 24h)
│  ├─ Test start
│  ├─ Test pass/fail
│  └─ Test output
│
└─ Custom Events (capacity: 10K, TTL: 24h)
   └─ window.__kaboom.annotate() calls
```

Total capacity: ~112K events max
Total memory: ~200MB (at 2KB per event average)

### Individual Ring Buffer

```typescript
interface RingBuffer<T> {
  capacity: number;
  ttl_ms: number;
  data: T[];
  head: number;          // Next write position
  tail: number;          // Oldest event position
  size: number;          // Current count
  created_at: number;
  last_cleanup: number;
}
```

#### Operations:
- `push(event)`: Add event, O(1)
- `pop()`: Remove oldest, O(1)
- `query(filters)`: All events matching filters, O(n) where n = size
- `snapshot()`: Clone for checkpoint, O(n)
- `clear()`: Empty buffer, O(1)
- `stats()`: Return count, capacity, etc, O(1)

---

## Go Implementation

**File:** `pkg/buffer/ring_buffer.go`

```go
package buffer

import (
	"sync"
	"time"
)

// NormalizedEvent is the unified event type for all sources
type NormalizedEvent struct {
	ID            string                 `json:"id"`
	Timestamp     int64                  `json:"timestamp"`
	Source        string                 `json:"source"` // "browser", "backend", "test", "git"
	Level         string                 `json:"level"`  // "debug", "info", "warn", "error", "critical"
	CorrelationID string                 `json:"correlation_id,omitempty"`
	Message       string                 `json:"message"`
	Metadata      map[string]interface{} `json:"metadata"`
	Tags          []string               `json:"tags"`
}

// RingBuffer is a circular, bounded buffer for events
type RingBuffer struct {
	mu        sync.RWMutex
	capacity  int
	ttlMs     int64
	data      []NormalizedEvent
	head      int       // Next write position
	tail      int       // Oldest event position
	size      int       // Current count
	createdAt time.Time
	lastCleanup time.Time
	pushCount int64     // Total pushes (for metrics)
	popCount  int64     // Total pops (for metrics)
}

// NewRingBuffer creates a new circular buffer
func NewRingBuffer(capacity int, ttlMs int64) *RingBuffer {
	return &RingBuffer{
		capacity:    capacity,
		ttlMs:       ttlMs,
		data:        make([]NormalizedEvent, capacity),
		head:        0,
		tail:        0,
		size:        0,
		createdAt:   time.Now(),
		lastCleanup: time.Now(),
	}
}

// Push adds an event to the buffer
// O(1) operation
// If buffer full, oldest event is overwritten (graceful overflow)
func (rb *RingBuffer) Push(event NormalizedEvent) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	// Cleanup expired events periodically
	if time.Since(rb.lastCleanup) > 5*time.Second {
		rb.cleanupExpired()
	}

	rb.data[rb.head] = event
	rb.head = (rb.head + 1) % rb.capacity

	if rb.size < rb.capacity {
		rb.size++
	} else {
		// Buffer full, move tail forward (we're overwriting oldest)
		rb.tail = (rb.tail + 1) % rb.capacity
		rb.popCount++
	}

	rb.pushCount++
}

// Pop removes and returns the oldest event
// O(1) operation
func (rb *RingBuffer) Pop() *NormalizedEvent {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size == 0 {
		return nil
	}

	event := rb.data[rb.tail]
	rb.tail = (rb.tail + 1) % rb.capacity
	rb.size--
	rb.popCount++

	return &event
}

// Query returns all events matching filters
// O(n) operation where n = size
// Returns copies to prevent external modification
func (rb *RingBuffer) Query(filter func(NormalizedEvent) bool) []NormalizedEvent {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	result := make([]NormalizedEvent, 0, rb.size)

	// Iterate from tail (oldest) to head (newest)
	for i := 0; i < rb.size; i++ {
		idx := (rb.tail + i) % rb.capacity
		event := rb.data[idx]

		if filter(event) {
			result = append(result, event)
		}
	}

	return result
}

// Timeline returns all events in chronological order
// O(n) operation
func (rb *RingBuffer) Timeline(limit int, offset int) []NormalizedEvent {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.size == 0 {
		return []NormalizedEvent{}
	}

	// Start from oldest (tail)
	events := make([]NormalizedEvent, 0, rb.size)
	for i := 0; i < rb.size; i++ {
		idx := (rb.tail + i) % rb.capacity
		events = append(events, rb.data[idx])
	}

	// Apply offset and limit
	if offset > len(events) {
		return []NormalizedEvent{}
	}

	end := offset + limit
	if end > len(events) {
		end = len(events)
	}

	return events[offset:end]
}

// Snapshot creates a copy of all events for checkpointing
// O(n) operation
func (rb *RingBuffer) Snapshot() []NormalizedEvent {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	snapshot := make([]NormalizedEvent, rb.size)

	for i := 0; i < rb.size; i++ {
		idx := (rb.tail + i) % rb.capacity
		snapshot[i] = rb.data[idx]
	}

	return snapshot
}

// Clear removes all events
// O(1) operation
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.head = 0
	rb.tail = 0
	rb.size = 0
}

// Stats returns buffer statistics
func (rb *RingBuffer) Stats() map[string]interface{} {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	return map[string]interface{}{
		"capacity":       rb.capacity,
		"size":           rb.size,
		"utilization":    float64(rb.size) / float64(rb.capacity),
		"ttl_hours":      rb.ttlMs / (1000 * 60 * 60),
		"total_pushed":   rb.pushCount,
		"total_popped":   rb.popCount,
		"age_seconds":    time.Since(rb.createdAt).Seconds(),
		"last_cleanup":   rb.lastCleanup.Unix(),
	}
}

// cleanupExpired removes events older than TTL
// Call this periodically during Push
// O(m) where m = number of expired events
func (rb *RingBuffer) cleanupExpired() {
	now := time.Now().UnixMilli()
	ttlThreshold := now - rb.ttlMs

	// Count how many events are expired from tail
	expiredCount := 0
	for i := 0; i < rb.size; i++ {
		idx := (rb.tail + i) % rb.capacity
		if rb.data[idx].Timestamp < ttlThreshold {
			expiredCount++
		} else {
			break // Remaining events are newer
		}
	}

	// Move tail forward to skip expired events
	if expiredCount > 0 {
		rb.tail = (rb.tail + expiredCount) % rb.capacity
		rb.size -= expiredCount
		rb.popCount += int64(expiredCount)
	}

	rb.lastCleanup = time.Now()
}

// Size returns current event count
func (rb *RingBuffer) Size() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	return rb.size
}

// Capacity returns buffer capacity
func (rb *RingBuffer) Capacity() int {
	return rb.capacity
}
```

---

## Ring Buffer Manager

**File:** `pkg/buffer/manager.go`

```go
package buffer

// BufferManager manages multiple ring buffers per stream
type BufferManager struct {
	buffers map[string]*RingBuffer
	mu      sync.RWMutex
}

// NewBufferManager creates a new manager with pre-configured buffers
func NewBufferManager() *BufferManager {
	bm := &BufferManager{
		buffers: make(map[string]*RingBuffer),
	}

	// Create buffers per stream
	bm.buffers["browser_logs"] = NewRingBuffer(10000, 24*60*60*1000)      // 10K events, 24h
	bm.buffers["browser_network"] = NewRingBuffer(5000, 24*60*60*1000)    // 5K events, 24h
	bm.buffers["browser_actions"] = NewRingBuffer(5000, 24*60*60*1000)    // 5K events, 24h
	bm.buffers["browser_dom"] = NewRingBuffer(2000, 24*60*60*1000)        // 2K events, 24h
	bm.buffers["backend_logs"] = NewRingBuffer(50000, 24*60*60*1000)      // 50K events, 24h
	bm.buffers["backend_network"] = NewRingBuffer(10000, 24*60*60*1000)   // 10K events, 24h
	bm.buffers["test_events"] = NewRingBuffer(5000, 24*60*60*1000)        // 5K events, 24h
	bm.buffers["custom_events"] = NewRingBuffer(10000, 24*60*60*1000)     // 10K events, 24h

	return bm
}

// Push adds event to appropriate buffer based on source
func (bm *BufferManager) Push(event NormalizedEvent) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	// Route to appropriate buffer
	streamName := bm.routeToStream(event)
	if buf, ok := bm.buffers[streamName]; ok {
		buf.Push(event)
	}
}

// routeToStream determines which buffer receives the event
func (bm *BufferManager) routeToStream(event NormalizedEvent) string {
	// Route based on source and metadata
	switch event.Source {
	case "browser":
		if eventType, ok := event.Metadata["event_type"].(string); ok {
			switch eventType {
			case "log":
				return "browser_logs"
			case "network":
				return "browser_network"
			case "action":
				return "browser_actions"
			case "snapshot":
				return "browser_dom"
			}
		}
		return "browser_logs"

	case "backend":
		if eventType, ok := event.Metadata["event_type"].(string); ok {
			if eventType == "network" {
				return "backend_network"
			}
		}
		return "backend_logs"

	case "test":
		return "test_events"

	case "custom":
		return "custom_events"

	default:
		return "browser_logs"
	}
}

// Query searches all buffers
func (bm *BufferManager) Query(filter func(NormalizedEvent) bool) []NormalizedEvent {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var result []NormalizedEvent

	for _, buf := range bm.buffers {
		result = append(result, buf.Query(filter)...)
	}

	// Sort by timestamp
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp < result[j].Timestamp
	})

	return result
}

// Timeline returns all events in chronological order
func (bm *BufferManager) Timeline(limit int, offset int) []NormalizedEvent {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	// Collect from all buffers
	allEvents := make([]NormalizedEvent, 0)
	for _, buf := range bm.buffers {
		allEvents = append(allEvents, buf.Timeline(buf.Size(), 0)...)
	}

	// Sort by timestamp
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Timestamp < allEvents[j].Timestamp
	})

	// Apply offset and limit
	if offset > len(allEvents) {
		return []NormalizedEvent{}
	}

	end := offset + limit
	if end > len(allEvents) {
		end = len(allEvents)
	}

	return allEvents[offset:end]
}

// Snapshot captures current state of all buffers for checkpointing
func (bm *BufferManager) Snapshot() map[string][]NormalizedEvent {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	snapshot := make(map[string][]NormalizedEvent)

	for name, buf := range bm.buffers {
		snapshot[name] = buf.Snapshot()
	}

	return snapshot
}

// Stats returns statistics for all buffers
func (bm *BufferManager) Stats() map[string]interface{} {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	totalSize := 0
	totalCapacity := 0
	bufferStats := make(map[string]interface{})

	for name, buf := range bm.buffers {
		stats := buf.Stats()
		bufferStats[name] = stats
		totalSize += buf.Size()
		totalCapacity += buf.Capacity()
	}

	return map[string]interface{}{
		"buffers":           bufferStats,
		"total_events":      totalSize,
		"total_capacity":    totalCapacity,
		"total_utilization": float64(totalSize) / float64(totalCapacity),
	}
}

// Clear empties all buffers
func (bm *BufferManager) Clear() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	for _, buf := range bm.buffers {
		buf.Clear()
	}
}
```

---

## Performance Characteristics

### Time Complexity
- `Push`: O(1) amortized (occasional TTL cleanup is O(m) where m << n)
- `Pop`: O(1)
- `Query`: O(n) where n = size
- `Snapshot`: O(n)
- `Timeline`: O(n log n) due to sorting

### Space Complexity
- Total: O(capacity × avg_event_size)
- Per-buffer: Fixed at `capacity × ~2KB = 2-100MB`
- Metadata: Negligible

### Memory Footprint
```
Browser Logs:    10K × 2KB = 20MB
Browser Network:  5K × 2KB = 10MB
Browser Actions:  5K × 2KB = 10MB
Browser DOM:      2K × 2KB = 4MB
Backend Logs:    50K × 2KB = 100MB
Backend Network: 10K × 2KB = 20MB
Test Events:      5K × 2KB = 10MB
Custom Events:   10K × 2KB = 20MB
─────────────────────────────────
Total:                    ~200MB
```

### Throughput
- Push: <0.1ms per event (no I/O)
- Pop: <0.1ms per event
- Query: ~10ms for 10K events (single buffer full scan)
- Snapshot: ~50ms for all buffers (full copy)

---

## Concurrency & Thread Safety

### Design:
- Sync.RWMutex for lock-free reads (multiple Query calls)
- Exclusive lock only during Push/Pop
- TTL cleanup is non-blocking (happens during Push, every 5 seconds)

### Lock Contention:
- High read concurrency: Multiple LLM queries can run in parallel
- Low write contention: Browser/backend streams rarely conflict
- No deadlocks: Single lock per buffer, no inter-buffer locking

### Testing:
- [ ] Concurrent push/query stress test (1000 threads)
- [ ] Concurrent snapshot during active pushing
- [ ] No data loss under contention
- [ ] Lock fairness (queries don't starve pushes)

---

## Checkpoint Integration

Checkpoints use `Snapshot()` to capture ring buffer state:

```typescript
interface Checkpoint {
  id: string;
  timestamp: number;
  duration_ms: number;
  description: string;
  buffers: Map<string, NormalizedEvent[]>;
}
```

When comparing checkpoints:
```
baseline = checkpoint("good state").buffers["browser_logs"]
current = checkpoint("after feature").buffers["browser_logs"]

diff = Compare(baseline, current)
// Returns: events in current but not baseline (regressions)
```

---

## HTTP API

**File:** `internal/api/buffers.go`

```go
// GET /buffers/timeline?limit=100&offset=0
// Returns all events in chronological order

// GET /buffers/query?source=browser&level=error
// Returns filtered events

// GET /buffers/stats
// Returns buffer utilization and stats

// POST /buffers/snapshot
// Creates checkpoint

// POST /buffers/clear
// Empties all buffers
```

---

## Testing Strategy

### Unit Tests:
- [ ] Push and pop correctness
- [ ] Circular wraparound at capacity
- [ ] TTL cleanup removes expired events
- [ ] Query filter accuracy
- [ ] Snapshot completeness
- [ ] Memory safety (no buffer overflow)

### Concurrency Tests:
- [ ] 1000 concurrent pushes, no data loss
- [ ] Query during active push
- [ ] Snapshot during push
- [ ] No deadlocks

### Performance Tests:
- [ ] Push latency <0.1ms
- [ ] Query on full buffer <50ms
- [ ] Memory footprint <200MB
- [ ] Cleanup doesn't block pushes

### Integration Tests:
- [ ] Browser events flow to buffer
- [ ] Backend events flow to buffer
- [ ] Events retrievable via HTTP API
- [ ] Checkpoint save/restore works

---

## Rollout Plan

### Sprint 1 (v6.0):
- Implement RingBuffer
- Implement BufferManager
- Integrate with browser extension (push events)
- HTTP API for testing
- Performance benchmarks

### Sprint 2 (v6.0):
- Integrate with backend log streaming (A2)
- Checkpoint system (B1)
- Query service uses buffers
- Performance tuning

### Future (v6.1+):
- Distributed ring buffers across processes
- Persistent snapshots (optional SSD cache)
- Compression for long-term storage

---

## Related Documents

- **Architecture:** [360-observability-architecture.md](../../../core/architecture/360-observability-architecture.md#storage-ring-buffers)
- **Schema:** [normalized-event-schema/TECH_SPEC.md](../normalized-event-schema/TECH_SPEC.md)
- **Sequencing:** [implementation-sequencing.md](../../../core/architecture/implementation-sequencing.md#sprint-a1-browser-extension--buffer-layer-week-1)

---

**Status:** Ready for implementation
**Estimated Effort:** 3 days (integrated with browser enhancement)
**Dependencies:** None (pure data structure)
