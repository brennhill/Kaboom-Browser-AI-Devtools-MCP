// Purpose: Tests for WebSocket event pagination.
// Docs: docs/features/feature/pagination/index.md

// pagination_websocket_test.go — Unit tests for websocket event cursor-based pagination
package pagination

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestEnrichWebSocketEntries(t *testing.T) {
	tests := []struct {
		name             string
		events           []types.WebSocketEvent
		wsTotalAdded     int64
		expectedFirstSeq int64
		expectedLastSeq  int64
		expectedCount    int
	}{
		{
			name:             "empty buffer",
			events:           []types.WebSocketEvent{},
			wsTotalAdded:     0,
			expectedFirstSeq: 0,
			expectedLastSeq:  0,
			expectedCount:    0,
		},
		{
			name: "single event",
			events: []types.WebSocketEvent{
				{Event: "message", ID: "ws-1", URL: "wss://echo.example.com", Timestamp: "2026-01-30T10:15:23Z"},
			},
			wsTotalAdded:     1,
			expectedFirstSeq: 1,
			expectedLastSeq:  1,
			expectedCount:    1,
		},
		{
			name: "multiple events no eviction",
			events: []types.WebSocketEvent{
				{Event: "open", ID: "ws-1", URL: "wss://echo.example.com", Timestamp: "2026-01-30T10:15:20Z"},
				{Event: "message", ID: "ws-1", URL: "wss://echo.example.com", Timestamp: "2026-01-30T10:15:21Z"},
				{Event: "close", ID: "ws-1", URL: "wss://echo.example.com", Timestamp: "2026-01-30T10:15:22Z"},
			},
			wsTotalAdded:     3,
			expectedFirstSeq: 1,
			expectedLastSeq:  3,
			expectedCount:    3,
		},
		{
			name: "buffer with evictions (wsTotalAdded > len)",
			events: []types.WebSocketEvent{
				{Event: "message", ID: "ws-2", URL: "wss://echo.example.com", Timestamp: "2026-01-30T10:20:00Z"},
				{Event: "close", ID: "ws-2", URL: "wss://echo.example.com", Timestamp: "2026-01-30T10:20:01Z"},
			},
			wsTotalAdded:     152, // 150 evicted, 2 remain
			expectedFirstSeq: 151, // First entry is sequence 151
			expectedLastSeq:  152, // Last entry is sequence 152
			expectedCount:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enriched := EnrichWebSocketEntries(tt.events, tt.wsTotalAdded)

			assertEnrichedEntryRange(
				t,
				enriched,
				tt.expectedCount,
				tt.expectedFirstSeq,
				tt.expectedLastSeq,
				func(entry WebSocketEntryWithSequence) int64 {
					return entry.Sequence
				},
			)

			if len(enriched) > 0 {
				// Verify timestamps are preserved (already RFC3339 strings)
				for i, e := range enriched {
					if e.Timestamp == "" && tt.events[i].Timestamp != "" {
						t.Errorf("Entry %d: timestamp was lost", i)
					}
				}
			}
		})
	}
}

func TestApplyWebSocketCursorPagination_NoCursor(t *testing.T) {
	// Create 100 events
	events := make([]types.WebSocketEvent, 100)
	for i := 0; i < 100; i++ {
		events[i] = types.WebSocketEvent{
			Event:     "message",
			ID:        "ws-1",
			URL:       "wss://echo.example.com",
			Timestamp: "2026-01-30T10:15:00Z", // Same timestamp for all (batched)
		}
	}
	enriched := EnrichWebSocketEntries(events, 100)

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
		func(afterCursor, beforeCursor string, limit int, restartOnEviction bool) ([]WebSocketEntryWithSequence, *CursorPaginationMetadata, error) {
			return ApplyWebSocketCursorPagination(enriched, afterCursor, beforeCursor, "", limit, restartOnEviction)
		},
		func(entry WebSocketEntryWithSequence) int64 {
			return entry.Sequence
		},
		func(entry WebSocketEntryWithSequence) string {
			return entry.Timestamp
		},
	)
}

func TestApplyWebSocketCursorPagination_AfterCursor(t *testing.T) {
	// Create 100 events (sequences 1-100)
	events := make([]types.WebSocketEvent, 100)
	for i := 0; i < 100; i++ {
		events[i] = types.WebSocketEvent{
			Event:     "message",
			ID:        "ws-1",
			URL:       "wss://echo.example.com",
			Timestamp: "2026-01-30T10:15:00Z",
		}
	}
	enriched := EnrichWebSocketEntries(events, 100)

	cursors := buildPaginationCursorSet(
		enriched,
		func(entry WebSocketEntryWithSequence) string { return entry.Timestamp },
		func(entry WebSocketEntryWithSequence) int64 { return entry.Sequence },
	)

	runAfterCursorPaginationCases(
		t,
		len(enriched),
		standardAfterCursorCases(cursors),
		func(afterCursor, beforeCursor string, limit int, restartOnEviction bool) ([]WebSocketEntryWithSequence, *CursorPaginationMetadata, error) {
			return ApplyWebSocketCursorPagination(enriched, afterCursor, beforeCursor, "", limit, restartOnEviction)
		},
		func(entry WebSocketEntryWithSequence) int64 {
			return entry.Sequence
		},
		func(entry WebSocketEntryWithSequence) string {
			return entry.Timestamp
		},
	)
}

func TestApplyWebSocketCursorPagination_CursorExpired(t *testing.T) {
	// Buffer has sequences 101-200 (100 entries evicted)
	events := make([]types.WebSocketEvent, 100)
	for i := 0; i < 100; i++ {
		events[i] = types.WebSocketEvent{
			Event:     "message",
			ID:        "ws-1",
			URL:       "wss://echo.example.com",
			Timestamp: "2026-01-30T10:20:00Z",
		}
	}
	enriched := EnrichWebSocketEntries(events, 200) // 200 total added, 100 evicted

	// Build a cursor for an evicted sequence (sequence 50, which is before sequence 101)
	expiredCursor := BuildCursor("2026-01-30T10:15:00Z", 50)

	runCursorExpiredPaginationCases(
		t,
		len(enriched),
		[]paginationCursorExpiredCase{
			{
				name:              "expired cursor without restart returns error",
				afterCursor:       expiredCursor,
				limit:             0,
				restartOnEviction: false,
				expectError:       true,
			},
			{
				name:                  "expired cursor with restart returns oldest available",
				afterCursor:           expiredCursor,
				limit:                 0,
				restartOnEviction:     true,
				expectError:           false,
				expectedCount:         100, // All 100 available entries (no limit)
				expectedFirstSeq:      101, // Oldest available is sequence 101
				expectedCursorRestart: true,
			},
			{
				name:                  "expired cursor with restart and limit",
				afterCursor:           expiredCursor,
				limit:                 10, // Limit applied
				restartOnEviction:     true,
				expectError:           false,
				expectedCount:         10,
				expectedFirstSeq:      101, // After restart, take FIRST 10 entries from oldest
				expectedCursorRestart: true,
			},
		},
		func(afterCursor, beforeCursor string, limit int, restartOnEviction bool) ([]WebSocketEntryWithSequence, *CursorPaginationMetadata, error) {
			return ApplyWebSocketCursorPagination(enriched, afterCursor, beforeCursor, "", limit, restartOnEviction)
		},
		func(entry WebSocketEntryWithSequence) int64 {
			return entry.Sequence
		},
		func(entry WebSocketEntryWithSequence) string {
			return entry.Timestamp
		},
	)
}

func TestSerializeWebSocketEntryWithSequence(t *testing.T) {
	event := WebSocketEntryWithSequence{
		Entry: types.WebSocketEvent{
			Event:     "message",
			ID:        "ws-1",
			URL:       "wss://echo.example.com",
			Direction: "incoming",
			Data:      `{"type":"ping"}`,
			Timestamp: "2026-01-30T10:15:23Z",
			TabID:     123,
		},
		Sequence:  5678,
		Timestamp: "2026-01-30T10:15:23Z",
	}

	result := SerializeWebSocketEntryWithSequence(event)

	// Verify required fields
	if result["event"] != "message" {
		t.Errorf("event = %v, want 'message'", result["event"])
	}

	if result["id"] != "ws-1" {
		t.Errorf("id = %v, want 'ws-1'", result["id"])
	}

	if result["url"] != "wss://echo.example.com" {
		t.Errorf("url = %v, want 'wss://echo.example.com'", result["url"])
	}

	if result["timestamp"] != "2026-01-30T10:15:23Z" {
		t.Errorf("timestamp = %v, want '2026-01-30T10:15:23Z'", result["timestamp"])
	}

	if result["sequence"] != int64(5678) {
		t.Errorf("sequence = %v, want 5678", result["sequence"])
	}

	if result["direction"] != "incoming" {
		t.Errorf("direction = %v, want 'incoming'", result["direction"])
	}

	// Verify tabId included
	if result["tab_id"] != 123 {
		t.Errorf("tab_id = %v, want 123", result["tab_id"])
	}
}
