// Purpose: Provides filtered cursor/read helpers on top of ring buffer core storage.
// Why: Keeps query-time filtering logic separate from write/read storage primitives.
// Docs: docs/features/feature/ring-buffer/index.md

package buffers

import "time"

// currentCursor returns a cursor pointing to the current end of the buffer.
func (rb *RingBuffer[T]) currentCursor() BufferCursor {
	return BufferCursor{Position: rb.totalAdded, Timestamp: time.Now()}
}

// resolveStartPosition clamps a cursor position to the oldest available entry.
// Returns the clamped start position and the number of entries available, or -1 if none.
func (rb *RingBuffer[T]) resolveStartPosition(cursorPos int64) (int64, int64) {
	oldestPosition := rb.totalAdded - int64(len(rb.entries))
	if oldestPosition < 0 {
		oldestPosition = 0
	}
	startPosition := cursorPos
	if startPosition < oldestPosition {
		startPosition = oldestPosition
	}
	return startPosition, rb.totalAdded - startPosition
}

// ReadFromWithFilter reads entries from cursor and applies a filter function.
// Only entries where filter returns true are included in the result.
func (rb *RingBuffer[T]) ReadFromWithFilter(cursor BufferCursor, filter func(T) bool, limit int) ([]T, BufferCursor) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if len(rb.entries) == 0 {
		return nil, rb.currentCursor()
	}

	startPosition, entriesAvailable := rb.resolveStartPosition(cursor.Position)
	if entriesAvailable <= 0 {
		return nil, rb.currentCursor()
	}

	startIndex := rb.positionToIndex(startPosition)
	capHint := int(entriesAvailable)
	if limit > 0 && limit < capHint {
		capHint = limit
	}
	result := make([]T, 0, capHint)
	for i := int64(0); i < entriesAvailable; i++ {
		idx := int((int64(startIndex) + i) % int64(len(rb.entries)))
		if filter(rb.entries[idx]) {
			result = append(result, rb.entries[idx])
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, rb.currentCursor()
}

// ReadAllWithFilter returns all entries that pass the filter, oldest first.
// Filters in a single RLock pass to avoid copying unneeded entries.
func (rb *RingBuffer[T]) ReadAllWithFilter(filter func(T) bool, limit int) []T {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if len(rb.entries) == 0 {
		return nil
	}

	capHint := len(rb.entries)
	if limit > 0 && limit < capHint {
		capHint = limit
	}
	result := make([]T, 0, capHint)

	if len(rb.entries) < rb.capacity {
		for i := 0; i < len(rb.entries); i++ {
			if filter(rb.entries[i]) {
				result = append(result, rb.entries[i])
				if limit > 0 && len(result) >= limit {
					break
				}
			}
		}
	} else {
		for i := 0; i < len(rb.entries); i++ {
			idx := (rb.head + i) % len(rb.entries)
			if filter(rb.entries[idx]) {
				result = append(result, rb.entries[idx])
				if limit > 0 && len(result) >= limit {
					break
				}
			}
		}
	}
	return result
}
