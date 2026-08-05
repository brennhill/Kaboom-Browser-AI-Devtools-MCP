// contract_test.go — Unit tests for saved-sequence contract invariants.
// Docs: docs/features/feature/batch-sequences/index.md

package sequencehandler

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
