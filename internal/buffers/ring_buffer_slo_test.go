// Purpose: SLO compliance tests for ring buffer latency and throughput budgets.
// Docs: docs/features/feature/ring-buffer/index.md

// ring_buffer_slo_test.go — Performance SLO tests for ring buffer operations.

package buffers

import (
	"testing"
	"time"
)

// TestSLOWriteOne validates that WriteOne completes in < 500ns average.
// This SLO supports the WebSocket < 0.1ms requirement from CLAUDE.md.
func TestSLOWriteOne(t *testing.T) {
	if raceDetectorEnabled {
		t.Skip("SLO test skipped under race detector (significantly slower execution)")
	}

	const iterations = 10000
	const maxAvgDuration = 500 * time.Nanosecond

	rb := NewRingBuffer[int](1000)

	start := time.Now()
	for i := 0; i < iterations; i++ {
		rb.WriteOne(i)
	}
	elapsed := time.Since(start)

	avgDuration := elapsed / iterations
	if avgDuration > maxAvgDuration {
		t.Errorf("WriteOne SLO violation: avg %v > %v (total %v for %d iterations)",
			avgDuration, maxAvgDuration, elapsed, iterations)
	} else {
		t.Logf("WriteOne SLO met: avg %v < %v (total %v for %d iterations)",
			avgDuration, maxAvgDuration, elapsed, iterations)
	}
}

// TestSLOReadAll validates that ReadAll on a 1000-entry buffer completes in < 100μs average.
// This ensures low-latency reads for observe tool responses.
func TestSLOReadAll(t *testing.T) {
	if raceDetectorEnabled {
		t.Skip("SLO test skipped under race detector (significantly slower execution)")
	}

	const iterations = 1000
	const bufferSize = 1000
	const maxAvgDuration = 100 * time.Microsecond

	rb := NewRingBuffer[int](bufferSize)

	// Fill buffer with 1000 entries
	for i := 0; i < bufferSize; i++ {
		rb.WriteOne(i)
	}

	start := time.Now()
	for i := 0; i < iterations; i++ {
		items := rb.ReadAll()
		if len(items) != bufferSize {
			t.Fatalf("ReadAll returned %d items, expected %d", len(items), bufferSize)
		}
	}
	elapsed := time.Since(start)

	avgDuration := elapsed / iterations
	if avgDuration > maxAvgDuration {
		t.Errorf("ReadAll SLO violation: avg %v > %v (total %v for %d iterations)",
			avgDuration, maxAvgDuration, elapsed, iterations)
	} else {
		t.Logf("ReadAll SLO met: avg %v < %v (total %v for %d iterations)",
			avgDuration, maxAvgDuration, elapsed, iterations)
	}
}

func BenchmarkRingBufferWriteOne(b *testing.B) {
	rb := NewRingBuffer[int](1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.WriteOne(i)
	}
}

func BenchmarkRingBufferWrite(b *testing.B) {
	rb := NewRingBuffer[int](1000)
	batch := make([]int, 10)
	for i := range batch {
		batch[i] = i
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.Write(batch)
	}
}

func BenchmarkRingBufferReadFrom(b *testing.B) {
	rb := NewRingBuffer[int](1000)
	batch := make([]int, 100)
	for i := 0; i < 10; i++ {
		rb.Write(batch)
	}
	cursor := BufferCursor{Position: 500, Timestamp: time.Now()}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.ReadFrom(cursor)
	}
}

func BenchmarkRingBufferReadAll(b *testing.B) {
	rb := NewRingBuffer[int](1000)
	rb.Write(make([]int, 1000))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.ReadAll()
	}
}

func BenchmarkRingBufferWriteWithEviction(b *testing.B) {
	rb := NewRingBuffer[int](1000)
	rb.Write(make([]int, 1000))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.WriteOne(i)
	}
}

func BenchmarkRingBufferConcurrent(b *testing.B) {
	rb := NewRingBuffer[int](10000)
	cursor := BufferCursor{Timestamp: time.Now()}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for i := 0; pb.Next(); i++ {
			if i%2 == 0 {
				rb.WriteOne(i)
			} else {
				rb.ReadFrom(cursor)
			}
		}
	})
}
