// Purpose: Coverage-expansion tests for pagination and cursor edge cases and branch paths.
// Docs: docs/features/feature/pagination/index.md

// pagination_coverage_test.go — Targeted tests for uncovered branches in pagination package.
package pagination

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"testing"
)

// ============================================
// ParseCursor — no-colon format error path
// ============================================

func TestParseCursor_NoColon(t *testing.T) {
	t.Parallel()
	_, err := ParseCursor("nocolon")
	if err == nil {
		t.Fatal("ParseCursor(\"nocolon\") expected error, got nil")
	}
	wantSubstr := "invalid cursor format"
	if !contains(err.Error(), wantSubstr) {
		t.Errorf("error = %q, want substring %q", err.Error(), wantSubstr)
	}
}

// ============================================
// IsOlder / IsNewer — RFC3339 fallback paths
// ============================================

func TestIsOlder_RFC3339FallbackCursorTimestamp(t *testing.T) {
	t.Parallel()
	// Cursor timestamp is plain RFC3339 (no nanoseconds) so RFC3339Nano parse
	// fails and the code falls back to RFC3339.
	cursor := Cursor{
		Timestamp: "2026-01-30T10:15:23Z",
		Sequence:  100,
	}
	// Entry uses RFC3339Nano — no fallback needed for the entry.
	// The cursor parse fails Nano, falls back to RFC3339.
	got := cursor.IsOlder("2026-01-30T10:15:22.000000Z", 99)
	if !got {
		t.Error("expected entry to be older than cursor")
	}
}

func TestIsOlder_RFC3339FallbackEntryTimestamp(t *testing.T) {
	t.Parallel()
	// Cursor timestamp is RFC3339Nano so first parse succeeds.
	// Entry timestamp is plain RFC3339 so Nano parse fails, falls back.
	cursor := Cursor{
		Timestamp: "2026-01-30T10:15:23.000000Z",
		Sequence:  100,
	}
	got := cursor.IsOlder("2026-01-30T10:15:22Z", 99)
	if !got {
		t.Error("expected entry to be older than cursor (entry RFC3339 fallback)")
	}
}

func TestIsNewer_RFC3339FallbackCursorTimestamp(t *testing.T) {
	t.Parallel()
	cursor := Cursor{
		Timestamp: "2026-01-30T10:15:23Z",
		Sequence:  100,
	}
	got := cursor.IsNewer("2026-01-30T10:15:24.000000Z", 101)
	if !got {
		t.Error("expected entry to be newer than cursor")
	}
}

func TestIsNewer_RFC3339FallbackEntryTimestamp(t *testing.T) {
	t.Parallel()
	cursor := Cursor{
		Timestamp: "2026-01-30T10:15:23.000000Z",
		Sequence:  100,
	}
	got := cursor.IsNewer("2026-01-30T10:15:24Z", 101)
	if !got {
		t.Error("expected entry to be newer than cursor (entry RFC3339 fallback)")
	}
}

// ============================================
// entryStr — missing key and non-string value
// ============================================

func TestEntryStr_MissingKey(t *testing.T) {
	t.Parallel()
	entry := types.LogEntry{"other": "value"}
	got := entryStr(entry, "missing")
	if got != "" {
		t.Errorf("entryStr(missing key) = %q, want empty", got)
	}
}

func TestEntryStr_NonStringValue(t *testing.T) {
	t.Parallel()
	entry := types.LogEntry{"count": 42}
	got := entryStr(entry, "count")
	if got != "" {
		t.Errorf("entryStr(int value) = %q, want empty", got)
	}
}

// ============================================
// resolveCursorType — "since" branch
// ============================================

func TestResolveCursorType_Since(t *testing.T) {
	t.Parallel()
	cursor, cursorType := resolveCursorType("", "", "2026-01-30T10:15:23Z:50")
	if cursor != "2026-01-30T10:15:23Z:50" {
		t.Errorf("cursor = %q, want since cursor string", cursor)
	}
	if cursorType != "since" {
		t.Errorf("cursorType = %q, want %q", cursorType, "since")
	}
}

func TestResolveCursorType_None(t *testing.T) {
	t.Parallel()
	cursor, cursorType := resolveCursorType("", "", "")
	if cursor != "" || cursorType != "" {
		t.Errorf("expected empty strings, got cursor=%q type=%q", cursor, cursorType)
	}
}

// ============================================
// matchesCursorType — "since" and "default" branches
// ============================================

func TestMatchesCursorType_Since_Exact(t *testing.T) {
	t.Parallel()
	cursor := Cursor{Timestamp: "2026-01-30T10:15:23Z", Sequence: 50}
	// "since" includes the exact match
	got := matchesCursorType(cursor, "since", "2026-01-30T10:15:23Z", 50)
	if !got {
		t.Error("since cursor should match exact timestamp+sequence")
	}
}

func TestMatchesCursorType_Since_Newer(t *testing.T) {
	t.Parallel()
	cursor := Cursor{Timestamp: "2026-01-30T10:15:23Z", Sequence: 50}
	got := matchesCursorType(cursor, "since", "2026-01-30T10:15:24Z", 51)
	if !got {
		t.Error("since cursor should match newer entries")
	}
}

func TestMatchesCursorType_Since_Older(t *testing.T) {
	t.Parallel()
	cursor := Cursor{Timestamp: "2026-01-30T10:15:23Z", Sequence: 50}
	got := matchesCursorType(cursor, "since", "2026-01-30T10:15:22Z", 49)
	if got {
		t.Error("since cursor should not match older entries")
	}
}

func TestMatchesCursorType_Default(t *testing.T) {
	t.Parallel()
	cursor := Cursor{Timestamp: "2026-01-30T10:15:23Z", Sequence: 50}
	got := matchesCursorType(cursor, "unknown", "2026-01-30T10:15:23Z", 50)
	if got {
		t.Error("unknown cursor type should return false")
	}
}

// ============================================
// checkCursorExpired — empty entries
// ============================================

func TestCheckCursorExpired_EmptyEntries(t *testing.T) {
	t.Parallel()
	metadata := &CursorPaginationMetadata{}
	err := checkCursorExpired([]LogEntryWithSequence{}, Cursor{Sequence: 10}, ":10", false, metadata)
	if err != nil {
		t.Errorf("checkCursorExpired(empty entries) should return nil, got %v", err)
	}
}

// ============================================
// ApplyCursorPagination — invalid cursor format
// ============================================

func TestApplyCursorPagination_InvalidCursorFormat(t *testing.T) {
	t.Parallel()
	entries := []LogEntryWithSequence{
		{Entry: types.LogEntry{}, Sequence: 1, Timestamp: "2026-01-30T10:15:23Z"},
	}
	_, _, err := ApplyLogCursorPagination(entries, "invalid-no-colon", "", "", 10, false)
	if err == nil {
		t.Fatal("expected error for invalid cursor format, got nil")
	}
	if !contains(err.Error(), "invalid cursor format") {
		t.Errorf("error = %q, want substring 'invalid cursor format'", err.Error())
	}
}

// ============================================
// ApplyCursorPagination — since cursor
// ============================================

func TestApplyLogCursorPagination_SinceCursor(t *testing.T) {
	t.Parallel()
	entries := []LogEntryWithSequence{
		{Entry: types.LogEntry{"ts": "2026-01-30T10:15:20Z"}, Sequence: 1, Timestamp: "2026-01-30T10:15:20Z"},
		{Entry: types.LogEntry{"ts": "2026-01-30T10:15:21Z"}, Sequence: 2, Timestamp: "2026-01-30T10:15:21Z"},
		{Entry: types.LogEntry{"ts": "2026-01-30T10:15:22Z"}, Sequence: 3, Timestamp: "2026-01-30T10:15:22Z"},
		{Entry: types.LogEntry{"ts": "2026-01-30T10:15:23Z"}, Sequence: 4, Timestamp: "2026-01-30T10:15:23Z"},
		{Entry: types.LogEntry{"ts": "2026-01-30T10:15:24Z"}, Sequence: 5, Timestamp: "2026-01-30T10:15:24Z"},
	}
	// since cursor at entry 2 should include entries 2, 3, 4, 5
	sinceCursor := BuildCursor("2026-01-30T10:15:21Z", 2)
	result, metadata, err := ApplyLogCursorPagination(entries, "", "", sinceCursor, 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 4 {
		t.Errorf("result count = %d, want 4 (since includes cursor entry)", len(result))
	}
	if result[0].Sequence != 2 {
		t.Errorf("first sequence = %d, want 2 (inclusive)", result[0].Sequence)
	}
	if result[len(result)-1].Sequence != 5 {
		t.Errorf("last sequence = %d, want 5", result[len(result)-1].Sequence)
	}
	if metadata.Count != 4 {
		t.Errorf("metadata count = %d, want 4", metadata.Count)
	}
	if metadata.Total != 5 {
		t.Errorf("metadata total = %d, want 5", metadata.Total)
	}
}

// ============================================
// SerializeActionEntryWithSequence — ScrollY, all optional fields
// ============================================

func TestSerializeActionEntryWithSequence_AllOptionalFields(t *testing.T) {
	t.Parallel()
	action := ActionEntryWithSequence{
		Entry: EnhancedAction{
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
	action := ActionEntryWithSequence{
		Entry: EnhancedAction{
			Type:      "navigate",
			Timestamp: 1738238123456,
		},
		Sequence:  1,
		Timestamp: "2026-01-30T10:15:23Z",
	}
	result := SerializeActionEntryWithSequence(action)

	// These keys should NOT be present when empty/zero
	absent := []string{"url", "value", "input_type", "key", "from_url", "to_url",
		"selected_value", "selected_text", "scroll_y", "tab_id", "selectors"}
	for _, key := range absent {
		if _, exists := result[key]; exists {
			t.Errorf("key %q should not be present when empty/zero, got %v", key, result[key])
		}
	}
}

// ============================================
// SerializeWebSocketEntryWithSequence — all optional fields
// ============================================

func TestSerializeWebSocketEntryWithSequence_AllOptionalFields(t *testing.T) {
	t.Parallel()
	sampled := &types.SamplingInfo{Rate: "1/10", Logged: "5", Window: "60s"}
	event := WebSocketEntryWithSequence{
		Entry: types.WebSocketEvent{
			Event:            "message",
			ID:               "ws-42",
			URL:              "wss://echo.example.com",
			Type:             "binary",
			Direction:        "outgoing",
			Data:             `{"ping":true}`,
			Size:             256,
			CloseCode:        1000,
			CloseReason:      "normal closure",
			BinaryFormat:     "protobuf",
			FormatConfidence: 0.95,
			Sampled:          sampled,
			TabID:            7,
			Timestamp:        "2026-01-30T10:15:23Z",
		},
		Sequence:  100,
		Timestamp: "2026-01-30T10:15:23Z",
	}
	result := SerializeWebSocketEntryWithSequence(event)

	stringChecks := map[string]string{
		"event":         "message",
		"id":            "ws-42",
		"url":           "wss://echo.example.com",
		"type":          "binary",
		"direction":     "outgoing",
		"data":          `{"ping":true}`,
		"reason":        "normal closure",
		"binary_format": "protobuf",
		"timestamp":     "2026-01-30T10:15:23Z",
	}
	for key, want := range stringChecks {
		got, exists := result[key]
		if !exists {
			t.Errorf("missing key %q", key)
			continue
		}
		if got != want {
			t.Errorf("result[%q] = %v, want %v", key, got, want)
		}
	}

	if result["sequence"] != int64(100) {
		t.Errorf("sequence = %v, want 100", result["sequence"])
	}
	if result["size"] != 256 {
		t.Errorf("size = %v, want 256", result["size"])
	}
	if result["code"] != 1000 {
		t.Errorf("code = %v, want 1000", result["code"])
	}
	if result["format_confidence"] != 0.95 {
		t.Errorf("format_confidence = %v, want 0.95", result["format_confidence"])
	}
	if result["tab_id"] != 7 {
		t.Errorf("tab_id = %v, want 7", result["tab_id"])
	}
	gotSampled, ok := result["sampled"].(*types.SamplingInfo)
	if !ok {
		t.Fatalf("sampled is not *types.SamplingInfo, got %T", result["sampled"])
	}
	if gotSampled.Rate != "1/10" {
		t.Errorf("sampled.Rate = %q, want %q", gotSampled.Rate, "1/10")
	}
}

func TestSerializeWebSocketEntryWithSequence_NoOptionalFields(t *testing.T) {
	t.Parallel()
	event := WebSocketEntryWithSequence{
		Entry: types.WebSocketEvent{
			Event: "open",
			ID:    "ws-1",
		},
		Sequence:  1,
		Timestamp: "2026-01-30T10:15:23Z",
	}
	result := SerializeWebSocketEntryWithSequence(event)

	// These keys should NOT be present when empty/zero
	absent := []string{"type", "url", "direction", "data", "reason",
		"binary_format", "size", "code", "format_confidence", "sampled", "tab_id"}
	for _, key := range absent {
		if _, exists := result[key]; exists {
			t.Errorf("key %q should not be present when empty/zero, got %v", key, result[key])
		}
	}

	// Required fields should always be present
	if result["event"] != "open" {
		t.Errorf("event = %v, want 'open'", result["event"])
	}
	if result["id"] != "ws-1" {
		t.Errorf("id = %v, want 'ws-1'", result["id"])
	}
	if result["sequence"] != int64(1) {
		t.Errorf("sequence = %v, want 1", result["sequence"])
	}
}
