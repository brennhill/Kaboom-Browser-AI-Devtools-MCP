// Purpose: Adapts generic pagination to log, action, and WebSocket entries.
// Why: All entry-family enrichment and serialization changes with the telemetry wire model.
// Docs: docs/features/feature/pagination/index.md

package pagination

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// Action Pagination
// ============================================

// ActionEntryWithSequence pairs an action entry with its sequence number and timestamp for pagination.
type ActionEntryWithSequence struct {
	Entry     types.EnhancedAction
	Sequence  int64
	Timestamp string // RFC3339 normalized timestamp
}

// GetSequence implements Sequenced.
func (e ActionEntryWithSequence) GetSequence() int64 { return e.Sequence }

// GetTimestamp implements Sequenced.
func (e ActionEntryWithSequence) GetTimestamp() string { return e.Timestamp }

// EnrichActionEntries adds sequence numbers and normalized timestamps to action entries for pagination.
// Must be called with the UNFILTERED entry list to get correct sequence numbers.
func EnrichActionEntries(actions []types.EnhancedAction, actionTotalAdded int64) []ActionEntryWithSequence {
	enriched := make([]ActionEntryWithSequence, len(actions))
	baseSeq := actionTotalAdded - int64(len(actions)) + 1

	for i, action := range actions {
		enriched[i] = ActionEntryWithSequence{
			Entry:     action,
			Sequence:  baseSeq + int64(i),
			Timestamp: NormalizeTimestamp(action.Timestamp),
		}
	}

	return enriched
}

// ApplyActionCursorPagination applies cursor-based pagination to action entries with sequence metadata.
// Returns filtered entries, cursor metadata, and any error.
func ApplyActionCursorPagination(
	enrichedEntries []ActionEntryWithSequence,
	afterCursor, beforeCursor, sinceCursor string,
	limit int,
	restartOnEviction bool,
) ([]ActionEntryWithSequence, *CursorPaginationMetadata, error) {
	return ApplyCursorPagination(enrichedEntries, CursorParams{
		AfterCursor:       afterCursor,
		BeforeCursor:      beforeCursor,
		SinceCursor:       sinceCursor,
		Limit:             limit,
		RestartOnEviction: restartOnEviction,
	})
}

// SerializeActionEntryWithSequence converts an ActionEntryWithSequence to a JSON-serializable map.
func SerializeActionEntryWithSequence(enriched ActionEntryWithSequence) map[string]any {
	result := map[string]any{
		"type":      enriched.Entry.Type,
		"timestamp": enriched.Timestamp,
		"sequence":  enriched.Sequence,
	}

	addNonEmpty(result, "url", enriched.Entry.URL)
	if len(enriched.Entry.Selectors) > 0 {
		result["selectors"] = enriched.Entry.Selectors
	}
	addNonEmpty(result, "value", enriched.Entry.Value)
	addNonEmpty(result, "input_type", enriched.Entry.InputType)
	addNonEmpty(result, "key", enriched.Entry.Key)
	addNonEmpty(result, "from_url", enriched.Entry.FromURL)
	addNonEmpty(result, "to_url", enriched.Entry.ToURL)
	addNonEmpty(result, "selected_value", enriched.Entry.SelectedValue)
	addNonEmpty(result, "selected_text", enriched.Entry.SelectedText)

	if enriched.Entry.ScrollY != 0 {
		result["scroll_y"] = enriched.Entry.ScrollY
	}
	if enriched.Entry.TabID > 0 {
		result["tab_id"] = enriched.Entry.TabID
	}

	return result
}

func entryStr(entry types.LogEntry, key string) string {
	value, ok := entry[key]
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return text
}

type LogEntryWithSequence struct {
	Entry     types.LogEntry
	Sequence  int64
	Timestamp string
}

func (entry LogEntryWithSequence) GetSequence() int64   { return entry.Sequence }
func (entry LogEntryWithSequence) GetTimestamp() string { return entry.Timestamp }

func EnrichLogEntries(entries []types.LogEntry, totalAdded int64) []LogEntryWithSequence {
	enriched := make([]LogEntryWithSequence, len(entries))
	baseSequence := totalAdded - int64(len(entries)) + 1
	for index, entry := range entries {
		enriched[index] = LogEntryWithSequence{Entry: entry, Sequence: baseSequence + int64(index), Timestamp: entryStr(entry, "ts")}
	}
	return enriched
}

func ApplyLogCursorPagination(entries []LogEntryWithSequence, after, before, since string, limit int, restart bool) ([]LogEntryWithSequence, *CursorPaginationMetadata, error) {
	return ApplyCursorPagination(entries, CursorParams{AfterCursor: after, BeforeCursor: before, SinceCursor: since, Limit: limit, RestartOnEviction: restart})
}

type WebSocketEntryWithSequence struct {
	Entry     types.WebSocketEvent
	Sequence  int64
	Timestamp string
}

func (entry WebSocketEntryWithSequence) GetSequence() int64   { return entry.Sequence }
func (entry WebSocketEntryWithSequence) GetTimestamp() string { return entry.Timestamp }

func EnrichWebSocketEntries(events []types.WebSocketEvent, totalAdded int64) []WebSocketEntryWithSequence {
	enriched := make([]WebSocketEntryWithSequence, len(events))
	baseSequence := totalAdded - int64(len(events)) + 1
	for index, event := range events {
		enriched[index] = WebSocketEntryWithSequence{Entry: event, Sequence: baseSequence + int64(index), Timestamp: event.Timestamp}
	}
	return enriched
}

func ApplyWebSocketCursorPagination(entries []WebSocketEntryWithSequence, after, before, since string, limit int, restart bool) ([]WebSocketEntryWithSequence, *CursorPaginationMetadata, error) {
	return ApplyCursorPagination(entries, CursorParams{AfterCursor: after, BeforeCursor: before, SinceCursor: since, Limit: limit, RestartOnEviction: restart})
}

func SerializeWebSocketEntryWithSequence(enriched WebSocketEntryWithSequence) map[string]any {
	result := map[string]any{"event": enriched.Entry.Event, "id": enriched.Entry.ID, "timestamp": enriched.Timestamp, "sequence": enriched.Sequence}
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
