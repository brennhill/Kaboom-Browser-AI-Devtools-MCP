// Purpose: Adapts generic cursor pagination to console/network log entries.
// Why: Keeps log field extraction/serialization separate from generic pagination behavior.
// Docs: docs/features/feature/pagination/index.md

package pagination

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// entryStr extracts a string field from a LogEntry map.
// Returns empty string if field doesn't exist or isn't a string.
func entryStr(entry types.LogEntry, key string) string {
	if v, ok := entry[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ============================================
// Log Pagination
// ============================================

// LogEntryWithSequence pairs a log entry with its sequence number and timestamp for pagination.
type LogEntryWithSequence struct {
	Entry     types.LogEntry
	Sequence  int64
	Timestamp string
}

// GetSequence implements Sequenced.
func (e LogEntryWithSequence) GetSequence() int64 { return e.Sequence }

// GetTimestamp implements Sequenced.
func (e LogEntryWithSequence) GetTimestamp() string { return e.Timestamp }

// EnrichLogEntries adds sequence numbers and timestamps to entries for pagination.
// Must be called with the UNFILTERED entry list to get correct sequence numbers.
func EnrichLogEntries(entries []types.LogEntry, logTotalAdded int64) []LogEntryWithSequence {
	enriched := make([]LogEntryWithSequence, len(entries))
	baseSeq := logTotalAdded - int64(len(entries)) + 1

	for i, entry := range entries {
		enriched[i] = LogEntryWithSequence{
			Entry:     entry,
			Sequence:  baseSeq + int64(i),
			Timestamp: entryStr(entry, "ts"),
		}
	}

	return enriched
}

// ApplyLogCursorPagination applies cursor-based pagination to log entries with sequence metadata.
// Returns filtered entries, cursor metadata, and any error.
func ApplyLogCursorPagination(
	enrichedEntries []LogEntryWithSequence,
	afterCursor, beforeCursor, sinceCursor string,
	limit int,
	restartOnEviction bool,
) ([]LogEntryWithSequence, *CursorPaginationMetadata, error) {
	return ApplyCursorPagination(enrichedEntries, CursorParams{
		AfterCursor:       afterCursor,
		BeforeCursor:      beforeCursor,
		SinceCursor:       sinceCursor,
		Limit:             limit,
		RestartOnEviction: restartOnEviction,
	})
}
