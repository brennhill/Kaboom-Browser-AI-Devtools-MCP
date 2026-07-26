// Purpose: Tests for semantic version comparison logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import "testing"

func TestParseVersionParts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  []int
	}{
		{"0.7.5", []int{0, 7, 5}},
		{"1.2.3", []int{1, 2, 3}},
		{"v0.7.5", []int{0, 7, 5}},
		{"10.20.30", []int{10, 20, 30}},
		{"0.0.0", []int{0, 0, 0}},
		{"1.0", []int{1, 0}},
		{"5", []int{5}},
	}

	for _, tt := range tests {
		got := parseVersionParts(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseVersionParts(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseVersionParts(%q)[%d] = %d, want %d", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestParseVersionParts_Malformed(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"", "abc", "v", "1.2.abc", "..."} {
		got := parseVersionParts(input)
		if got == nil {
			continue // nil is acceptable for malformed
		}
		// Any returned parts should be valid ints (no negative)
		for _, v := range got {
			if v < 0 {
				t.Errorf("parseVersionParts(%q) returned negative part: %d", input, v)
			}
		}
	}
}

func TestIsNewerVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		candidate string
		current   string
		want      bool
	}{
		// Newer patch
		{"0.7.6", "0.7.5", true},
		// Newer minor
		{"0.8.0", "0.7.5", true},
		// Newer major
		{"1.0.0", "0.7.5", true},
		// Same version
		{"0.7.5", "0.7.5", false},
		// Older patch
		{"0.7.4", "0.7.5", false},
		// Older minor
		{"0.6.9", "0.7.5", false},
		// Older major
		{"0.5.0", "0.7.5", false},
		// v-prefix handling
		{"v0.7.6", "v0.7.5", true},
		{"v0.7.6", "0.7.5", true},
		{"0.7.6", "v0.7.5", true},
		// Mixed length versions
		{"0.8", "0.7.5", true},
		{"0.7.5.1", "0.7.5", true},
		// Empty strings
		{"", "0.7.5", false},
		{"0.7.6", "", false},
		{"", "", false},
		// Malformed
		{"abc", "0.7.5", false},
		{"0.7.6", "xyz", false},
	}

	for _, tt := range tests {
		got := isNewerVersion(tt.candidate, tt.current)
		if got != tt.want {
			t.Errorf("isNewerVersion(%q, %q) = %v, want %v", tt.candidate, tt.current, got, tt.want)
		}
	}
}
