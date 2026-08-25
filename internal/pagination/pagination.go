// Purpose: Implements the cursor pagination package and generic slicing engine.
// Why: Keeps shared pagination rules centralized while adapters handle domain-specific fields.
// Docs: docs/features/feature/pagination/index.md

// Package pagination provides cursor-based slicing for bounded telemetry streams.
package pagination

import (
	"fmt"
	"strconv"
	"strings"
	"time"
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

type Cursor struct {
	Timestamp string // RFC3339 timestamp
	Sequence  int64  // Monotonic sequence number (tiebreaker for same-millisecond entries)
}

// ParseCursor parses a composite cursor string "timestamp:sequence" or ":sequence" into a Cursor struct.
// Supports sequence-only cursors (":N") for logs without timestamps.
// Returns zero cursor if input is empty (for first page request).
// Returns error if cursor format is invalid.
func ParseCursor(cursorStr string) (Cursor, error) {
	if cursorStr == "" {
		return Cursor{}, nil // Empty cursor = start from beginning
	}

	// Find the last colon (since RFC3339 timestamps contain colons)
	lastColonIdx := strings.LastIndex(cursorStr, ":")
	if lastColonIdx == -1 {
		return Cursor{}, fmt.Errorf("invalid cursor format: expected 'timestamp:sequence' or ':sequence', got '%s'", cursorStr)
	}

	// Split on last colon: everything before is timestamp, everything after is sequence
	timestamp := cursorStr[:lastColonIdx]
	sequenceStr := cursorStr[lastColonIdx+1:]

	// Validate timestamp format (RFC3339) if present
	if timestamp != "" {
		_, err := time.Parse(time.RFC3339, timestamp)
		if err != nil {
			// Try with nanosecond precision
			_, err = time.Parse(time.RFC3339Nano, timestamp)
			if err != nil {
				return Cursor{}, fmt.Errorf("invalid timestamp in cursor: %w", err)
			}
		}
	}

	// Parse sequence number
	sequence, err := strconv.ParseInt(sequenceStr, 10, 64)
	if err != nil {
		return Cursor{}, fmt.Errorf("invalid sequence in cursor: %w", err)
	}

	return Cursor{
		Timestamp: timestamp,
		Sequence:  sequence,
	}, nil
}

// BuildCursor creates a composite cursor string from timestamp and sequence.
// Returns sequence-only cursor (":N") when timestamp is unavailable.
func BuildCursor(timestamp string, sequence int64) string {
	if timestamp == "" {
		// Return sequence-only cursor for logs without timestamps
		return ":" + strconv.FormatInt(sequence, 10)
	}
	return timestamp + ":" + strconv.FormatInt(sequence, 10)
}

// compareTimestamp parses cursor and entry timestamps (RFC3339Nano with
// RFC3339 fallback) and returns -1 when the entry precedes the cursor,
// 1 when it follows, and 0 when they are equal.
func (c Cursor) compareTimestamp(entryTimestamp string) int {
	cursorTime, err := time.Parse(time.RFC3339Nano, c.Timestamp)
	if err != nil {
		// Fallback to RFC3339 (millisecond precision)
		cursorTime, _ = time.Parse(time.RFC3339, c.Timestamp)
	}

	entryTime, err := time.Parse(time.RFC3339Nano, entryTimestamp)
	if err != nil {
		// Fallback to RFC3339
		entryTime, _ = time.Parse(time.RFC3339, entryTimestamp)
	}

	switch {
	case entryTime.Before(cursorTime):
		return -1
	case entryTime.After(cursorTime):
		return 1
	default:
		return 0
	}
}

// IsOlder returns true if this entry is older than the cursor (for backward pagination).
// Compares timestamp first, then sequence as tiebreaker for same-millisecond entries.
// For sequence-only cursors (no timestamp), compares by sequence number alone.
func (c Cursor) IsOlder(entryTimestamp string, entrySequence int64) bool {
	// Sequence-only cursor: compare by sequence number
	if c.Timestamp == "" {
		return entrySequence < c.Sequence
	}

	if cmp := c.compareTimestamp(entryTimestamp); cmp != 0 {
		return cmp < 0
	}

	// Timestamps match - use sequence as tiebreaker
	return entrySequence < c.Sequence
}

// IsNewer returns true if this entry is newer than the cursor (for forward pagination).
// For sequence-only cursors (no timestamp), compares by sequence number alone.
func (c Cursor) IsNewer(entryTimestamp string, entrySequence int64) bool {
	// Sequence-only cursor: compare by sequence number
	if c.Timestamp == "" {
		return entrySequence > c.Sequence
	}

	if cmp := c.compareTimestamp(entryTimestamp); cmp != 0 {
		return cmp > 0
	}

	// Timestamps match - use sequence as tiebreaker
	return entrySequence > c.Sequence
}

// NormalizeTimestamp converts various timestamp formats to RFC3339 string.
// Handles: int64 (Unix milliseconds), time.Time, string (passthrough).
func NormalizeTimestamp(ts any) string {
	switch v := ts.(type) {
	case string:
		// Already a string, assume RFC3339 format
		return v
	case int64:
		// Unix milliseconds → RFC3339
		return time.UnixMilli(v).UTC().Format(time.RFC3339)
	case time.Time:
		// Go time.Time → RFC3339
		return v.UTC().Format(time.RFC3339)
	default:
		// Unknown type, return empty
		return ""
	}
}

// CursorPaginationMetadata contains metadata for cursor-based pagination responses.
type CursorPaginationMetadata struct {
	Cursor          string `json:"cursor,omitempty"`           // Composite cursor of last returned entry
	Count           int    `json:"count"`                      // Number of entries in this page
	HasMore         bool   `json:"has_more"`                   // More entries available
	OldestTimestamp string `json:"oldest_timestamp,omitempty"` // Oldest entry in buffer
	NewestTimestamp string `json:"newest_timestamp,omitempty"` // Newest entry in buffer
	Total           int    `json:"total"`                      // Total entries in buffer
	CursorRestarted bool   `json:"cursor_restarted,omitempty"` // True if cursor expired and auto-restarted
	OriginalCursor  string `json:"original_cursor,omitempty"`  // Original cursor if restarted
	Warning         string `json:"warning,omitempty"`          // Warning message if applicable
}
