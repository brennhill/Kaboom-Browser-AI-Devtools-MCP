// Purpose: Implements ring buffer storage primitives and cursor-safe access patterns.
// Why: Prevents unbounded memory growth while preserving recent evidence for debugging.
// Docs: docs/features/feature/ring-buffer/index.md

package buffers

import (
	"sync"
	"time"
)

// ============================================
// BufferCursor
// ============================================

// BufferCursor tracks a read position in a ring buffer.
// The timestamp allows detecting when the buffer has wrapped and
// the position is no longer valid (all data at that position has been evicted).
type BufferCursor struct {
	Position  int64     // Monotonic position in the buffer (total items ever added)
	Timestamp time.Time // When this position was last valid
}

// ============================================
// RingBuffer
// ============================================

// RingBuffer is a generic fixed-capacity circular buffer with timestamp tracking.
// Entries are evicted in FIFO order when capacity is reached.
// Supports cursor-based reads so multiple clients can maintain independent read positions.
type RingBuffer[T any] struct {
	mu sync.RWMutex

	// Buffer storage
	entries  []T
	addedAt  []time.Time // Parallel slice: when each entry was added
	capacity int
	now      func() time.Time

	// Position tracking
	totalAdded int64 // Monotonic counter of all entries ever added
	head       int   // Index where next write goes
}

// NewRingBuffer creates a new ring buffer with the given capacity.
func NewRingBuffer[T any](capacity int) *RingBuffer[T] {
	return &RingBuffer[T]{
		entries:  make([]T, 0, capacity),
		addedAt:  make([]time.Time, 0, capacity),
		capacity: capacity,
		now:      time.Now,
	}
}

// Write appends entries to the buffer, evicting oldest entries if at capacity.
// Returns the number of entries actually written (may be less if entries > capacity).
func (rb *RingBuffer[T]) Write(entries []T) int {
	if len(entries) == 0 {
		return 0
	}

	rb.mu.Lock()
	defer rb.mu.Unlock()

	now := rb.now()
	written := 0

	for _, entry := range entries {
		rb.writeOneLocked(entry, now)
		written++
	}

	return written
}

// WriteOne appends a single entry to the buffer.
func (rb *RingBuffer[T]) WriteOne(entry T) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.writeOneLocked(entry, rb.now())
}

// writeOneLocked adds one entry, must be called with mu held.
func (rb *RingBuffer[T]) writeOneLocked(entry T, addedAt time.Time) {
	if len(rb.entries) < rb.capacity {
		rb.entries = append(rb.entries, entry)
		rb.addedAt = append(rb.addedAt, addedAt)
	} else {
		rb.entries[rb.head] = entry
		rb.addedAt[rb.head] = addedAt
	}
	rb.head = (rb.head + 1) % rb.capacity
	rb.totalAdded++
}

// ReadFrom reads entries starting from the given cursor position.
// Returns entries added after the cursor position and a new cursor for subsequent reads.
// If the cursor position has been evicted (buffer wrapped), reads from the oldest available.
func (rb *RingBuffer[T]) ReadFrom(cursor BufferCursor) ([]T, BufferCursor) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if len(rb.entries) == 0 {
		return nil, BufferCursor{Position: rb.totalAdded, Timestamp: rb.now()}
	}

	oldestPosition := rb.totalAdded - int64(len(rb.entries))
	if oldestPosition < 0 {
		oldestPosition = 0
	}

	startPosition := cursor.Position
	if startPosition < oldestPosition {
		startPosition = oldestPosition
	}

	entriesAvailable := rb.totalAdded - startPosition
	if entriesAvailable <= 0 {
		return nil, BufferCursor{Position: rb.totalAdded, Timestamp: rb.now()}
	}

	startIndex := rb.positionToIndex(startPosition)

	result := make([]T, 0, entriesAvailable)
	for i := int64(0); i < entriesAvailable; i++ {
		idx := int((int64(startIndex) + i) % int64(len(rb.entries)))
		result = append(result, rb.entries[idx])
	}

	return result, BufferCursor{Position: rb.totalAdded, Timestamp: rb.now()}
}

// ReadAll returns all entries currently in the buffer, oldest first.
func (rb *RingBuffer[T]) ReadAll() []T {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if len(rb.entries) == 0 {
		return nil
	}

	result := make([]T, len(rb.entries))
	if len(rb.entries) < rb.capacity {
		copy(result, rb.entries)
	} else {
		n := copy(result, rb.entries[rb.head:])
		copy(result[n:], rb.entries[:rb.head])
	}
	return result
}

// ReadLast returns the last n entries, oldest first.
func (rb *RingBuffer[T]) ReadLast(n int) []T {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if len(rb.entries) == 0 || n <= 0 {
		return nil
	}

	if n > len(rb.entries) {
		n = len(rb.entries)
	}

	result := make([]T, n)

	if len(rb.entries) < rb.capacity {
		startIdx := len(rb.entries) - n
		copy(result, rb.entries[startIdx:])
	} else {
		endIdx := (rb.head - 1 + rb.capacity) % rb.capacity
		for i := n - 1; i >= 0; i-- {
			result[i] = rb.entries[endIdx]
			endIdx = (endIdx - 1 + rb.capacity) % rb.capacity
		}
	}
	return result
}

// FindPositionAtTime returns the position of the first entry added at or after the given time.
// Returns -1 if no entries are at or after the given time.
func (rb *RingBuffer[T]) FindPositionAtTime(t time.Time) int64 {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if len(rb.entries) == 0 {
		return -1
	}

	oldestPosition := rb.totalAdded - int64(len(rb.entries))
	if oldestPosition < 0 {
		oldestPosition = 0
	}

	for i := 0; i < len(rb.entries); i++ {
		idx := rb.positionToIndex(oldestPosition + int64(i))
		if !rb.addedAt[idx].Before(t) {
			return oldestPosition + int64(i)
		}
	}

	return -1
}

// GetCurrentPosition returns the current total added count (next write position).
func (rb *RingBuffer[T]) GetCurrentPosition() int64 {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.totalAdded
}

// Len returns the number of entries currently in the buffer.
func (rb *RingBuffer[T]) Len() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return len(rb.entries)
}

// Cap returns the buffer capacity.
func (rb *RingBuffer[T]) Cap() int {
	return rb.capacity // Immutable, no lock needed
}

// Clear removes all entries from the buffer, preserving pre-allocated capacity.
func (rb *RingBuffer[T]) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.entries = make([]T, 0, rb.capacity)
	rb.addedAt = make([]time.Time, 0, rb.capacity)
	rb.head = 0
	// Don't reset totalAdded - cursors need it to stay monotonic.
}

// positionToIndex converts a monotonic position to a buffer index.
// Must be called with at least a read lock held.
func (rb *RingBuffer[T]) positionToIndex(position int64) int {
	oldestPosition := rb.totalAdded - int64(len(rb.entries))
	if len(rb.entries) < rb.capacity {
		// Entries are stored at indices 0..len-1, oldest first. Offsetting by
		// oldestPosition matters after Clear(): totalAdded keeps counting, so
		// position alone would index past the slice (panic) or read wrong entries.
		return int(position - oldestPosition)
	}
	offset := int(position - oldestPosition)
	return (rb.head + offset) % rb.capacity
}
