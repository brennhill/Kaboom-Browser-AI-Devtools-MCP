// Purpose: Implements generic cursor pagination over sequenced telemetry collections.
// Why: Keeps shared pagination rules centralized while adapters handle domain-specific fields.
// Docs: docs/features/feature/pagination/index.md

package pagination

import (
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// Type aliases for imported types to avoid qualifying every use.
type (
	// EnhancedAction is a user action from the capture package.
	EnhancedAction = types.EnhancedAction
	// WebSocketEvent is a WebSocket event from the capture package.
	WebSocketEvent = types.WebSocketEvent
)

// ============================================
// Sequenced is the interface for entries with pagination metadata.
// ============================================

// Sequenced provides access to sequence and timestamp for cursor pagination.
type Sequenced interface {
	GetSequence() int64
	GetTimestamp() string
}

// CursorParams bundles cursor pagination parameters.
type CursorParams struct {
	AfterCursor       string
	BeforeCursor      string
	SinceCursor       string
	Limit             int
	RestartOnEviction bool
}

// resolveCursorType determines which cursor string and type to use.
func resolveCursorType(after, before, since string) (string, string) {
	if after != "" {
		return after, "after"
	}
	if before != "" {
		return before, "before"
	}
	if since != "" {
		return since, "since"
	}
	return "", ""
}

// checkCursorExpired checks if the cursor has expired due to buffer overflow.
// Returns true if cursor expired and was handled (restart or error).
func checkCursorExpired[T Sequenced](
	entries []T, cursor Cursor, cursorStr string,
	restartOnEviction bool, metadata *CursorPaginationMetadata,
) error {
	if len(entries) == 0 || cursor.Sequence <= 0 {
		return nil
	}
	oldestSeq := entries[0].GetSequence()
	if cursor.Sequence >= oldestSeq {
		return nil
	}
	if restartOnEviction {
		metadata.CursorRestarted = true
		metadata.OriginalCursor = cursorStr
		metadata.Warning = fmt.Sprintf("Cursor expired (buffer overflow). Restarted from oldest available entry. Lost entries: %d to %d",
			cursor.Sequence, oldestSeq-1)
		return nil
	}
	return fmt.Errorf("cursor expired (buffer overflow). Requested sequence %d, oldest available is %d. Lost %d entries",
		cursor.Sequence, oldestSeq, oldestSeq-cursor.Sequence)
}

// filterByCursor filters entries using the cursor comparison for the given cursor type.
func filterByCursor[T Sequenced](entries []T, cursor Cursor, cursorType string) []T {
	var filtered []T
	for _, entry := range entries {
		if matchesCursorType(cursor, cursorType, entry.GetTimestamp(), entry.GetSequence()) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// matchesCursorType returns true if an entry matches the cursor filter for the given type.
func matchesCursorType(cursor Cursor, cursorType, ts string, seq int64) bool {
	switch cursorType {
	case "after":
		return cursor.IsOlder(ts, seq)
	case "before":
		return cursor.IsNewer(ts, seq)
	case "since":
		return cursor.IsNewer(ts, seq) || (ts == cursor.Timestamp && seq == cursor.Sequence)
	default:
		return false
	}
}

// applyLimit trims entries to limit, respecting pagination direction.
func applyLimit[T Sequenced](entries []T, limit int, forwardPagination bool) []T {
	if limit <= 0 || limit >= len(entries) {
		return entries
	}
	if forwardPagination {
		return entries[:limit]
	}
	return entries[len(entries)-limit:]
}

// buildMetadata populates pagination metadata from the result set.
// The continuation cursor follows the walk direction: backward (after_cursor)
// walks continue from the oldest returned entry, forward walks from the newest.
// Building the after-walk cursor from the newest entry would make every
// subsequent page overlap the previous one.
func buildMetadata[T Sequenced](entries []T, backwardWalk bool, countBeforeLimit int, metadata *CursorPaginationMetadata) {
	metadata.Count = len(entries)
	if len(entries) == 0 {
		return
	}
	metadata.OldestTimestamp = entries[0].GetTimestamp()
	metadata.NewestTimestamp = entries[len(entries)-1].GetTimestamp()
	cursorEntry := entries[len(entries)-1]
	if backwardWalk {
		cursorEntry = entries[0]
	}
	metadata.Cursor = BuildCursor(cursorEntry.GetTimestamp(), cursorEntry.GetSequence())
	if countBeforeLimit > len(entries) {
		metadata.HasMore = true
	}
}

// ApplyCursorPagination is the generic cursor pagination implementation.
// Works for any Sequenced type (logs, actions, websocket events).
func ApplyCursorPagination[T Sequenced](entries []T, p CursorParams) ([]T, *CursorPaginationMetadata, error) {
	metadata := &CursorPaginationMetadata{Total: len(entries)}

	cursorStr, cursorType := resolveCursorType(p.AfterCursor, p.BeforeCursor, p.SinceCursor)

	if cursorStr == "" {
		// Default view: newest entries, forward continuation cursor.
		countBeforeLimit := len(entries)
		entries = applyLimit(entries, p.Limit, false)
		buildMetadata(entries, false, countBeforeLimit, metadata)
		return entries, metadata, nil
	}

	cursor, err := ParseCursor(cursorStr)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid cursor format: %w", err)
	}

	if err := checkCursorExpired(entries, cursor, cursorStr, p.RestartOnEviction, metadata); err != nil {
		return nil, nil, err
	}

	if !metadata.CursorRestarted {
		entries = filterByCursor(entries, cursor, cursorType)
	}

	countBeforeLimit := len(entries)
	// after_cursor walks backward through older history: limit keeps the
	// entries closest to the cursor (the tail) and the continuation cursor is
	// the oldest returned entry. before/since cursors — and after-walks that
	// restarted from the oldest entry after eviction — paginate forward.
	backwardWalk := cursorType == "after" && !metadata.CursorRestarted
	entries = applyLimit(entries, p.Limit, !backwardWalk)
	buildMetadata(entries, backwardWalk, countBeforeLimit, metadata)
	return entries, metadata, nil
}

// addNonEmpty adds a key-value pair to the map only if the string value is non-empty.
func addNonEmpty(m map[string]any, key, value string) {
	if value != "" {
		m[key] = value
	}
}
