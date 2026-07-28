// Purpose: Adapts generic cursor pagination to websocket event streams.
// Why: Keeps websocket-specific enrichment and serialization isolated from generic pagination rules.
// Docs: docs/features/feature/pagination/index.md

package pagination

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// WebSocket Pagination
// ============================================

// WebSocketEntryWithSequence pairs a websocket event with its sequence number and timestamp for pagination.
type WebSocketEntryWithSequence struct {
	Entry     types.WebSocketEvent
	Sequence  int64
	Timestamp string // RFC3339 timestamp (already string in types.WebSocketEvent)
}

// GetSequence implements Sequenced.
func (e WebSocketEntryWithSequence) GetSequence() int64 { return e.Sequence }

// GetTimestamp implements Sequenced.
func (e WebSocketEntryWithSequence) GetTimestamp() string { return e.Timestamp }

// EnrichWebSocketEntries adds sequence numbers to websocket events for pagination.
// Must be called with the UNFILTERED entry list to get correct sequence numbers.
func EnrichWebSocketEntries(events []types.WebSocketEvent, wsTotalAdded int64) []WebSocketEntryWithSequence {
	enriched := make([]WebSocketEntryWithSequence, len(events))
	baseSeq := wsTotalAdded - int64(len(events)) + 1

	for i, event := range events {
		enriched[i] = WebSocketEntryWithSequence{
			Entry:     event,
			Sequence:  baseSeq + int64(i),
			Timestamp: event.Timestamp,
		}
	}

	return enriched
}

// ApplyWebSocketCursorPagination applies cursor-based pagination to websocket events with sequence metadata.
// Returns filtered entries, cursor metadata, and any error.
func ApplyWebSocketCursorPagination(
	enrichedEntries []WebSocketEntryWithSequence,
	afterCursor, beforeCursor, sinceCursor string,
	limit int,
	restartOnEviction bool,
) ([]WebSocketEntryWithSequence, *CursorPaginationMetadata, error) {
	return ApplyCursorPagination(enrichedEntries, CursorParams{
		AfterCursor:       afterCursor,
		BeforeCursor:      beforeCursor,
		SinceCursor:       sinceCursor,
		Limit:             limit,
		RestartOnEviction: restartOnEviction,
	})
}

// SerializeWebSocketEntryWithSequence converts a WebSocketEntryWithSequence to a JSON-serializable map.
func SerializeWebSocketEntryWithSequence(enriched WebSocketEntryWithSequence) map[string]any {
	result := map[string]any{
		"event":     enriched.Entry.Event,
		"id":        enriched.Entry.ID,
		"timestamp": enriched.Timestamp,
		"sequence":  enriched.Sequence,
	}

	addNonEmpty(result, "type", enriched.Entry.Type)
	addNonEmpty(result, "url", enriched.Entry.URL)
	addNonEmpty(result, "direction", enriched.Entry.Direction)
	addNonEmpty(result, "data", enriched.Entry.Data)
	addNonEmpty(result, "reason", enriched.Entry.CloseReason)
	addNonEmpty(result, "binary_format", enriched.Entry.BinaryFormat)

	if enriched.Entry.Size > 0 {
		result["size"] = enriched.Entry.Size
	}
	if enriched.Entry.CloseCode > 0 {
		result["code"] = enriched.Entry.CloseCode
	}
	if enriched.Entry.FormatConfidence > 0 {
		result["format_confidence"] = enriched.Entry.FormatConfidence
	}
	if enriched.Entry.Sampled != nil {
		result["sampled"] = enriched.Entry.Sampled
	}
	if enriched.Entry.TabID > 0 {
		result["tab_id"] = enriched.Entry.TabID
	}

	return result
}
