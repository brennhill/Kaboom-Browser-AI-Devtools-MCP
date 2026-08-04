// Purpose: Implements cursor pagination over captured telemetry collections.
// Why: Makes large evidence sets queryable without losing ordering or cursor consistency.
// Docs: docs/features/feature/pagination/index.md

package pagination

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Cursor represents a pagination cursor combining timestamp and sequence for stable iteration.
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

// IsOlder returns true if this entry is older than the cursor (for backward pagination).
// Compares timestamp first, then sequence as tiebreaker for same-millisecond entries.
// For sequence-only cursors (no timestamp), compares by sequence number alone.
func (c Cursor) IsOlder(entryTimestamp string, entrySequence int64) bool {
	// Sequence-only cursor: compare by sequence number
	if c.Timestamp == "" {
		return entrySequence < c.Sequence
	}

	// Parse timestamps for comparison
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

	// Compare timestamps first
	if entryTime.Before(cursorTime) {
		return true // Entry is older by timestamp
	}
	if entryTime.After(cursorTime) {
		return false // Entry is newer by timestamp
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

	cursorTime, err := time.Parse(time.RFC3339Nano, c.Timestamp)
	if err != nil {
		cursorTime, _ = time.Parse(time.RFC3339, c.Timestamp)
	}

	entryTime, err := time.Parse(time.RFC3339Nano, entryTimestamp)
	if err != nil {
		entryTime, _ = time.Parse(time.RFC3339, entryTimestamp)
	}

	// Compare timestamps first
	if entryTime.After(cursorTime) {
		return true // Entry is newer by timestamp
	}
	if entryTime.Before(cursorTime) {
		return false // Entry is older by timestamp
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
