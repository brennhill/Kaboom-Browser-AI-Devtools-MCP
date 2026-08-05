// Purpose: Benchmark pagination and cursor throughput and latency.
// Docs: docs/features/feature/pagination/index.md

package pagination

import (
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// BenchmarkParseCursor measures cursor parsing performance
func BenchmarkParseCursor(b *testing.B) {
	cursor := "2026-01-30T10:15:23.456789Z:1234"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseCursor(cursor)
	}
}

// BenchmarkBuildCursor measures cursor building performance
func BenchmarkBuildCursor(b *testing.B) {
	ts := "2026-01-30T10:15:23.456789Z"
	seq := int64(1234)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildCursor(ts, seq)
	}
}

// BenchmarkEnrichLogEntries measures log entry enrichment performance
func BenchmarkEnrichLogEntries(b *testing.B) {
	// Create 1000 log entries
	entries := make([]types.LogEntry, 1000)

	for i := 0; i < 1000; i++ {
		entries[i] = types.LogEntry{
			"message":   "test log entry",
			"level":     "info",
			"timestamp": "2026-01-30T10:15:23Z",
		}
	}

	totalAdded := int64(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EnrichLogEntries(entries, totalAdded)
	}
}

// BenchmarkApplyLogCursorPagination measures pagination performance on enriched datasets
func BenchmarkApplyLogCursorPagination(b *testing.B) {
	// Create and enrich 1000 log entries
	entries := make([]types.LogEntry, 1000)

	for i := 0; i < 1000; i++ {
		entries[i] = types.LogEntry{
			"message":   "test log entry",
			"level":     "info",
			"timestamp": "2026-01-30T10:15:23Z",
		}
	}

	totalAdded := int64(1000)
	enriched := EnrichLogEntries(entries, totalAdded)
	cursor := "2026-01-30T10:15:23Z:500"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ApplyLogCursorPagination(enriched, cursor, "", "", 100, false)
	}
}

// BenchmarkEnrichWebSocketEntries measures WebSocket enrichment performance
func BenchmarkEnrichWebSocketEntries(b *testing.B) {
	// Create 1000 WebSocket events
	events := make([]types.WebSocketEvent, 1000)

	for i := 0; i < 1000; i++ {
		events[i] = types.WebSocketEvent{
			Timestamp: "2026-01-30T10:15:23.456789Z",
			ID:        "ws_bench",
			Event:     "message",
			Data:      "test data",
		}
	}

	totalAdded := int64(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EnrichWebSocketEntries(events, totalAdded)
	}
}

// BenchmarkApplyWebSocketCursorPagination measures WebSocket pagination performance
func BenchmarkApplyWebSocketCursorPagination(b *testing.B) {
	// Create and enrich 1000 WebSocket events
	events := make([]types.WebSocketEvent, 1000)

	for i := 0; i < 1000; i++ {
		events[i] = types.WebSocketEvent{
			Timestamp: "2026-01-30T10:15:23.456789Z",
			ID:        "ws_bench",
			Event:     "message",
			Data:      "test data",
		}
	}

	totalAdded := int64(1000)
	enriched := EnrichWebSocketEntries(events, totalAdded)
	cursor := "2026-01-30T10:15:23.456789Z:500"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ApplyWebSocketCursorPagination(enriched, cursor, "", "", 50, false)
	}
}

// BenchmarkEnrichActionEntries measures action enrichment performance
func BenchmarkEnrichActionEntries(b *testing.B) {
	// Create 1000 actions
	actions := make([]types.EnhancedAction, 1000)

	for i := 0; i < 1000; i++ {
		actions[i] = types.EnhancedAction{
			Timestamp: 1706615723456789000,
			Type:      "click",
			Selectors: map[string]any{"css": "button"},
			URL:       "https://example.com",
		}
	}

	totalAdded := int64(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EnrichActionEntries(actions, totalAdded)
	}
}

// BenchmarkApplyActionCursorPagination measures action pagination performance
func BenchmarkApplyActionCursorPagination(b *testing.B) {
	// Create and enrich 1000 actions
	actions := make([]types.EnhancedAction, 1000)

	for i := 0; i < 1000; i++ {
		actions[i] = types.EnhancedAction{
			Timestamp: 1706615723456789000,
			Type:      "click",
			Selectors: map[string]any{"css": "button"},
			URL:       "https://example.com",
		}
	}

	totalAdded := int64(1000)
	enriched := EnrichActionEntries(actions, totalAdded)
	cursor := "2026-01-30T10:15:23.456789Z:500"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ApplyActionCursorPagination(enriched, cursor, "", "", 50, false)
	}
}

// FuzzParseCursor fuzzes the ParseCursor function to verify parsing robustness.
// Tests valid cursor formats, malformed inputs, and edge cases (overflow, unicode, large strings).
//
// Invariants:
// 1. Round-trip: if ParseCursor(s) succeeds, BuildCursor(c.Timestamp, c.Sequence) must parse identically
// 2. Error or valid: never returns garbage (nil error → valid Cursor)
// 3. Empty string always succeeds with zero cursor
func FuzzParseCursor(f *testing.F) {
	// Seed corpus: valid cursors
	f.Add("2024-01-01T00:00:00Z:42")
	f.Add(":100")                               // sequence-only
	f.Add("")                                   // empty (first page)
	f.Add("2024-01-01T00:00:00.123456789Z:999") // RFC3339Nano
	f.Add("2024-12-31T23:59:59Z:0")
	f.Add(":0")
	f.Add("2024-01-01T00:00:00Z:-1") // negative sequence

	// Seed corpus: invalid cursors
	f.Add("no-colon")
	f.Add("2024-01-01T00:00:00Z:not-a-number")
	f.Add(strings.Repeat("a", 10*1024))               // 10KB string
	f.Add("日本語:42")                                   // unicode
	f.Add("2024-01-01T00:00:00Z:9999999999999999999") // int64 overflow
	f.Add("2024-01-01T00:00:00Z")                     // missing sequence
	f.Add("::")
	f.Add("::123")
	f.Add("invalid-timestamp:42")
	f.Add("2024-13-01T00:00:00Z:42") // invalid month
	f.Add(":abc")                    // non-numeric sequence

	f.Fuzz(func(t *testing.T, cursorStr string) {
		// Invariant 3: Empty string always succeeds with zero cursor
		if cursorStr == "" {
			cursor, err := ParseCursor(cursorStr)
			if err != nil {
				t.Fatalf("ParseCursor(\"\") failed: %v (must always succeed)", err)
			}
			if cursor.Timestamp != "" || cursor.Sequence != 0 {
				t.Fatalf("ParseCursor(\"\") returned non-zero cursor: %+v", cursor)
			}
			return
		}

		// Parse the cursor
		cursor, err := ParseCursor(cursorStr)

		if err != nil {
			// Invalid cursor: error is expected for certain inputs
			// Verify error message is non-empty
			if err.Error() == "" {
				t.Fatalf("ParseCursor returned error with empty message for input %q", cursorStr)
			}
			return
		}

		// Invariant 2: If no error, cursor must be valid
		// - Sequence can be any valid int64 (including negative)
		// - Timestamp must be empty or valid RFC3339/RFC3339Nano
		if cursor.Timestamp != "" {
			// Validate timestamp format
			_, err1 := time.Parse(time.RFC3339, cursor.Timestamp)
			_, err2 := time.Parse(time.RFC3339Nano, cursor.Timestamp)
			if err1 != nil && err2 != nil {
				t.Fatalf("ParseCursor succeeded but returned invalid timestamp %q (neither RFC3339 nor RFC3339Nano)", cursor.Timestamp)
			}
		}

		// Invariant 1: Round-trip consistency
		// If ParseCursor(s) succeeded, BuildCursor(timestamp, sequence) → ParseCursor() must produce identical result
		rebuilt := BuildCursor(cursor.Timestamp, cursor.Sequence)
		cursor2, err2 := ParseCursor(rebuilt)
		if err2 != nil {
			t.Fatalf("Round-trip failed: ParseCursor(%q) → %+v → BuildCursor → %q → ParseCursor failed: %v",
				cursorStr, cursor, rebuilt, err2)
		}

		if cursor2.Timestamp != cursor.Timestamp || cursor2.Sequence != cursor.Sequence {
			t.Fatalf("Round-trip mismatch: ParseCursor(%q) → %+v, but BuildCursor → ParseCursor → %+v",
				cursorStr, cursor, cursor2)
		}

		// Additional validation: rebuilt cursor should have expected format
		expectedFormat := true
		if cursor.Timestamp == "" {
			// Sequence-only format: ":N"
			if !strings.HasPrefix(rebuilt, ":") || strings.Count(rebuilt, ":") != 1 {
				expectedFormat = false
			}
		} else {
			// Full format: "timestamp:N"
			// Should have at least 2 colons (timestamp has colons + separator)
			if strings.Count(rebuilt, ":") < 2 {
				expectedFormat = false
			}
		}
		if !expectedFormat {
			t.Fatalf("BuildCursor produced unexpected format %q for cursor %+v", rebuilt, cursor)
		}
	})
}

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
