// Purpose: Benchmark pagination and cursor throughput and latency.
// Docs: docs/features/feature/pagination/index.md

package pagination

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"testing"
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
