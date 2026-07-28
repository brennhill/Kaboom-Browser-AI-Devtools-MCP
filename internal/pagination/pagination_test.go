// Purpose: Tests for pagination slicing, limits, and offset correctness.
// Docs: docs/features/feature/pagination/index.md

// pagination_test.go — Unit tests for cursor pagination helpers
package pagination

import (
	"fmt"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func buildSequentialLogEntries(baseTime time.Time, startIndex, count int) []LogEntryWithSequence {
	entries := make([]LogEntryWithSequence, count)
	for i := 0; i < count; i++ {
		idx := startIndex + i
		ts := baseTime.Add(time.Duration(idx) * time.Second).Format(time.RFC3339)
		entries[i] = LogEntryWithSequence{
			Entry:     types.LogEntry{"ts": ts, "message": fmt.Sprintf("Log %d", idx)},
			Sequence:  int64(idx + 1),
			Timestamp: ts,
		}
	}
	return entries
}

func TestEnrichLogEntries(t *testing.T) {
	baseTime := time.Date(2026, 1, 30, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		entries       []types.LogEntry
		logTotalAdded int64
		wantFirstSeq  int64
		wantLastSeq   int64
	}{
		{
			name:          "empty buffer",
			entries:       []types.LogEntry{},
			logTotalAdded: 0,
			wantFirstSeq:  0,
			wantLastSeq:   0,
		},
		{
			name: "single entry",
			entries: []types.LogEntry{
				{"ts": baseTime.Format(time.RFC3339), "message": "Log 1"},
			},
			logTotalAdded: 1,
			wantFirstSeq:  1,
			wantLastSeq:   1,
		},
		{
			name: "multiple entries",
			entries: []types.LogEntry{
				{"ts": baseTime.Format(time.RFC3339), "message": "Log 1"},
				{"ts": baseTime.Add(1 * time.Second).Format(time.RFC3339), "message": "Log 2"},
				{"ts": baseTime.Add(2 * time.Second).Format(time.RFC3339), "message": "Log 3"},
			},
			logTotalAdded: 3,
			wantFirstSeq:  1,
			wantLastSeq:   3,
		},
		{
			name: "buffer with evictions (logTotalAdded > buffer length)",
			entries: []types.LogEntry{
				{"ts": baseTime.Format(time.RFC3339), "message": "Log 101"},
				{"ts": baseTime.Add(1 * time.Second).Format(time.RFC3339), "message": "Log 102"},
			},
			logTotalAdded: 102,
			wantFirstSeq:  101, // First 100 logs were evicted
			wantLastSeq:   102,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enriched := EnrichLogEntries(tt.entries, tt.logTotalAdded)

			if len(enriched) != len(tt.entries) {
				t.Errorf("EnrichLogEntries() returned %d entries, want %d", len(enriched), len(tt.entries))
				return
			}

			if len(enriched) == 0 {
				return // Empty case - no sequence to check
			}

			firstSeq := enriched[0].Sequence
			lastSeq := enriched[len(enriched)-1].Sequence

			if firstSeq != tt.wantFirstSeq {
				t.Errorf("First sequence = %d, want %d", firstSeq, tt.wantFirstSeq)
			}
			if lastSeq != tt.wantLastSeq {
				t.Errorf("Last sequence = %d, want %d", lastSeq, tt.wantLastSeq)
			}

			// Verify sequences are monotonically increasing
			for i := 1; i < len(enriched); i++ {
				if enriched[i].Sequence != enriched[i-1].Sequence+1 {
					t.Errorf("Non-monotonic sequence at index %d: %d -> %d", i, enriched[i-1].Sequence, enriched[i].Sequence)
				}
			}

			// Verify timestamps were extracted correctly
			for i, e := range enriched {
				expectedTs := entryStr(tt.entries[i], "ts")
				if e.Timestamp != expectedTs {
					t.Errorf("Entry %d timestamp = %v, want %v", i, e.Timestamp, expectedTs)
				}
			}
		})
	}
}

func TestApplyLogCursorPagination_NoСursor(t *testing.T) {
	baseTime := time.Date(2026, 1, 30, 10, 0, 0, 0, time.UTC)
	entries := buildSequentialLogEntries(baseTime, 0, 100)

	tests := []struct {
		name      string
		limit     int
		wantCount int
		wantFirst int64 // Expected first sequence
		wantLast  int64 // Expected last sequence
	}{
		{
			name:      "no limit returns all",
			limit:     0,
			wantCount: 100,
			wantFirst: 1,
			wantLast:  100,
		},
		{
			name:      "limit 50 returns last 50 (newest)",
			limit:     50,
			wantCount: 50,
			wantFirst: 51,
			wantLast:  100,
		},
		{
			name:      "limit exceeds buffer returns all",
			limit:     200,
			wantCount: 100,
			wantFirst: 1,
			wantLast:  100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, metadata, err := ApplyLogCursorPagination(entries, "", "", "", tt.limit, false)
			if err != nil {
				t.Fatalf("ApplyLogCursorPagination() unexpected error: %v", err)
			}

			assertPaginationCountAndTotal(t, len(result), tt.wantCount, metadata, 100)

			if len(result) > 0 {
				if result[0].Sequence != tt.wantFirst {
					t.Errorf("First sequence = %d, want %d", result[0].Sequence, tt.wantFirst)
				}
				if result[len(result)-1].Sequence != tt.wantLast {
					t.Errorf("Last sequence = %d, want %d", result[len(result)-1].Sequence, tt.wantLast)
				}

				assertPaginationCursorFields(
					t,
					metadata,
					result[0].Timestamp,
					result[len(result)-1].Timestamp,
					result[len(result)-1].Timestamp,
					result[len(result)-1].Sequence,
				)
			} else {
				assertPaginationEmptyCursor(t, metadata)
			}
		})
	}
}

func TestApplyLogCursorPagination_AfterCursor(t *testing.T) {
	baseTime := time.Date(2026, 1, 30, 10, 0, 0, 0, time.UTC)
	entries := buildSequentialLogEntries(baseTime, 0, 100)

	runAfterCursorPaginationCases(
		t,
		len(entries),
		[]paginationAfterCursorCase{
			{
				name:             "after cursor gets older entries",
				afterCursor:      BuildCursor(entries[50].Timestamp, entries[50].Sequence), // After entry 51
				limit:            0,
				expectedCount:    50, // Entries 1-50 are older
				expectedFirstSeq: 1,
				expectedLastSeq:  50,
				expectedHasMore:  false,
			},
			{
				name:             "after cursor with limit",
				afterCursor:      BuildCursor(entries[50].Timestamp, entries[50].Sequence),
				limit:            25,
				expectedCount:    25,
				expectedFirstSeq: 26, // Last 25 of the older entries
				expectedLastSeq:  50,
				expectedHasMore:  true,
			},
			{
				name:             "after cursor at beginning returns empty",
				afterCursor:      BuildCursor(entries[0].Timestamp, entries[0].Sequence),
				limit:            0,
				expectedCount:    0,
				expectedFirstSeq: 0,
				expectedLastSeq:  0,
				expectedHasMore:  false,
			},
			{
				name:             "after cursor at end returns all but last",
				afterCursor:      BuildCursor(entries[99].Timestamp, entries[99].Sequence),
				limit:            0,
				expectedCount:    99,
				expectedFirstSeq: 1,
				expectedLastSeq:  99,
				expectedHasMore:  false,
			},
		},
		func(afterCursor, beforeCursor string, limit int, restartOnEviction bool) ([]LogEntryWithSequence, *CursorPaginationMetadata, error) {
			return ApplyLogCursorPagination(entries, afterCursor, beforeCursor, "", limit, restartOnEviction)
		},
		func(entry LogEntryWithSequence) int64 { return entry.Sequence },
		func(entry LogEntryWithSequence) string { return entry.Timestamp },
	)
}

// TestApplyLogCursorPagination_AfterCursorWalkNoOverlap is the regression test
// for the after_cursor+limit pagination overlap bug: the continuation cursor
// for an after-walk (older history) was built from the newest returned entry,
// so each page re-fetched most of the previous one. A full multi-page walk
// must visit every entry exactly once and terminate.
func TestApplyLogCursorPagination_AfterCursorWalkNoOverlap(t *testing.T) {
	baseTime := time.Date(2026, 1, 30, 10, 0, 0, 0, time.UTC)
	const total = 100
	const limit = 25
	entries := buildSequentialLogEntries(baseTime, 0, total)

	seen := make(map[int64]bool, total)
	recordPage := func(page []LogEntryWithSequence) {
		t.Helper()
		for _, e := range page {
			if seen[e.Sequence] {
				t.Fatalf("Sequence %d returned twice — pages overlap", e.Sequence)
			}
			seen[e.Sequence] = true
		}
	}

	// Page 1: default view returns the newest entries (76-100).
	page, metadata, err := ApplyLogCursorPagination(entries, "", "", "", limit, false)
	if err != nil {
		t.Fatalf("Unexpected error on first page: %v", err)
	}
	if len(page) != limit {
		t.Fatalf("First page count = %d, want %d", len(page), limit)
	}
	recordPage(page)

	// Walk older history. Each continuation cursor must come from the OLDEST
	// entry of the previous page; with the buggy newest-entry cursor this walk
	// never advances and the page guard below trips.
	cursor := BuildCursor(page[0].Timestamp, page[0].Sequence)
	const maxPages = total/limit + 2 // guard against a non-advancing walk
	for pages := 0; ; pages++ {
		if pages > maxPages {
			t.Fatalf("Walk did not terminate after %d pages — cursor is not advancing", maxPages)
		}
		page, metadata, err = ApplyLogCursorPagination(entries, cursor, "", "", limit, false)
		if err != nil {
			t.Fatalf("Unexpected error during walk: %v", err)
		}
		if len(page) == 0 {
			break
		}
		recordPage(page)
		wantCursor := BuildCursor(page[0].Timestamp, page[0].Sequence)
		if metadata.Cursor != wantCursor {
			t.Fatalf("After-walk cursor = %q, want oldest returned entry %q", metadata.Cursor, wantCursor)
		}
		cursor = metadata.Cursor
	}

	// Complete coverage: every sequence 1..total visited exactly once.
	if len(seen) != total {
		t.Fatalf("Walk covered %d entries, want %d", len(seen), total)
	}
	for seq := int64(1); seq <= total; seq++ {
		if !seen[seq] {
			t.Errorf("Sequence %d was never returned", seq)
		}
	}
}

func TestApplyLogCursorPagination_BeforeCursor(t *testing.T) {
	baseTime := time.Date(2026, 1, 30, 10, 0, 0, 0, time.UTC)
	entries := buildSequentialLogEntries(baseTime, 0, 100)

	runBeforeCursorPaginationCases(
		t,
		len(entries),
		[]paginationBeforeCursorCase{
			{
				name:             "before cursor gets newer entries",
				beforeCursor:     BuildCursor(entries[50].Timestamp, entries[50].Sequence), // Before entry 51
				limit:            0,
				expectedCount:    49, // Entries 52-100 are newer
				expectedFirstSeq: 52,
				expectedLastSeq:  100,
			},
			{
				name:             "before cursor with limit takes first N",
				beforeCursor:     BuildCursor(entries[50].Timestamp, entries[50].Sequence),
				limit:            25,
				expectedCount:    25,
				expectedFirstSeq: 52, // First 25 of the newer entries
				expectedLastSeq:  76,
			},
			{
				name:             "before cursor at end returns empty",
				beforeCursor:     BuildCursor(entries[99].Timestamp, entries[99].Sequence),
				limit:            0,
				expectedCount:    0,
				expectedFirstSeq: 0,
				expectedLastSeq:  0,
			},
			{
				name:             "before cursor at beginning returns all but first",
				beforeCursor:     BuildCursor(entries[0].Timestamp, entries[0].Sequence),
				limit:            0,
				expectedCount:    99,
				expectedFirstSeq: 2,
				expectedLastSeq:  100,
			},
		},
		func(afterCursor, beforeCursor string, limit int, restartOnEviction bool) ([]LogEntryWithSequence, *CursorPaginationMetadata, error) {
			return ApplyLogCursorPagination(entries, afterCursor, beforeCursor, "", limit, restartOnEviction)
		},
		func(entry LogEntryWithSequence) int64 { return entry.Sequence },
		func(entry LogEntryWithSequence) string { return entry.Timestamp },
	)
}

func TestApplyLogCursorPagination_CursorExpired(t *testing.T) {
	baseTime := time.Date(2026, 1, 30, 10, 0, 0, 0, time.UTC)
	// Buffer has entries 101-200 (first 100 were evicted)
	entries := buildSequentialLogEntries(baseTime, 100, 100)

	t.Run("expired cursor without restart returns error", func(t *testing.T) {
		// Cursor points to sequence 50 (which was evicted)
		expiredCursor := BuildCursor(baseTime.Add(50*time.Second).Format(time.RFC3339), 50)

		result, metadata, err := ApplyLogCursorPagination(entries, expiredCursor, "", "", 10, false)
		if err == nil {
			t.Fatal("ApplyLogCursorPagination() expected error for expired cursor, got nil")
		}

		// Should return nil results on error
		if result != nil {
			t.Errorf("Result should be nil on error, got %d entries", len(result))
		}
		if metadata != nil {
			t.Errorf("Metadata should be nil on error, got %+v", metadata)
		}

		wantErrSubstr := "cursor expired"
		if !contains(err.Error(), wantErrSubstr) {
			t.Errorf("Error = %v, want error containing %q", err, wantErrSubstr)
		}
	})

	t.Run("expired cursor with restart returns oldest available", func(t *testing.T) {
		// Cursor points to sequence 50 (which was evicted)
		expiredCursor := BuildCursor(baseTime.Add(50*time.Second).Format(time.RFC3339), 50)

		result, metadata, err := ApplyLogCursorPagination(entries, expiredCursor, "", "", 10, true)
		if err != nil {
			t.Fatalf("ApplyLogCursorPagination() unexpected error: %v", err)
		}

		// Should return limited entries from restart (limit=10)
		if len(result) != 10 {
			t.Errorf("Result count = %d, want 10 (limit applied after restart)", len(result))
		}

		// Should start from oldest available (sequence 101)
		if result[0].Sequence != 101 {
			t.Errorf("First sequence = %d, want 101 (oldest after restart)", result[0].Sequence)
		}

		if !metadata.CursorRestarted {
			t.Error("Metadata.CursorRestarted = false, want true")
		}

		if metadata.OriginalCursor != expiredCursor {
			t.Errorf("Metadata.OriginalCursor = %v, want %v", metadata.OriginalCursor, expiredCursor)
		}

		if metadata.Warning == "" {
			t.Error("Metadata.Warning is empty, want warning message")
		}

		if len(result) > 0 {
			// Restarted walks paginate forward, so the cursor is the newest entry.
			assertPaginationCursorFields(
				t,
				metadata,
				result[0].Timestamp,
				result[len(result)-1].Timestamp,
				result[len(result)-1].Timestamp,
				result[len(result)-1].Sequence,
			)
		}
	})
}
