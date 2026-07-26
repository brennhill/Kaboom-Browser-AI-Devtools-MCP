// toolconfigure_test.go — Unit tests for the toolconfigure sub-package exported API.

package toolconfigure

import (
	"testing"
)

// Sequence types and constants
// ---------------------------------------------------------------------------

func TestSequenceNamePattern_Valid(t *testing.T) {
	valid := []string{"my-sequence", "login_flow", "TestA123", "a", "A-B_c"}
	for _, name := range valid {
		if !SequenceNamePattern.MatchString(name) {
			t.Errorf("expected %q to match SequenceNamePattern", name)
		}
	}
}

func TestSequenceNamePattern_Invalid(t *testing.T) {
	invalid := []string{"", "has space", "special!", "path/name", "tab\there"}
	for _, name := range invalid {
		if SequenceNamePattern.MatchString(name) {
			t.Errorf("expected %q to NOT match SequenceNamePattern", name)
		}
	}
}

func TestSequenceConstants(t *testing.T) {
	if MaxSequenceSteps != 50 {
		t.Errorf("MaxSequenceSteps: want 50, got %d", MaxSequenceSteps)
	}
	if MaxSequenceNameLen != 64 {
		t.Errorf("MaxSequenceNameLen: want 64, got %d", MaxSequenceNameLen)
	}
	if DefaultStepTimeout != 10000 {
		t.Errorf("DefaultStepTimeout: want 10000, got %d", DefaultStepTimeout)
	}
}

// ---------------------------------------------------------------------------
// NormalizeTelemetryMode
// ---------------------------------------------------------------------------

func TestNormalizeTelemetryMode(t *testing.T) {
	tests := []struct {
		input  string
		want   string
		wantOK bool
	}{
		{"off", "off", true},
		{"auto", "auto", true},
		{"full", "full", true},
		{"invalid", "", false},
		{"", "", false},
		{"  off  ", "  off  ", true}, // validates after trim, returns original
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := NormalizeTelemetryMode(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ok: want %v, got %v", tt.wantOK, ok)
			}
			if got != tt.want {
				t.Errorf("mode: want %q, got %q", tt.want, got)
			}
		})
	}
}
