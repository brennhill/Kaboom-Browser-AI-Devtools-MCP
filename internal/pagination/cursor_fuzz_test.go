// Purpose: Fuzz pagination and cursor operations for edge-case inputs and panics.
// Docs: docs/features/feature/pagination/index.md

// cursor_fuzz_test.go — Fuzz tests for cursor parsing.
package pagination

import (
	"strings"
	"testing"
	"time"
)

// FuzzParseCursor fuzzes the ParseCursor function to verify parsing robustness.
// Tests valid cursor formats, malformed inputs, and edge cases (overflow, unicode, large strings).
//
// Invariants:
// 1. Round-trip: if ParseCursor(s) succeeds, BuildCursor(c.Timestamp, c.Sequence) must parse identically
// 2. Error or valid: never returns garbage (nil error → valid Cursor)
// 3. Empty string always succeeds with zero cursor
func FuzzParseCursor(f *testing.F) {
	// Seed corpus: valid cursors
	f.Add("2024-01-01T00:00:00Z:42")
	f.Add(":100")                               // sequence-only
	f.Add("")                                   // empty (first page)
	f.Add("2024-01-01T00:00:00.123456789Z:999") // RFC3339Nano
	f.Add("2024-12-31T23:59:59Z:0")
	f.Add(":0")
	f.Add("2024-01-01T00:00:00Z:-1") // negative sequence

	// Seed corpus: invalid cursors
	f.Add("no-colon")
	f.Add("2024-01-01T00:00:00Z:not-a-number")
	f.Add(strings.Repeat("a", 10*1024))               // 10KB string
	f.Add("日本語:42")                                   // unicode
	f.Add("2024-01-01T00:00:00Z:9999999999999999999") // int64 overflow
	f.Add("2024-01-01T00:00:00Z")                     // missing sequence
	f.Add("::")
	f.Add("::123")
	f.Add("invalid-timestamp:42")
	f.Add("2024-13-01T00:00:00Z:42") // invalid month
	f.Add(":abc")                    // non-numeric sequence

	f.Fuzz(func(t *testing.T, cursorStr string) {
		// Invariant 3: Empty string always succeeds with zero cursor
		if cursorStr == "" {
			cursor, err := ParseCursor(cursorStr)
			if err != nil {
				t.Fatalf("ParseCursor(\"\") failed: %v (must always succeed)", err)
			}
			if cursor.Timestamp != "" || cursor.Sequence != 0 {
				t.Fatalf("ParseCursor(\"\") returned non-zero cursor: %+v", cursor)
			}
			return
		}

		// Parse the cursor
		cursor, err := ParseCursor(cursorStr)

		if err != nil {
			// Invalid cursor: error is expected for certain inputs
			// Verify error message is non-empty
			if err.Error() == "" {
				t.Fatalf("ParseCursor returned error with empty message for input %q", cursorStr)
			}
			return
		}

		// Invariant 2: If no error, cursor must be valid
		// - Sequence can be any valid int64 (including negative)
		// - Timestamp must be empty or valid RFC3339/RFC3339Nano
		if cursor.Timestamp != "" {
			// Validate timestamp format
			_, err1 := time.Parse(time.RFC3339, cursor.Timestamp)
			_, err2 := time.Parse(time.RFC3339Nano, cursor.Timestamp)
			if err1 != nil && err2 != nil {
				t.Fatalf("ParseCursor succeeded but returned invalid timestamp %q (neither RFC3339 nor RFC3339Nano)", cursor.Timestamp)
			}
		}

		// Invariant 1: Round-trip consistency
		// If ParseCursor(s) succeeded, BuildCursor(timestamp, sequence) → ParseCursor() must produce identical result
		rebuilt := BuildCursor(cursor.Timestamp, cursor.Sequence)
		cursor2, err2 := ParseCursor(rebuilt)
		if err2 != nil {
			t.Fatalf("Round-trip failed: ParseCursor(%q) → %+v → BuildCursor → %q → ParseCursor failed: %v",
				cursorStr, cursor, rebuilt, err2)
		}

		if cursor2.Timestamp != cursor.Timestamp || cursor2.Sequence != cursor.Sequence {
			t.Fatalf("Round-trip mismatch: ParseCursor(%q) → %+v, but BuildCursor → ParseCursor → %+v",
				cursorStr, cursor, cursor2)
		}

		// Additional validation: rebuilt cursor should have expected format
		expectedFormat := true
		if cursor.Timestamp == "" {
			// Sequence-only format: ":N"
			if !strings.HasPrefix(rebuilt, ":") || strings.Count(rebuilt, ":") != 1 {
				expectedFormat = false
			}
		} else {
			// Full format: "timestamp:N"
			// Should have at least 2 colons (timestamp has colons + separator)
			if strings.Count(rebuilt, ":") < 2 {
				expectedFormat = false
			}
		}
		if !expectedFormat {
			t.Fatalf("BuildCursor produced unexpected format %q for cursor %+v", rebuilt, cursor)
		}
	})
}
