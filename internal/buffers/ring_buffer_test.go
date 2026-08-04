// Purpose: Tests for ring buffer append, eviction, and iteration correctness.
// Docs: docs/features/feature/ring-buffer/index.md

package buffers

import (
	"sync"
	"testing"
	"time"
)

// ============================================
// Basic Write/Read
// ============================================

func TestRingBuffer_WriteAndReadAll(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](5)

	rb.Write([]int{1, 2, 3})
	all := rb.ReadAll()

	if len(all) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(all))
	}
	for i, v := range all {
		if v != i+1 {
			t.Errorf("Entry %d: expected %d, got %d", i, i+1, v)
		}
	}
}

func TestRingBuffer_WriteOne(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[string](10)
	rb.WriteOne("hello")
	rb.WriteOne("world")

	all := rb.ReadAll()
	if len(all) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(all))
	}
	if all[0] != "hello" || all[1] != "world" {
		t.Errorf("Got %v", all)
	}
}

func TestRingBuffer_EmptyRead(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](5)

	all := rb.ReadAll()
	if all != nil {
		t.Errorf("Empty buffer ReadAll should return nil, got %v", all)
	}
}

func TestRingBuffer_WriteEmpty(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](5)
	n := rb.Write([]int{})
	if n != 0 {
		t.Errorf("Writing empty slice should return 0, got %d", n)
	}
}

// ============================================
// Wraparound
// ============================================

func TestRingBuffer_Wraparound(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](3)

	rb.Write([]int{1, 2, 3}) // fills buffer
	rb.Write([]int{4, 5})    // overwrites 1, 2

	all := rb.ReadAll()
	if len(all) != 3 {
		t.Fatalf("Expected 3 entries after wraparound, got %d", len(all))
	}
	// Should be [3, 4, 5] (oldest first)
	expected := []int{3, 4, 5}
	for i, v := range all {
		if v != expected[i] {
			t.Errorf("Entry %d: expected %d, got %d", i, expected[i], v)
		}
	}
}

func TestRingBuffer_FullOverwrite(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](3)

	// Write more than capacity
	rb.Write([]int{1, 2, 3, 4, 5, 6, 7})

	all := rb.ReadAll()
	if len(all) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(all))
	}
	// Last 3 should be [5, 6, 7]
	expected := []int{5, 6, 7}
	for i, v := range all {
		if v != expected[i] {
			t.Errorf("Entry %d: expected %d, got %d", i, expected[i], v)
		}
	}
}

// ============================================
// ReadLast
// ============================================

func TestRingBuffer_ReadLast(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](10)
	rb.Write([]int{1, 2, 3, 4, 5})

	last2 := rb.ReadLast(2)
	if len(last2) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(last2))
	}
	if last2[0] != 4 || last2[1] != 5 {
		t.Errorf("ReadLast(2) = %v, expected [4, 5]", last2)
	}
}

func TestRingBuffer_ReadLastExceedsSize(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](10)
	rb.Write([]int{1, 2, 3})

	last := rb.ReadLast(10)
	if len(last) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(last))
	}
}

func TestRingBuffer_ReadLastAfterWraparound(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](3)
	rb.Write([]int{1, 2, 3, 4, 5}) // buffer has [3, 4, 5]

	last2 := rb.ReadLast(2)
	if len(last2) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(last2))
	}
	if last2[0] != 4 || last2[1] != 5 {
		t.Errorf("ReadLast(2) after wraparound = %v, expected [4, 5]", last2)
	}
}

func TestRingBuffer_ReadLastZero(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](5)
	rb.WriteOne(1)
	result := rb.ReadLast(0)
	if result != nil {
		t.Errorf("ReadLast(0) should return nil, got %v", result)
	}
}

// ============================================
// Cursor-Based Reads
// ============================================

func TestRingBuffer_ReadFromZero(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](10)
	rb.Write([]int{10, 20, 30})

	entries, cursor := rb.ReadFrom(BufferCursor{Position: 0})
	if len(entries) != 3 {
		t.Fatalf("Expected 3 entries from position 0, got %d", len(entries))
	}
	if cursor.Position != 3 {
		t.Errorf("Expected cursor position 3, got %d", cursor.Position)
	}
}

func TestRingBuffer_ReadFromMiddle(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](10)
	rb.Write([]int{10, 20, 30, 40, 50})

	entries, cursor := rb.ReadFrom(BufferCursor{Position: 2})
	if len(entries) != 3 {
		t.Fatalf("Expected 3 entries from position 2, got %d", len(entries))
	}
	if entries[0] != 30 || entries[1] != 40 || entries[2] != 50 {
		t.Errorf("Got %v, expected [30, 40, 50]", entries)
	}
	if cursor.Position != 5 {
		t.Errorf("Expected cursor position 5, got %d", cursor.Position)
	}
}

func TestRingBuffer_ReadFromCurrent(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](10)
	rb.Write([]int{10, 20, 30})

	// Read from current position (nothing new)
	entries, cursor := rb.ReadFrom(BufferCursor{Position: 3})
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries from current position, got %d", len(entries))
	}
	if cursor.Position != 3 {
		t.Errorf("Expected cursor position 3, got %d", cursor.Position)
	}
}

func TestRingBuffer_ReadFromEvicted(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](3)
	rb.Write([]int{1, 2, 3, 4, 5}) // buffer has [3, 4, 5], positions 0-1 evicted

	// Read from evicted position - should start from oldest available
	entries, cursor := rb.ReadFrom(BufferCursor{Position: 0})
	if len(entries) != 3 {
		t.Fatalf("Expected 3 entries (all available), got %d", len(entries))
	}
	if entries[0] != 3 {
		t.Errorf("First entry should be 3 (oldest available), got %d", entries[0])
	}
	if cursor.Position != 5 {
		t.Errorf("Expected cursor position 5, got %d", cursor.Position)
	}
}

func TestRingBuffer_SequentialCursorReads(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](100)

	// Write batch 1
	rb.Write([]int{1, 2, 3})

	// First read
	entries1, cursor := rb.ReadFrom(BufferCursor{})
	if len(entries1) != 3 {
		t.Fatalf("First read: expected 3, got %d", len(entries1))
	}

	// Write batch 2
	rb.Write([]int{4, 5})

	// Second read from cursor
	entries2, cursor := rb.ReadFrom(cursor)
	if len(entries2) != 2 {
		t.Fatalf("Second read: expected 2, got %d", len(entries2))
	}
	if entries2[0] != 4 || entries2[1] != 5 {
		t.Errorf("Second read: got %v, expected [4, 5]", entries2)
	}

	// Third read (nothing new)
	entries3, _ := rb.ReadFrom(cursor)
	if len(entries3) != 0 {
		t.Errorf("Third read: expected 0, got %d", len(entries3))
	}
}

// ============================================
// FindPositionAtTime
// ============================================

func TestRingBuffer_FindPositionAtTime(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[string](10)
	writeTime := time.Unix(100, 0)
	rb.now = func() time.Time { return writeTime }

	before := writeTime.Add(-time.Nanosecond)
	rb.Write([]string{"a", "b", "c"})
	after := writeTime.Add(time.Nanosecond)

	// Position at time before any writes should return 0
	pos := rb.FindPositionAtTime(before)
	if pos != 0 {
		t.Errorf("Position before writes: expected 0, got %d", pos)
	}

	// Position at time after all writes should return -1 (all before)
	pos = rb.FindPositionAtTime(after)
	if pos != -1 {
		t.Errorf("Position after writes: expected -1, got %d", pos)
	}
}

func TestRingBuffer_FindPositionAtTime_Empty(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](5)
	pos := rb.FindPositionAtTime(time.Now())
	if pos != -1 {
		t.Errorf("Empty buffer: expected -1, got %d", pos)
	}
}

// ============================================
// Position Tracking
// ============================================

func TestRingBuffer_GetCurrentPosition(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](5)

	if rb.GetCurrentPosition() != 0 {
		t.Error("New buffer should have position 0")
	}

	rb.Write([]int{1, 2, 3})
	if rb.GetCurrentPosition() != 3 {
		t.Errorf("Expected position 3, got %d", rb.GetCurrentPosition())
	}

	rb.Write([]int{4, 5, 6, 7}) // wraps
	if rb.GetCurrentPosition() != 7 {
		t.Errorf("Expected position 7, got %d", rb.GetCurrentPosition())
	}
}

func TestRingBuffer_LenAndCap(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](5)

	if rb.Len() != 0 {
		t.Error("New buffer length should be 0")
	}
	if rb.Cap() != 5 {
		t.Error("Capacity should be 5")
	}

	rb.Write([]int{1, 2, 3})
	if rb.Len() != 3 {
		t.Errorf("Length should be 3, got %d", rb.Len())
	}

	rb.Write([]int{4, 5, 6, 7}) // wraps
	if rb.Len() != 5 {
		t.Errorf("Length should be 5 (capped), got %d", rb.Len())
	}
}

// ============================================
// Clear
// ============================================

func TestRingBuffer_Clear(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](5)
	rb.Write([]int{1, 2, 3})
	rb.Clear()

	if rb.Len() != 0 {
		t.Errorf("Length after clear should be 0, got %d", rb.Len())
	}

	all := rb.ReadAll()
	if all != nil {
		t.Errorf("ReadAll after clear should return nil, got %v", all)
	}

	// Position should not reset (for cursor continuity)
	if rb.GetCurrentPosition() != 3 {
		t.Errorf("Position should be preserved after clear, got %d", rb.GetCurrentPosition())
	}
}

// TestRingBuffer_ReadFromAndFindPositionAfterClear is the regression test for
// positionToIndex mishandling the not-full case after Clear(): Clear preserves
// totalAdded, so monotonic positions no longer equal slice indices. The old
// int(position) mapping read the wrong entries in ReadFrom and indexed out of
// range (panic) in FindPositionAtTime.
func TestRingBuffer_ReadFromAndFindPositionAfterClear(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](5)
	rb.Write([]int{1, 2, 3}) // positions 0-2
	rb.Clear()               // totalAdded stays at 3, entries empty
	rb.Write([]int{4, 5})    // positions 3-4 at slice indices 0-1

	// ReadFrom with a pre-clear cursor must return exactly the post-clear entries.
	entries, cursor := rb.ReadFrom(BufferCursor{Position: 3})
	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries after clear, got %d (%v)", len(entries), entries)
	}
	if entries[0] != 4 || entries[1] != 5 {
		t.Errorf("Expected [4 5], got %v", entries)
	}
	if cursor.Position != 5 {
		t.Errorf("Expected cursor at 5, got %d", cursor.Position)
	}

	// FindPositionAtTime must not panic and must resolve to the oldest
	// surviving entry's monotonic position (3), not its slice index (0).
	pos := rb.FindPositionAtTime(time.Time{}) // zero time matches every entry
	if pos != 3 {
		t.Errorf("FindPositionAtTime = %d, want 3 (oldest post-clear position)", pos)
	}

	// Reading from that position must include all surviving entries.
	entries, _ = rb.ReadFrom(BufferCursor{Position: pos})
	if len(entries) != 2 || entries[0] != 4 {
		t.Errorf("ReadFrom(FindPositionAtTime) = %v, want [4 5]", entries)
	}
}

// ============================================
// Filtered Reads
// ============================================

func TestRingBuffer_ReadFromWithFilter(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](10)
	rb.Write([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

	// Only even numbers
	even := func(n int) bool { return n%2 == 0 }
	entries, cursor := rb.ReadFromWithFilter(BufferCursor{}, even, 0)

	if len(entries) != 5 {
		t.Fatalf("Expected 5 even entries, got %d", len(entries))
	}
	if cursor.Position != 10 {
		t.Errorf("Expected cursor at 10, got %d", cursor.Position)
	}
}

func TestRingBuffer_ReadFromWithFilter_Limit(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](10)
	rb.Write([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

	all := func(n int) bool { return true }
	entries, _ := rb.ReadFromWithFilter(BufferCursor{}, all, 3)

	if len(entries) != 3 {
		t.Fatalf("Expected 3 entries with limit, got %d", len(entries))
	}
}

func TestRingBuffer_ReadAllWithFilter(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](10)
	rb.Write([]int{1, 2, 3, 4, 5})

	gt3 := func(n int) bool { return n > 3 }
	entries := rb.ReadAllWithFilter(gt3, 0)

	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries > 3, got %d", len(entries))
	}
}

// ============================================
// Concurrency
// ============================================

func TestRingBuffer_ConcurrentWriteAndRead(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](100)
	var wg sync.WaitGroup
	start := make(chan struct{})

	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			for j := 0; j < 100; j++ {
				rb.WriteOne(i*100 + j)
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			cursor := BufferCursor{}
			for j := 0; j < 50; j++ {
				_, cursor = rb.ReadFrom(cursor)
			}
		}()
	}

	close(start)
	wg.Wait()

	// Buffer should have entries and not have panicked
	if rb.Len() == 0 {
		t.Error("Buffer should have entries after concurrent writes")
	}
	if rb.GetCurrentPosition() != 1000 {
		t.Errorf("Expected 1000 total writes, got %d", rb.GetCurrentPosition())
	}
}

func TestRingBuffer_ConcurrentCursorReads(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](50)
	rb.Write([]int{1, 2, 3, 4, 5})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			entries, _ := rb.ReadFrom(BufferCursor{Position: 2})
			if len(entries) != 3 {
				t.Errorf("Expected 3 entries, got %d", len(entries))
			}
		}()
	}

	wg.Wait()
}

// ============================================
// Edge Cases
// ============================================

func TestRingBuffer_CapacityOne(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[string](1)

	rb.WriteOne("a")
	if rb.ReadAll()[0] != "a" {
		t.Error("Expected 'a'")
	}

	rb.WriteOne("b")
	if rb.ReadAll()[0] != "b" {
		t.Error("Expected 'b' after overwrite")
	}
	if rb.Len() != 1 {
		t.Errorf("Expected len 1, got %d", rb.Len())
	}
}

func TestRingBuffer_MultipleWraparounds(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer[int](3)

	// Write 15 entries (5 full rotations)
	for i := 1; i <= 15; i++ {
		rb.WriteOne(i)
	}

	all := rb.ReadAll()
	if len(all) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(all))
	}
	// Should be [13, 14, 15]
	expected := []int{13, 14, 15}
	for i, v := range all {
		if v != expected[i] {
			t.Errorf("Entry %d: expected %d, got %d", i, expected[i], v)
		}
	}

	if rb.GetCurrentPosition() != 15 {
		t.Errorf("Expected position 15, got %d", rb.GetCurrentPosition())
	}
}
