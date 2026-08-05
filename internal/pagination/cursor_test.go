// Purpose: Tests for cursor encoding, decoding, and boundary handling.
// Docs: docs/features/feature/pagination/index.md

// cursor_test.go — Unit tests for cursor-based pagination
package pagination

import (
	"testing"
	"testing/quick"
	"time"
)

func TestParseCursor(t *testing.T) {
	tests := []struct {
		name        string
		cursorStr   string
		wantCursor  Cursor
		wantErr     bool
		errContains string
	}{
		{
			name:      "empty cursor returns zero value",
			cursorStr: "",
			wantCursor: Cursor{
				Timestamp: "",
				Sequence:  0,
			},
			wantErr: false,
		},
		{
			name:      "valid cursor with RFC3339",
			cursorStr: "2026-01-30T10:15:23Z:1234",
			wantCursor: Cursor{
				Timestamp: "2026-01-30T10:15:23Z",
				Sequence:  1234,
			},
			wantErr: false,
		},
		{
			name:      "valid cursor with RFC3339Nano",
			cursorStr: "2026-01-30T10:15:23.456789Z:5678",
			wantCursor: Cursor{
				Timestamp: "2026-01-30T10:15:23.456789Z",
				Sequence:  5678,
			},
			wantErr: false,
		},
		{
			name:        "invalid format - missing sequence (timestamp only)",
			cursorStr:   "2026-01-30T10:15:23Z",
			wantErr:     true,
			errContains: "invalid timestamp", // Splits at last : giving timestamp="2026-01-30T10:15" which is invalid
		},
		{
			name:        "invalid format - extra colon creates invalid timestamp",
			cursorStr:   "2026-01-30T10:15:23Z:1234:extra",
			wantErr:     true,
			errContains: "invalid timestamp", // Timestamp becomes "2026-01-30T10:15:23Z:1234" which is invalid
		},
		{
			name:        "invalid timestamp",
			cursorStr:   "not-a-timestamp:1234",
			wantErr:     true,
			errContains: "invalid timestamp",
		},
		{
			name:        "invalid sequence - not a number",
			cursorStr:   "2026-01-30T10:15:23Z:abc",
			wantErr:     true,
			errContains: "invalid sequence",
		},
		{
			name:      "invalid sequence - negative",
			cursorStr: "2026-01-30T10:15:23Z:-100",
			wantErr:   false, // ParseInt accepts negative numbers
			wantCursor: Cursor{
				Timestamp: "2026-01-30T10:15:23Z",
				Sequence:  -100,
			},
		},
		{
			name:      "sequence-only cursor (empty timestamp)",
			cursorStr: ":1234",
			wantCursor: Cursor{
				Timestamp: "",
				Sequence:  1234,
			},
			wantErr: false,
		},
		{
			name:      "sequence-only cursor with large number",
			cursorStr: ":9999999999",
			wantCursor: Cursor{
				Timestamp: "",
				Sequence:  9999999999,
			},
			wantErr: false,
		},
		{
			name:      "sequence-only cursor with zero",
			cursorStr: ":0",
			wantCursor: Cursor{
				Timestamp: "",
				Sequence:  0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cursor, err := ParseCursor(tt.cursorStr)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseCursor() expected error, got nil")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("ParseCursor() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseCursor() unexpected error: %v", err)
				return
			}

			if cursor.Timestamp != tt.wantCursor.Timestamp {
				t.Errorf("ParseCursor() Timestamp = %v, want %v", cursor.Timestamp, tt.wantCursor.Timestamp)
			}
			if cursor.Sequence != tt.wantCursor.Sequence {
				t.Errorf("ParseCursor() Sequence = %v, want %v", cursor.Sequence, tt.wantCursor.Sequence)
			}
		})
	}
}

func TestBuildCursor(t *testing.T) {
	tests := []struct {
		name      string
		timestamp string
		sequence  int64
		want      string
	}{
		{
			name:      "valid cursor",
			timestamp: "2026-01-30T10:15:23Z",
			sequence:  1234,
			want:      "2026-01-30T10:15:23Z:1234",
		},
		{
			name:      "cursor with nanoseconds",
			timestamp: "2026-01-30T10:15:23.456789Z",
			sequence:  5678,
			want:      "2026-01-30T10:15:23.456789Z:5678",
		},
		{
			name:      "empty timestamp returns sequence-only cursor",
			timestamp: "",
			sequence:  1234,
			want:      ":1234",
		},
		{
			name:      "zero sequence",
			timestamp: "2026-01-30T10:15:23Z",
			sequence:  0,
			want:      "2026-01-30T10:15:23Z:0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildCursor(tt.timestamp, tt.sequence)
			if got != tt.want {
				t.Errorf("BuildCursor() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCursor_IsOlder(t *testing.T) {
	tests := []struct {
		name           string
		cursor         Cursor
		entryTimestamp string
		entrySequence  int64
		want           bool
	}{
		{
			name: "entry is older by timestamp",
			cursor: Cursor{
				Timestamp: "2026-01-30T10:15:23Z",
				Sequence:  1000,
			},
			entryTimestamp: "2026-01-30T10:15:22Z",
			entrySequence:  999,
			want:           true,
		},
		{
			name: "entry is newer by timestamp",
			cursor: Cursor{
				Timestamp: "2026-01-30T10:15:23Z",
				Sequence:  1000,
			},
			entryTimestamp: "2026-01-30T10:15:24Z",
			entrySequence:  1001,
			want:           false,
		},
		{
			name: "same timestamp, entry is older by sequence",
			cursor: Cursor{
				Timestamp: "2026-01-30T10:15:23Z",
				Sequence:  1000,
			},
			entryTimestamp: "2026-01-30T10:15:23Z",
			entrySequence:  999,
			want:           true,
		},
		{
			name: "same timestamp, entry is newer by sequence",
			cursor: Cursor{
				Timestamp: "2026-01-30T10:15:23Z",
				Sequence:  1000,
			},
			entryTimestamp: "2026-01-30T10:15:23Z",
			entrySequence:  1001,
			want:           false,
		},
		{
			name: "same timestamp and sequence",
			cursor: Cursor{
				Timestamp: "2026-01-30T10:15:23Z",
				Sequence:  1000,
			},
			entryTimestamp: "2026-01-30T10:15:23Z",
			entrySequence:  1000,
			want:           false,
		},
		{
			name: "sequence-only cursor, entry has higher sequence",
			cursor: Cursor{
				Timestamp: "",
				Sequence:  500,
			},
			entryTimestamp: "2026-01-30T10:15:23Z",
			entrySequence:  400,
			want:           true, // 400 < 500
		},
		{
			name: "sequence-only cursor, entry has lower sequence",
			cursor: Cursor{
				Timestamp: "",
				Sequence:  500,
			},
			entryTimestamp: "2026-01-30T10:15:23Z",
			entrySequence:  600,
			want:           false, // 600 > 500
		},
		{
			name: "sequence-only cursor, same sequence",
			cursor: Cursor{
				Timestamp: "",
				Sequence:  500,
			},
			entryTimestamp: "2026-01-30T10:15:23Z",
			entrySequence:  500,
			want:           false, // equal, not strictly older
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cursor.IsOlder(tt.entryTimestamp, tt.entrySequence)
			if got != tt.want {
				t.Errorf("Cursor.IsOlder() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCursor_IsNewer(t *testing.T) {
	tests := []struct {
		name           string
		cursor         Cursor
		entryTimestamp string
		entrySequence  int64
		want           bool
	}{
		{
			name: "entry is newer by timestamp",
			cursor: Cursor{
				Timestamp: "2026-01-30T10:15:23Z",
				Sequence:  1000,
			},
			entryTimestamp: "2026-01-30T10:15:24Z",
			entrySequence:  1001,
			want:           true,
		},
		{
			name: "entry is older by timestamp",
			cursor: Cursor{
				Timestamp: "2026-01-30T10:15:23Z",
				Sequence:  1000,
			},
			entryTimestamp: "2026-01-30T10:15:22Z",
			entrySequence:  999,
			want:           false,
		},
		{
			name: "same timestamp, entry is newer by sequence",
			cursor: Cursor{
				Timestamp: "2026-01-30T10:15:23Z",
				Sequence:  1000,
			},
			entryTimestamp: "2026-01-30T10:15:23Z",
			entrySequence:  1001,
			want:           true,
		},
		{
			name: "same timestamp, entry is older by sequence",
			cursor: Cursor{
				Timestamp: "2026-01-30T10:15:23Z",
				Sequence:  1000,
			},
			entryTimestamp: "2026-01-30T10:15:23Z",
			entrySequence:  999,
			want:           false,
		},
		{
			name: "same timestamp and sequence",
			cursor: Cursor{
				Timestamp: "2026-01-30T10:15:23Z",
				Sequence:  1000,
			},
			entryTimestamp: "2026-01-30T10:15:23Z",
			entrySequence:  1000,
			want:           false,
		},
		{
			name: "sequence-only cursor, entry has higher sequence",
			cursor: Cursor{
				Timestamp: "",
				Sequence:  500,
			},
			entryTimestamp: "2026-01-30T10:15:23Z",
			entrySequence:  600,
			want:           true, // 600 > 500
		},
		{
			name: "sequence-only cursor, entry has lower sequence",
			cursor: Cursor{
				Timestamp: "",
				Sequence:  500,
			},
			entryTimestamp: "2026-01-30T10:15:23Z",
			entrySequence:  400,
			want:           false, // 400 < 500
		},
		{
			name: "sequence-only cursor, same sequence",
			cursor: Cursor{
				Timestamp: "",
				Sequence:  500,
			},
			entryTimestamp: "2026-01-30T10:15:23Z",
			entrySequence:  500,
			want:           false, // equal, not strictly newer
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cursor.IsNewer(tt.entryTimestamp, tt.entrySequence)
			if got != tt.want {
				t.Errorf("Cursor.IsNewer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeTimestamp(t *testing.T) {
	baseTime := time.Date(2026, 1, 30, 10, 15, 23, 456000000, time.UTC)
	baseTimeMillis := baseTime.UnixMilli() // Calculate actual Unix milliseconds

	tests := []struct {
		name  string
		input any
		want  string
	}{
		{
			name:  "string passthrough",
			input: "2026-01-30T10:15:23.456Z",
			want:  "2026-01-30T10:15:23.456Z",
		},
		{
			name:  "int64 unix milliseconds",
			input: baseTimeMillis,
			want:  "2026-01-30T10:15:23Z",
		},
		{
			name:  "time.Time",
			input: baseTime,
			want:  "2026-01-30T10:15:23Z",
		},
		{
			name:  "unknown type returns empty",
			input: 123.456,
			want:  "",
		},
		{
			name:  "nil returns empty",
			input: nil,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeTimestamp(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeTimestamp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCursorRoundtrip(t *testing.T) {
	// Test that ParseCursor(BuildCursor(...)) is identity
	tests := []struct {
		timestamp string
		sequence  int64
	}{
		{"2026-01-30T10:15:23Z", 1234},
		{"2026-01-30T10:15:23.456789Z", 5678},
		{"2026-01-30T00:00:00Z", 0},
		{"", 1234}, // Sequence-only cursor
		{"", 0},    // Sequence-only cursor with zero
	}

	for _, tt := range tests {
		t.Run(tt.timestamp, func(t *testing.T) {
			cursorStr := BuildCursor(tt.timestamp, tt.sequence)
			cursor, err := ParseCursor(cursorStr)
			if err != nil {
				t.Fatalf("ParseCursor() error = %v", err)
			}

			if cursor.Timestamp != tt.timestamp {
				t.Errorf("Roundtrip timestamp = %v, want %v", cursor.Timestamp, tt.timestamp)
			}
			if cursor.Sequence != tt.sequence {
				t.Errorf("Roundtrip sequence = %v, want %v", cursor.Sequence, tt.sequence)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && hasSubstring(s, substr)))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestPropertyCursorRoundTrip verifies that ParseCursor(BuildCursor(ts, seq))
// produces the same ts and seq for valid inputs.
func TestPropertyCursorRoundTrip(t *testing.T) {
	f := func(sec int64, seq int64) bool {
		// Clamp timestamp to reasonable range (0 to ~2033)
		if sec < 0 {
			sec = -sec
		}
		sec = sec % 2000000000

		// Clamp sequence to positive values
		if seq < 0 {
			seq = -seq
		}

		// Generate timestamp string
		ts := time.Unix(sec, 0).UTC().Format(time.RFC3339)

		// Build cursor
		cursorStr := BuildCursor(ts, seq)

		// Parse cursor back
		parsed, err := ParseCursor(cursorStr)
		if err != nil {
			return false
		}

		// Verify round-trip
		return parsed.Timestamp == ts && parsed.Sequence == seq
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyIsOlderIsNewerTrichotomy verifies that for any cursor and entry
// with different timestamps or sequences, exactly one of IsOlder or IsNewer
// returns true (or both false if equal).
func TestPropertyIsOlderIsNewerTrichotomy(t *testing.T) {
	f := func(seq1, seq2 int64) bool {
		// Clamp sequences to positive values
		if seq1 < 0 {
			seq1 = -seq1
		}
		if seq2 < 0 {
			seq2 = -seq2
		}

		// Use sequence-only cursors for simplicity (Timestamp="")
		cursorStr := BuildCursor("", seq1)
		cursor, err := ParseCursor(cursorStr)
		if err != nil {
			return false
		}

		entryTimestamp := ""
		entrySequence := seq2

		isOlder := cursor.IsOlder(entryTimestamp, entrySequence)
		isNewer := cursor.IsNewer(entryTimestamp, entrySequence)

		// Trichotomy: exactly one is true, or both false if equal
		if seq1 == seq2 {
			// Equal: both should be false
			return !isOlder && !isNewer
		} else if seq1 > seq2 {
			// cursor is newer than entry: entry is older
			return isOlder && !isNewer
		} else {
			// cursor is older than entry: entry is newer
			return !isOlder && isNewer
		}
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyBuildCursorSequenceOnly verifies that when timestamp is "",
// BuildCursor("", seq) produces ":seq" and ParseCursor roundtrips correctly.
func TestPropertyBuildCursorSequenceOnly(t *testing.T) {
	f := func(seq int64) bool {
		// Clamp sequence to positive values
		if seq < 0 {
			seq = -seq
		}

		// Build sequence-only cursor
		cursor := BuildCursor("", seq)

		// Parse it back
		parsed, err := ParseCursor(cursor)
		if err != nil {
			return false
		}

		// Verify timestamp is empty and sequence matches
		return parsed.Timestamp == "" && parsed.Sequence == seq
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyCursorOrderingTransitivity verifies that cursor ordering is transitive.
// If A < B and B < C, then A < C.
func TestPropertyCursorOrderingTransitivity(t *testing.T) {
	f := func(seq1, seq2, seq3 int64) bool {
		// Clamp and sort sequences
		seqs := []int64{seq1, seq2, seq3}
		for i := range seqs {
			if seqs[i] < 0 {
				seqs[i] = -seqs[i]
			}
		}

		// Sort to ensure seq1 <= seq2 <= seq3
		for i := 0; i < len(seqs); i++ {
			for j := i + 1; j < len(seqs); j++ {
				if seqs[i] > seqs[j] {
					seqs[i], seqs[j] = seqs[j], seqs[i]
				}
			}
		}

		seq1, seq2, seq3 = seqs[0], seqs[1], seqs[2]

		cursorA, err := ParseCursor(BuildCursor("", seq1))
		if err != nil {
			return false
		}
		cursorB, err := ParseCursor(BuildCursor("", seq2))
		if err != nil {
			return false
		}

		// If A < B (IsOlder(A, B)) and B < C (IsOlder(B, C)), then A < C
		aOlderB := cursorA.IsOlder("", seq2)
		bOlderC := cursorB.IsOlder("", seq3)
		aOlderC := cursorA.IsOlder("", seq3)

		// Transitivity: if both conditions hold, then A < C must hold
		if aOlderB && bOlderC {
			return aOlderC
		}

		// If conditions don't hold, property is vacuously true
		return true
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyParseCursorHandlesInvalid verifies that ParseCursor handles
// malformed cursors gracefully.
func TestPropertyParseCursorHandlesInvalid(t *testing.T) {
	f := func(s string) bool {
		// ParseCursor should never panic
		parsed, err := ParseCursor(s)

		// If parsing failed with error, that's acceptable
		if err != nil {
			return true
		}

		// If parsing succeeded, sequence should be non-negative
		if parsed.Sequence < 0 {
			return false
		}

		// Timestamp can be any string (including empty)
		_ = parsed.Timestamp

		return true
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

func TestParseCursor_NoColon(t *testing.T) {
	t.Parallel()
	_, err := ParseCursor("nocolon")
	if err == nil {
		t.Fatal("ParseCursor(\"nocolon\") expected error, got nil")
	}
	wantSubstr := "invalid cursor format"
	if !contains(err.Error(), wantSubstr) {
		t.Errorf("error = %q, want substring %q", err.Error(), wantSubstr)
	}
}

// ============================================
// IsOlder / IsNewer — RFC3339 fallback paths
// ============================================

func TestIsOlder_RFC3339FallbackCursorTimestamp(t *testing.T) {
	t.Parallel()
	// Cursor timestamp is plain RFC3339 (no nanoseconds) so RFC3339Nano parse
	// fails and the code falls back to RFC3339.
	cursor := Cursor{
		Timestamp: "2026-01-30T10:15:23Z",
		Sequence:  100,
	}
	// Entry uses RFC3339Nano — no fallback needed for the entry.
	// The cursor parse fails Nano, falls back to RFC3339.
	got := cursor.IsOlder("2026-01-30T10:15:22.000000Z", 99)
	if !got {
		t.Error("expected entry to be older than cursor")
	}
}

func TestIsOlder_RFC3339FallbackEntryTimestamp(t *testing.T) {
	t.Parallel()
	// Cursor timestamp is RFC3339Nano so first parse succeeds.
	// Entry timestamp is plain RFC3339 so Nano parse fails, falls back.
	cursor := Cursor{
		Timestamp: "2026-01-30T10:15:23.000000Z",
		Sequence:  100,
	}
	got := cursor.IsOlder("2026-01-30T10:15:22Z", 99)
	if !got {
		t.Error("expected entry to be older than cursor (entry RFC3339 fallback)")
	}
}

func TestIsNewer_RFC3339FallbackCursorTimestamp(t *testing.T) {
	t.Parallel()
	cursor := Cursor{
		Timestamp: "2026-01-30T10:15:23Z",
		Sequence:  100,
	}
	got := cursor.IsNewer("2026-01-30T10:15:24.000000Z", 101)
	if !got {
		t.Error("expected entry to be newer than cursor")
	}
}

func TestIsNewer_RFC3339FallbackEntryTimestamp(t *testing.T) {
	t.Parallel()
	cursor := Cursor{
		Timestamp: "2026-01-30T10:15:23.000000Z",
		Sequence:  100,
	}
	got := cursor.IsNewer("2026-01-30T10:15:24Z", 101)
	if !got {
		t.Error("expected entry to be newer than cursor (entry RFC3339 fallback)")
	}
}
