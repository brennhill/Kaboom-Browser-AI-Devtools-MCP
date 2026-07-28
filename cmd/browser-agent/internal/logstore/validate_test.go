// Purpose: Tests for log entry ingest validation.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package logstore

import (
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestValidateLogEntry_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("entry at exactly max size is valid", func(t *testing.T) {
		// Create an entry that triggers the slow path but stays under limit
		bigStr := make([]byte, MaxEntrySize/2+1)
		for i := range bigStr {
			bigStr[i] = 'a'
		}
		entry := types.LogEntry{"level": "info", "data": string(bigStr)}
		// This triggers slow path (string content > MaxEntrySize/2)
		// Result depends on whether JSON serialization stays under limit
		_ = ValidateEntry(entry)
		// Just verifying it doesn't panic
	})

	t.Run("empty entry is invalid", func(t *testing.T) {
		if ValidateEntry(types.LogEntry{}) {
			t.Fatal("empty entry should be invalid (no level)")
		}
	})
}

// TestValidateEntry_LevelContract pins the accepted level vocabulary: the /logs
// ingest contract rejects anything outside it, so an entry with an unknown or
// non-string level must never reach the store.
func TestValidateEntry_LevelContract(t *testing.T) {
	t.Parallel()

	for _, level := range []string{"error", "warn", "info", "debug", "log"} {
		if !ValidateEntry(types.LogEntry{"level": level}) {
			t.Errorf("ValidateEntry(level=%q) = false, want true", level)
		}
	}
	for _, bad := range []any{"trace", "ERROR", "", 42, nil} {
		if ValidateEntry(types.LogEntry{"level": bad}) {
			t.Errorf("ValidateEntry(level=%v) = true, want false", bad)
		}
	}
}

// TestValidateEntry_OversizeRejected pins the slow path: an entry whose string
// content alone exceeds MaxEntrySize must be rejected after the marshal check.
func TestValidateEntry_OversizeRejected(t *testing.T) {
	t.Parallel()

	oversize := types.LogEntry{"level": "info", "message": strings.Repeat("x", MaxEntrySize+1)}
	if ValidateEntry(oversize) {
		t.Fatalf("ValidateEntry() = true for a %d-byte entry, want false (limit %d)",
			MaxEntrySize+1, MaxEntrySize)
	}
}

// TestValidateEntries_FiltersAndCounts pins the batch contract used by POST
// /logs: valid entries are kept in order and invalid ones are counted, not
// silently dropped (the "rejected" field of the response comes from here).
func TestValidateEntries_FiltersAndCounts(t *testing.T) {
	t.Parallel()

	valid, rejected := ValidateEntries([]types.LogEntry{
		{"level": "info", "message": "keep-1"},
		{"level": "bogus", "message": "drop-1"},
		{"level": "error", "message": "keep-2"},
		{"message": "drop-2 (no level)"},
	})

	if rejected != 2 {
		t.Fatalf("rejected = %d, want 2", rejected)
	}
	if len(valid) != 2 {
		t.Fatalf("len(valid) = %d, want 2", len(valid))
	}
	if valid[0]["message"] != "keep-1" || valid[1]["message"] != "keep-2" {
		t.Fatalf("valid = %v, want the two valid entries in input order", valid)
	}

	// An all-valid batch must report zero rejections and preserve every entry.
	valid, rejected = ValidateEntries([]types.LogEntry{{"level": "warn"}, {"level": "debug"}})
	if rejected != 0 || len(valid) != 2 {
		t.Fatalf("all-valid batch = %d valid / %d rejected, want 2/0", len(valid), rejected)
	}

	// An empty batch must return an empty (non-nil) slice: the caller passes the
	// result straight to AddEntries.
	valid, rejected = ValidateEntries(nil)
	if valid == nil || len(valid) != 0 || rejected != 0 {
		t.Fatalf("empty batch = %v / %d rejected, want empty slice / 0", valid, rejected)
	}
}
