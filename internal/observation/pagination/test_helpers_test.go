// Purpose: Shared test assertions/helpers for pagination test suites.
// Docs: docs/features/feature/pagination/index.md

package pagination

import "testing"

type paginationNoCursorCase struct {
	name          string
	limit         int
	expectedCount int
}

type paginationAfterCursorCase struct {
	name             string
	afterCursor      string
	limit            int
	expectedCount    int
	expectedFirstSeq int64
	expectedLastSeq  int64
	expectedHasMore  bool
}

type paginationBeforeCursorCase struct {
	name             string
	beforeCursor     string
	limit            int
	expectedCount    int
	expectedFirstSeq int64
	expectedLastSeq  int64
}

type paginationCursorExpiredCase struct {
	name                  string
	afterCursor           string
	limit                 int
	restartOnEviction     bool
	expectError           bool
	expectedCount         int
	expectedFirstSeq      int64
	expectedCursorRestart bool
}

func runNoCursorPaginationCases[T any](
	t *testing.T,
	total int,
	cases []paginationNoCursorCase,
	paginate func(afterCursor, beforeCursor string, limit int, restartOnEviction bool) ([]T, *CursorPaginationMetadata, error),
	sequenceFor func(T) int64,
	timestampFor func(T) string,
) {
	t.Helper()
	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			result, metadata, err := paginate("", "", tt.limit, false)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			assertPaginationCountAndTotal(t, len(result), tt.expectedCount, metadata, total)

			if tt.limit > 0 && tt.limit < total && len(result) > 0 {
				expectedFirstSeq := int64(total - tt.expectedCount + 1)
				if sequenceFor(result[0]) != expectedFirstSeq {
					t.Errorf("First sequence = %d, want %d (should be last %d entries)", sequenceFor(result[0]), expectedFirstSeq, tt.expectedCount)
				}
			}

			if len(result) > 0 {
				assertPaginationCursorFields(
					t,
					metadata,
					timestampFor(result[0]),
					timestampFor(result[len(result)-1]),
					timestampFor(result[len(result)-1]),
					sequenceFor(result[len(result)-1]),
				)
				return
			}
			assertPaginationEmptyCursor(t, metadata)
		})
	}

}

func runAfterCursorPaginationCases[T any](
	t *testing.T,
	total int,
	cases []paginationAfterCursorCase,
	paginate func(afterCursor, beforeCursor string, limit int, restartOnEviction bool) ([]T, *CursorPaginationMetadata, error),
	sequenceFor func(T) int64,
	timestampFor func(T) string,
) {
	t.Helper()
	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			result, metadata, err := paginate(tt.afterCursor, "", tt.limit, false)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			assertDirectionalCursorResult(
				t,
				total,
				tt.expectedCount,
				tt.expectedFirstSeq,
				tt.expectedLastSeq,
				true, // after-walks continue from the oldest returned entry
				result,
				metadata,
				sequenceFor,
				timestampFor,
			)
			if metadata.HasMore != tt.expectedHasMore {
				t.Errorf("HasMore = %v, want %v", metadata.HasMore, tt.expectedHasMore)
			}
		})
	}
}

func runBeforeCursorPaginationCases[T any](
	t *testing.T,
	total int,
	cases []paginationBeforeCursorCase,
	paginate func(afterCursor, beforeCursor string, limit int, restartOnEviction bool) ([]T, *CursorPaginationMetadata, error),
	sequenceFor func(T) int64,
	timestampFor func(T) string,
) {
	t.Helper()
	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			result, metadata, err := paginate("", tt.beforeCursor, tt.limit, false)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			assertDirectionalCursorResult(
				t,
				total,
				tt.expectedCount,
				tt.expectedFirstSeq,
				tt.expectedLastSeq,
				false, // before-walks continue from the newest returned entry
				result,
				metadata,
				sequenceFor,
				timestampFor,
			)
		})
	}
}

func assertDirectionalCursorResult[T any](
	t *testing.T,
	total int,
	expectedCount int,
	expectedFirstSeq int64,
	expectedLastSeq int64,
	cursorFromOldest bool,
	result []T,
	metadata *CursorPaginationMetadata,
	sequenceFor func(T) int64,
	timestampFor func(T) string,
) {
	t.Helper()
	assertPaginationCountAndTotal(t, len(result), expectedCount, metadata, total)
	if expectedCount == 0 {
		assertPaginationEmptyCursor(t, metadata)
		return
	}

	if sequenceFor(result[0]) != expectedFirstSeq {
		t.Errorf("First sequence = %d, want %d", sequenceFor(result[0]), expectedFirstSeq)
	}
	if sequenceFor(result[len(result)-1]) != expectedLastSeq {
		t.Errorf("Last sequence = %d, want %d", sequenceFor(result[len(result)-1]), expectedLastSeq)
	}
	cursorEntry := result[len(result)-1]
	if cursorFromOldest {
		cursorEntry = result[0]
	}
	assertPaginationCursorFields(
		t,
		metadata,
		timestampFor(result[0]),
		timestampFor(result[len(result)-1]),
		timestampFor(cursorEntry),
		sequenceFor(cursorEntry),
	)
}

func runCursorExpiredPaginationCases[T any](
	t *testing.T,
	total int,
	cases []paginationCursorExpiredCase,
	paginate func(afterCursor, beforeCursor string, limit int, restartOnEviction bool) ([]T, *CursorPaginationMetadata, error),
	sequenceFor func(T) int64,
	timestampFor func(T) string,
) {
	t.Helper()
	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			result, metadata, err := paginate(tt.afterCursor, "", tt.limit, tt.restartOnEviction)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			assertPaginationCountAndTotal(t, len(result), tt.expectedCount, metadata, total)
			if tt.expectedCount > 0 {
				if sequenceFor(result[0]) != tt.expectedFirstSeq {
					t.Errorf("First sequence = %d, want %d (oldest after restart)", sequenceFor(result[0]), tt.expectedFirstSeq)
				}
				// Restarted walks paginate forward, so the cursor is the newest entry.
				assertPaginationCursorFields(
					t,
					metadata,
					timestampFor(result[0]),
					timestampFor(result[len(result)-1]),
					timestampFor(result[len(result)-1]),
					sequenceFor(result[len(result)-1]),
				)
			}

			if metadata.CursorRestarted != tt.expectedCursorRestart {
				t.Errorf("CursorRestarted = %v, want %v", metadata.CursorRestarted, tt.expectedCursorRestart)
			}
			if tt.expectedCursorRestart && metadata.Warning == "" {
				t.Error("Expected warning when cursor restarted, got empty string")
			}
		})
	}
}

func assertEnrichedEntryRange[T any](
	t *testing.T,
	enriched []T,
	expectedCount int,
	expectedFirstSeq int64,
	expectedLastSeq int64,
	sequenceFor func(T) int64,
) {
	t.Helper()
	if len(enriched) != expectedCount {
		t.Errorf("Enriched count = %d, want %d", len(enriched), expectedCount)
	}
	if len(enriched) == 0 {
		return
	}

	firstSeq := sequenceFor(enriched[0])
	if firstSeq != expectedFirstSeq {
		t.Errorf("First sequence = %d, want %d", firstSeq, expectedFirstSeq)
	}

	lastSeq := sequenceFor(enriched[len(enriched)-1])
	if lastSeq != expectedLastSeq {
		t.Errorf("Last sequence = %d, want %d", lastSeq, expectedLastSeq)
	}
}

type paginationCursorSet struct {
	Cursor1   string
	Cursor50  string
	Cursor100 string
}

func buildPaginationCursorSet[T any](
	entries []T,
	timestampFor func(T) string,
	sequenceFor func(T) int64,
) paginationCursorSet {
	return paginationCursorSet{
		Cursor1:   BuildCursor(timestampFor(entries[0]), sequenceFor(entries[0])),
		Cursor50:  BuildCursor(timestampFor(entries[49]), sequenceFor(entries[49])),
		Cursor100: BuildCursor(timestampFor(entries[99]), sequenceFor(entries[99])),
	}
}

func standardAfterCursorCases(cursors paginationCursorSet) []paginationAfterCursorCase {
	return []paginationAfterCursorCase{
		{
			name:             "after cursor gets older entries",
			afterCursor:      cursors.Cursor50,
			limit:            0,
			expectedCount:    49,
			expectedFirstSeq: 1,
			expectedLastSeq:  49,
			expectedHasMore:  false,
		},
		{
			name:             "after cursor with limit",
			afterCursor:      cursors.Cursor50,
			limit:            10,
			expectedCount:    10,
			expectedFirstSeq: 40,
			expectedLastSeq:  49,
			expectedHasMore:  true,
		},
		{
			name:             "after cursor at beginning",
			afterCursor:      cursors.Cursor1,
			limit:            0,
			expectedCount:    0,
			expectedFirstSeq: 0,
			expectedLastSeq:  0,
			expectedHasMore:  false,
		},
		{
			name:             "after cursor at end",
			afterCursor:      cursors.Cursor100,
			limit:            0,
			expectedCount:    99,
			expectedFirstSeq: 1,
			expectedLastSeq:  99,
			expectedHasMore:  false,
		},
	}
}

func assertPaginationCountAndTotal(t *testing.T, gotCount, wantCount int, metadata *CursorPaginationMetadata, expectedTotal int) {
	t.Helper()
	if metadata == nil {
		t.Fatal("metadata is nil")
	}
	if gotCount != wantCount {
		t.Errorf("Result count = %d, want %d", gotCount, wantCount)
	}
	if metadata.Count != wantCount {
		t.Errorf("Metadata count = %d, want %d", metadata.Count, wantCount)
	}
	if metadata.Total != expectedTotal {
		t.Errorf("Metadata total = %d, want %d", metadata.Total, expectedTotal)
	}
}

func assertPaginationCursorFields(
	t *testing.T,
	metadata *CursorPaginationMetadata,
	oldestTimestamp string,
	newestTimestamp string,
	cursorTimestamp string,
	cursorSequence int64,
) {
	t.Helper()
	if metadata == nil {
		t.Fatal("metadata is nil")
	}
	expectedCursor := BuildCursor(cursorTimestamp, cursorSequence)
	if metadata.Cursor != expectedCursor {
		t.Errorf("Metadata cursor = %v, want %v", metadata.Cursor, expectedCursor)
	}
	if metadata.OldestTimestamp != oldestTimestamp {
		t.Errorf("OldestTimestamp = %v, want %v", metadata.OldestTimestamp, oldestTimestamp)
	}
	if metadata.NewestTimestamp != newestTimestamp {
		t.Errorf("NewestTimestamp = %v, want %v", metadata.NewestTimestamp, newestTimestamp)
	}
}

func assertPaginationEmptyCursor(t *testing.T, metadata *CursorPaginationMetadata) {
	t.Helper()
	if metadata == nil {
		t.Fatal("metadata is nil")
	}
	if metadata.Cursor != "" {
		t.Errorf("Metadata cursor should be empty for empty results, got %v", metadata.Cursor)
	}
}
