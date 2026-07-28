// Purpose: Adapts generic cursor pagination to captured user action entries.
// Why: Keeps action timestamp normalization and serialization separate from generic cursor logic.
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
