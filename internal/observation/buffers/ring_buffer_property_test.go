// Purpose: Property-based tests for ring buffer invariants.
// Docs: docs/features/feature/ring-buffer/index.md

// ring_buffer_property_test.go — Property-based tests for ring buffer.

package buffers

import (
	"testing"
	"testing/quick"
)

// TestPropertyCapacityBound verifies that after writing N items, Len() <= Cap() always.
func TestPropertyCapacityBound(t *testing.T) {
	f := func(items []int, capacityOffset uint8) bool {
		// Generate capacity as uint8+1 (1-256)
		capacity := int(capacityOffset) + 1
		if capacity <= 0 {
			capacity = 1
		}

		rb := NewRingBuffer[int](capacity)

		// Write all items
		for _, item := range items {
			rb.WriteOne(item)
		}

		// Verify length never exceeds capacity
		return rb.Len() <= rb.Cap()
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyWriteReadConsistency verifies that ReadAll returns items where
// the last min(N, C) items match what was written.
func TestPropertyWriteReadConsistency(t *testing.T) {
	f := func(items []int, capacityOffset uint8) bool {
		if len(items) == 0 {
			return true // Vacuously true for empty input
		}

		// Generate capacity as uint8+1 (1-256)
		capacity := int(capacityOffset) + 1
		if capacity <= 0 {
			capacity = 1
		}

		rb := NewRingBuffer[int](capacity)

		// Write all items
		for _, item := range items {
			rb.WriteOne(item)
		}

		// Read all items back
		readItems := rb.ReadAll()

		// Determine expected count
		expectedCount := len(items)
		if expectedCount > capacity {
			expectedCount = capacity
		}

		// Verify count matches
		if len(readItems) != expectedCount {
			return false
		}

		// Verify the last expectedCount items match
		startIdx := len(items) - expectedCount
		for i := 0; i < expectedCount; i++ {
			if readItems[i] != items[startIdx+i] {
				return false
			}
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyPositionMonotonicity verifies that GetCurrentPosition() never
// decreases after writes.
func TestPropertyPositionMonotonicity(t *testing.T) {
	f := func(batches [][]int, capacityOffset uint8) bool {
		if len(batches) == 0 {
			return true // Vacuously true for empty input
		}

		// Generate capacity as uint8+1 (1-256)
		capacity := int(capacityOffset) + 1
		if capacity <= 0 {
			capacity = 1
		}

		rb := NewRingBuffer[int](capacity)

		var prevPosition int64

		// Write batches and check position is non-decreasing
		for _, batch := range batches {
			for _, item := range batch {
				rb.WriteOne(item)
			}

			currentPosition := rb.GetCurrentPosition()

			// Position should never decrease
			if currentPosition < prevPosition {
				return false
			}

			prevPosition = currentPosition
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyClearResetsLength verifies that Clear() resets length to 0
// but preserves capacity.
func TestPropertyClearResetsLength(t *testing.T) {
	f := func(items []int, capacityOffset uint8) bool {
		// Generate capacity as uint8+1 (1-256)
		capacity := int(capacityOffset) + 1
		if capacity <= 0 {
			capacity = 1
		}

		rb := NewRingBuffer[int](capacity)

		// Write items
		for _, item := range items {
			rb.WriteOne(item)
		}

		// Clear the buffer
		rb.Clear()

		// Verify length is 0 and capacity is preserved
		return rb.Len() == 0 && rb.Cap() == capacity
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyReadAllNonDestructive verifies that ReadAll() does not modify
// the buffer state (non-destructive read).
func TestPropertyReadAllNonDestructive(t *testing.T) {
	f := func(items []int, capacityOffset uint8) bool {
		if len(items) == 0 {
			return true // Vacuously true for empty input
		}

		// Generate capacity as uint8+1 (1-256)
		capacity := int(capacityOffset) + 1
		if capacity <= 0 {
			capacity = 1
		}

		rb := NewRingBuffer[int](capacity)

		// Write items
		for _, item := range items {
			rb.WriteOne(item)
		}

		// Read twice
		first := rb.ReadAll()
		second := rb.ReadAll()

		// Results should be identical
		if len(first) != len(second) {
			return false
		}

		for i := range first {
			if first[i] != second[i] {
				return false
			}
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}
