// Purpose: Stress tests for ring buffer under concurrent load.
// Docs: docs/features/feature/ring-buffer/index.md

// ring_buffer_stress_test.go — Concurrent stress tests for ring buffer.
package buffers

import (
	"sync"
	"testing"
	"time"
)

// TestStressRingBufferConcurrent verifies thread-safety under heavy concurrent load.
// Launches 50 writers, 20 readers, and 5 clearers, all operating simultaneously
// on a shared ring buffer. Designed to be run with -race to detect data races.
func TestStressRingBufferConcurrent(t *testing.T) {
	t.Run("concurrent_stress", func(t *testing.T) {
		const (
			capacity        = 100
			numWriters      = 50
			numReaders      = 20
			numClearers     = 5
			writesPerWriter = 100
			readsPerReader  = 50
			clearsPerClear  = 10
		)

		rb := NewRingBuffer[int](capacity)
		var wg sync.WaitGroup
		start := make(chan struct{})

		// Launch writer goroutines
		for writerID := 0; writerID < numWriters; writerID++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				<-start
				for i := 0; i < writesPerWriter; i++ {
					value := id*1000 + i
					rb.WriteOne(value)
				}
			}(writerID)
		}

		// Launch reader goroutines
		for readerID := 0; readerID < numReaders; readerID++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				<-start
				for i := 0; i < readsPerReader; i++ {
					_ = rb.ReadAll()
				}
			}(readerID)
		}

		// Launch clearer goroutines
		for clearID := 0; clearID < numClearers; clearID++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				<-start
				for i := 0; i < clearsPerClear; i++ {
					rb.Clear()
				}
			}(clearID)
		}

		close(start)
		// Wait for all goroutines to complete
		wg.Wait()

		// Verify final state invariants
		finalLen := rb.Len()
		finalCap := rb.Cap()

		if finalLen > finalCap {
			t.Errorf("Ring buffer length (%d) exceeds capacity (%d)", finalLen, finalCap)
		}

		if finalCap != capacity {
			t.Errorf("Ring buffer capacity changed: expected %d, got %d", capacity, finalCap)
		}

		// Verify that we can still read without panicking
		_ = rb.ReadAll()
		_ = rb.ReadLast(10)
		_ = rb.GetCurrentPosition()

		t.Logf("Stress test completed: %d writers, %d readers, %d clearers", numWriters, numReaders, numClearers)
		t.Logf("Final state: len=%d, cap=%d, position=%d", finalLen, finalCap, rb.GetCurrentPosition())
	})
}

// TestStressRingBufferCursorConcurrent verifies cursor-based reads under concurrent load.
// Multiple readers use cursors to track their read positions while writers continuously add data.
func TestStressRingBufferCursorConcurrent(t *testing.T) {
	t.Run("cursor_concurrent_stress", func(t *testing.T) {
		const (
			capacity        = 50
			numWriters      = 10
			numReaders      = 10
			writesPerWriter = 100
			readsPerReader  = 50
		)

		rb := NewRingBuffer[int](capacity)
		var wg sync.WaitGroup
		start := make(chan struct{})

		// Launch writer goroutines
		for writerID := 0; writerID < numWriters; writerID++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				<-start
				for i := 0; i < writesPerWriter; i++ {
					value := id*1000 + i
					rb.WriteOne(value)
				}
			}(writerID)
		}

		// Launch cursor-based reader goroutines
		for readerID := 0; readerID < numReaders; readerID++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				<-start
				cursor := BufferCursor{Position: 0, Timestamp: time.Now()}
				for i := 0; i < readsPerReader; i++ {
					_, newCursor := rb.ReadFrom(cursor)
					cursor = newCursor
				}
			}(readerID)
		}

		close(start)
		wg.Wait()

		// Verify final state
		if rb.Len() > rb.Cap() {
			t.Errorf("Ring buffer length (%d) exceeds capacity (%d)", rb.Len(), rb.Cap())
		}

		t.Logf("Cursor stress test completed: %d writers, %d readers", numWriters, numReaders)
	})
}

// TestStressRingBufferBatchWrites verifies batch write operations under concurrent load.
func TestStressRingBufferBatchWrites(t *testing.T) {
	t.Run("batch_write_stress", func(t *testing.T) {
		const (
			capacity        = 100
			numWriters      = 20
			batchesPerWrite = 50
			batchSize       = 10
		)

		rb := NewRingBuffer[int](capacity)
		var wg sync.WaitGroup

		// Launch batch writer goroutines
		for writerID := 0; writerID < numWriters; writerID++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for i := 0; i < batchesPerWrite; i++ {
					batch := make([]int, batchSize)
					for j := 0; j < batchSize; j++ {
						batch[j] = id*10000 + i*100 + j
					}
					rb.Write(batch)
				}
			}(writerID)
		}

		wg.Wait()

		// Verify final state
		if rb.Len() > rb.Cap() {
			t.Errorf("Ring buffer length (%d) exceeds capacity (%d)", rb.Len(), rb.Cap())
		}

		t.Logf("Batch write stress test completed: %d writers, %d batches each", numWriters, batchesPerWrite)
	})
}
