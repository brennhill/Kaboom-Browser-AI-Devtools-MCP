// Purpose: Tests the change-coupled action, log, and WebSocket pagination adapters.
// Docs: docs/features/feature/pagination/index.md

package pagination

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestEnrichActionEntries(t *testing.T) {
	tests := []struct {
		name             string
		actions          []types.EnhancedAction
		actionTotalAdded int64
		expectedFirstSeq int64
		expectedLastSeq  int64
		expectedCount    int
	}{
		{
			name:             "empty buffer",
			actions:          []types.EnhancedAction{},
			actionTotalAdded: 0,
			expectedFirstSeq: 0,
			expectedLastSeq:  0,
			expectedCount:    0,
		},
		{
			name: "single action",
			actions: []types.EnhancedAction{
				{Type: "click", Timestamp: 1738238123456, URL: "https://example.com"},
			},
			actionTotalAdded: 1,
			expectedFirstSeq: 1,
			expectedLastSeq:  1,
			expectedCount:    1,
		},
		{
			name: "multiple actions no eviction",
			actions: []types.EnhancedAction{
				{Type: "click", Timestamp: 1738238123000, URL: "https://example.com"},
				{Type: "input", Timestamp: 1738238124000, URL: "https://example.com"},
				{Type: "navigate", Timestamp: 1738238125000, URL: "https://example.com/page2"},
			},
			actionTotalAdded: 3,
			expectedFirstSeq: 1,
			expectedLastSeq:  3,
			expectedCount:    3,
		},
		{
			name: "buffer with evictions (actionTotalAdded > len)",
			actions: []types.EnhancedAction{
				{Type: "click", Timestamp: 1738238200000, URL: "https://example.com"},
				{Type: "input", Timestamp: 1738238201000, URL: "https://example.com"},
			},
			actionTotalAdded: 152, // 150 evicted, 2 remain
			expectedFirstSeq: 151, // First entry is sequence 151
			expectedLastSeq:  152, // Last entry is sequence 152
			expectedCount:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enriched := EnrichActionEntries(tt.actions, tt.actionTotalAdded)

			assertEnrichedEntryRange(
				t,
				enriched,
				tt.expectedCount,
				tt.expectedFirstSeq,
				tt.expectedLastSeq,
				func(entry ActionEntryWithSequence) int64 {
					return entry.Sequence
				},
			)

			if len(enriched) > 0 {
				// Verify timestamps are normalized to RFC3339
				for i, e := range enriched {
					if e.Timestamp == "" {
						t.Errorf("Entry %d: timestamp is empty", i)
					}
					// Timestamp should be RFC3339 string like "2026-01-30T10:15:23Z"
					if len(e.Timestamp) < 20 {
						t.Errorf("Entry %d: timestamp %q looks invalid", i, e.Timestamp)
					}
				}
			}
		})
	}
}

func TestApplyActionCursorPagination_NoCursor(t *testing.T) {
	// Create 100 actions
	actions := make([]types.EnhancedAction, 100)
	for i := 0; i < 100; i++ {
		actions[i] = types.EnhancedAction{
			Type:      "click",
			Timestamp: int64(1738238000000 + i*1000), // 1 second apart
			URL:       "https://example.com",
		}
	}
	enriched := EnrichActionEntries(actions, 100)

	runNoCursorPaginationCases(
		t,
		100,
		[]paginationNoCursorCase{
			{
				name:          "no limit returns all",
				limit:         0,
				expectedCount: 100,
			},
			{
				name:          "limit 50 returns last 50",
				limit:         50,
				expectedCount: 50,
			},
			{
				name:          "limit exceeds buffer size",
				limit:         200,
				expectedCount: 100,
			},
		},
		func(afterCursor, beforeCursor string, limit int, restartOnEviction bool) ([]ActionEntryWithSequence, *CursorPaginationMetadata, error) {
			return ApplyActionCursorPagination(enriched, afterCursor, beforeCursor, "", limit, restartOnEviction)
		},
		func(entry ActionEntryWithSequence) int64 {
			return entry.Sequence
		},
		func(entry ActionEntryWithSequence) string {
			return entry.Timestamp
		},
	)
}

func TestApplyActionCursorPagination_AfterCursor(t *testing.T) {
	// Create 100 actions (sequences 1-100)
	actions := make([]types.EnhancedAction, 100)
	for i := 0; i < 100; i++ {
		actions[i] = types.EnhancedAction{
			Type:      "click",
			Timestamp: int64(1738238000000 + i*1000),
			URL:       "https://example.com",
		}
	}
	enriched := EnrichActionEntries(actions, 100)

	cursors := buildPaginationCursorSet(
		enriched,
		func(entry ActionEntryWithSequence) string { return entry.Timestamp },
		func(entry ActionEntryWithSequence) int64 { return entry.Sequence },
	)

	runAfterCursorPaginationCases(
		t,
		len(enriched),
		standardAfterCursorCases(cursors),
		func(afterCursor, beforeCursor string, limit int, restartOnEviction bool) ([]ActionEntryWithSequence, *CursorPaginationMetadata, error) {
			return ApplyActionCursorPagination(enriched, afterCursor, beforeCursor, "", limit, restartOnEviction)
		},
		func(entry ActionEntryWithSequence) int64 {
			return entry.Sequence
		},
		func(entry ActionEntryWithSequence) string {
			return entry.Timestamp
		},
	)
}

func TestApplyActionCursorPagination_BeforeCursor(t *testing.T) {
	// Create 100 actions (sequences 1-100)
	actions := make([]types.EnhancedAction, 100)
	for i := 0; i < 100; i++ {
		actions[i] = types.EnhancedAction{
			Type:      "click",
			Timestamp: int64(1738238000000 + i*1000),
			URL:       "https://example.com",
		}
	}
	enriched := EnrichActionEntries(actions, 100)

	// Build cursor from actual enriched data
	cursor50 := BuildCursor(enriched[49].Timestamp, enriched[49].Sequence) // Sequence 50

	tests := []struct {
		name             string
		beforeCursor     string
		limit            int
		expectedCount    int
		expectedFirstSeq int64
		expectedLastSeq  int64
	}{
		{
			name:             "before cursor gets newer entries",
			beforeCursor:     cursor50, // Cursor at sequence 50
			limit:            0,
			expectedCount:    50, // Sequences 51-100
			expectedFirstSeq: 51,
			expectedLastSeq:  100,
		},
		{
			name:             "before cursor with limit",
			beforeCursor:     cursor50, // Cursor at sequence 50
			limit:            10,
			expectedCount:    10, // First 10 of sequences 51-100 = sequences 51-60
			expectedFirstSeq: 51,
			expectedLastSeq:  60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, metadata, err := ApplyActionCursorPagination(enriched, "", tt.beforeCursor, "", tt.limit, false)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			assertPaginationCountAndTotal(t, len(result), tt.expectedCount, metadata, len(enriched))

			if tt.expectedCount > 0 {
				firstSeq := result[0].Sequence
				if firstSeq != tt.expectedFirstSeq {
					t.Errorf("First sequence = %d, want %d", firstSeq, tt.expectedFirstSeq)
				}

				lastSeq := result[len(result)-1].Sequence
				if lastSeq != tt.expectedLastSeq {
					t.Errorf("Last sequence = %d, want %d", lastSeq, tt.expectedLastSeq)
				}

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
}

func TestApplyActionCursorPagination_CursorExpired(t *testing.T) {
	// Buffer has sequences 101-200 (100 entries evicted)
	actions := make([]types.EnhancedAction, 100)
	for i := 0; i < 100; i++ {
		actions[i] = types.EnhancedAction{
			Type:      "click",
			Timestamp: int64(1738238000000 + (100+i)*1000), // Start from 100 seconds in
			URL:       "https://example.com",
		}
	}
	enriched := EnrichActionEntries(actions, 200) // 200 total added, 100 evicted

	// Build a cursor for an evicted sequence (sequence 50, which is before sequence 101)
	// Use a timestamp that would correspond to an older action
	expiredCursor := BuildCursor(NormalizeTimestamp(int64(1738238000000+50*1000)), 50)

	runCursorExpiredPaginationCases(
		t,
		len(enriched),
		[]paginationCursorExpiredCase{
			{
				name:              "expired cursor without restart returns error",
				afterCursor:       expiredCursor, // Cursor at evicted sequence 50
				limit:             0,
				restartOnEviction: false,
				expectError:       true,
			},
			{
				name:                  "expired cursor with restart returns oldest available",
				afterCursor:           expiredCursor, // Cursor at evicted sequence 50
				limit:                 0,
				restartOnEviction:     true,
				expectError:           false,
				expectedCount:         100, // All 100 available entries (no limit)
				expectedFirstSeq:      101, // Oldest available is sequence 101
				expectedCursorRestart: true,
			},
			{
				name:                  "expired cursor with restart and limit",
				afterCursor:           expiredCursor, // Cursor at evicted sequence 50
				limit:                 10,            // Limit applied
				restartOnEviction:     true,
				expectError:           false,
				expectedCount:         10,
				expectedFirstSeq:      101, // After restart, take FIRST 10 entries from oldest
				expectedCursorRestart: true,
			},
		},
		func(afterCursor, beforeCursor string, limit int, restartOnEviction bool) ([]ActionEntryWithSequence, *CursorPaginationMetadata, error) {
			return ApplyActionCursorPagination(enriched, afterCursor, beforeCursor, "", limit, restartOnEviction)
		},
		func(entry ActionEntryWithSequence) int64 {
			return entry.Sequence
		},
		func(entry ActionEntryWithSequence) string {
			return entry.Timestamp
		},
	)
}

func TestSerializeActionEntryWithSequence(t *testing.T) {
	action := ActionEntryWithSequence{
		Entry: types.EnhancedAction{
			Type:      "click",
			Timestamp: 1738238123456,
			URL:       "https://example.com",
			Selectors: map[string]any{
				"testId": "submit-button",
				"role":   "button",
			},
			TabID: 123,
		},
		Sequence:  5678,
		Timestamp: "2026-01-30T10:15:23Z",
	}

	result := SerializeActionEntryWithSequence(action)

	// Verify required fields
	if result["type"] != "click" {
		t.Errorf("type = %v, want 'click'", result["type"])
	}

	if result["url"] != "https://example.com" {
		t.Errorf("url = %v, want 'https://example.com'", result["url"])
	}

	if result["timestamp"] != "2026-01-30T10:15:23Z" {
		t.Errorf("timestamp = %v, want '2026-01-30T10:15:23Z'", result["timestamp"])
	}

	if result["sequence"] != int64(5678) {
		t.Errorf("sequence = %v, want 5678", result["sequence"])
	}

	// Verify selectors preserved
	selectors, ok := result["selectors"].(map[string]any)
	if !ok {
		t.Errorf("selectors not a map")
	} else {
		if selectors["testId"] != "submit-button" {
			t.Errorf("selectors.testId = %v, want 'submit-button'", selectors["testId"])
		}
	}

	// Verify tabId included
	if result["tab_id"] != 123 {
		t.Errorf("tab_id = %v, want 123", result["tab_id"])
	}
}

func TestSerializeActionEntryWithSequence_NoTabID(t *testing.T) {
	action := ActionEntryWithSequence{
		Entry: types.EnhancedAction{
			Type:      "navigate",
			Timestamp: 1738238123456,
			URL:       "https://example.com",
		},
		Sequence:  1,
		Timestamp: "2026-01-30T10:15:23Z",
	}

	result := SerializeActionEntryWithSequence(action)

	// Verify tabId not included when zero
	if _, exists := result["tab_id"]; exists {
		t.Errorf("tab_id should not be included when zero, got %v", result["tab_id"])
	}
}

func TestSerializeActionEntryWithSequence_AllOptionalFields(t *testing.T) {
	t.Parallel()
	action := ActionEntryWithSequence{
		Entry: types.EnhancedAction{
			Type:          "click",
			Timestamp:     1738238123456,
			URL:           "https://example.com",
			Selectors:     map[string]any{"css": "button"},
			Value:         "submit",
			InputType:     "button",
			Key:           "Enter",
			FromURL:       "https://example.com/page1",
			ToURL:         "https://example.com/page2",
			SelectedValue: "option1",
			SelectedText:  "Option 1",
			ScrollY:       500,
			TabID:         42,
		},
		Sequence:  10,
		Timestamp: "2026-01-30T10:15:23Z",
	}
	result := SerializeActionEntryWithSequence(action)

	// Verify all fields
	checks := map[string]any{
		"type":           "click",
		"timestamp":      "2026-01-30T10:15:23Z",
		"sequence":       int64(10),
		"url":            "https://example.com",
		"value":          "submit",
		"input_type":     "button",
		"key":            "Enter",
		"from_url":       "https://example.com/page1",
		"to_url":         "https://example.com/page2",
		"selected_value": "option1",
		"selected_text":  "Option 1",
		"scroll_y":       500,
		"tab_id":         42,
	}
	for key, want := range checks {
		got, exists := result[key]
		if !exists {
			t.Errorf("missing key %q in serialized action", key)
			continue
		}
		if got != want {
			t.Errorf("result[%q] = %v (%T), want %v (%T)", key, got, got, want, want)
		}
	}
	// Verify selectors is a map
	if _, ok := result["selectors"].(map[string]any); !ok {
		t.Error("selectors should be a map[string]any")
	}
}

func TestSerializeActionEntryWithSequence_NoOptionalFields(t *testing.T) {
	t.Parallel()

	// Keys the serializer emits only when the corresponding field is set.
	optional := []string{"url", "value", "input_type", "key", "from_url", "to_url",
		"selected_value", "selected_text", "scroll_y", "tab_id", "selectors"}

	// Discriminating control: a fully populated entry must emit every one of
	// those keys. This proves each name is really produced by the serializer,
	// so their absence below is meaningful — a nil/empty map (or a misspelled
	// key in this list) would otherwise satisfy the absence checks trivially.
	populated := SerializeActionEntryWithSequence(ActionEntryWithSequence{
		Entry: types.EnhancedAction{
			Type:          "click",
			Timestamp:     1738238123456,
			URL:           "https://example.com",
			Selectors:     map[string]any{"css": "button"},
			Value:         "submit",
			InputType:     "button",
			Key:           "Enter",
			FromURL:       "https://example.com/page1",
			ToURL:         "https://example.com/page2",
			SelectedValue: "option1",
			SelectedText:  "Option 1",
			ScrollY:       500,
			TabID:         42,
		},
		Sequence:  2,
		Timestamp: "2026-01-30T10:15:23Z",
	})
	for _, key := range optional {
		if _, exists := populated[key]; !exists {
			t.Fatalf("control: key %q missing for a fully populated entry; the absence assertions would be meaningless", key)
		}
	}

	// Subject: nothing optional is set, so none of those keys may appear.
	result := SerializeActionEntryWithSequence(ActionEntryWithSequence{
		Entry: types.EnhancedAction{
			Type:      "navigate",
			Timestamp: 1738238123456,
		},
		Sequence:  1,
		Timestamp: "2026-01-30T10:15:23Z",
	})
	for _, key := range []string{"type", "timestamp", "sequence"} {
		if _, exists := result[key]; !exists {
			t.Fatalf("required key %q missing — the serializer produced nothing to assert against", key)
		}
	}
	for _, key := range optional {
		if _, exists := result[key]; exists {
			t.Errorf("key %q should not be present when empty/zero, got %v", key, result[key])
		}
	}
}

// ============================================
// SerializeWebSocketEntryWithSequence — all optional fields
// ============================================
