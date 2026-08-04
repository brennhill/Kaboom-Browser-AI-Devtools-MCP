// Purpose: SLO compliance tests for pagination and cursor latency and throughput budgets.
// Docs: docs/features/feature/pagination/index.md

// cursor_slo_test.go — Performance SLO tests for cursor operations.

package pagination

import (
	"testing"
	"time"
)

// TestSLOParseCursor validates that ParseCursor completes in < 1μs average.
// This SLO ensures cursor parsing doesn't add latency to pagination-heavy operations.
func TestSLOParseCursor(t *testing.T) {
	if testing.Short() {
		t.Skip("wall-clock SLO runs in the isolated performance lane")
	}
	if raceDetectorEnabled {
		t.Skip("SLO test skipped under race detector (significantly slower execution)")
	}

	const iterations = 10000
	const maxAvgDuration = 1 * time.Microsecond

	// Valid cursor string format: timestamp:sequence
	cursorStr := "2024-01-01T00:00:00Z:42"

	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, err := ParseCursor(cursorStr)
		if err != nil {
			t.Fatalf("ParseCursor failed: %v", err)
		}
	}
	elapsed := time.Since(start)

	avgDuration := elapsed / iterations
	if avgDuration > maxAvgDuration {
		t.Errorf("ParseCursor SLO violation: avg %v > %v (total %v for %d iterations)",
			avgDuration, maxAvgDuration, elapsed, iterations)
	} else {
		t.Logf("ParseCursor SLO met: avg %v < %v (total %v for %d iterations)",
			avgDuration, maxAvgDuration, elapsed, iterations)
	}
}

// TestSLOBuildCursor validates that BuildCursor completes in < 500ns average.
// This SLO ensures cursor generation is negligible overhead in response paths.
func TestSLOBuildCursor(t *testing.T) {
	if testing.Short() {
		t.Skip("wall-clock SLO runs in the isolated performance lane")
	}
	if raceDetectorEnabled {
		t.Skip("SLO test skipped under race detector (significantly slower execution)")
	}

	const iterations = 10000
	const maxAvgDuration = 500 * time.Nanosecond

	timestamp := "2024-01-01T00:00:00Z"
	sequence := int64(42)

	start := time.Now()
	for i := 0; i < iterations; i++ {
		_ = BuildCursor(timestamp, sequence)
	}
	elapsed := time.Since(start)

	avgDuration := elapsed / iterations
	if avgDuration > maxAvgDuration {
		t.Errorf("BuildCursor SLO violation: avg %v > %v (total %v for %d iterations)",
			avgDuration, maxAvgDuration, elapsed, iterations)
	} else {
		t.Logf("BuildCursor SLO met: avg %v < %v (total %v for %d iterations)",
			avgDuration, maxAvgDuration, elapsed, iterations)
	}
}
