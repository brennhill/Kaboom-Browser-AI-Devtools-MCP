// check_go_test_determinism_test.go — Regression tests for the unit-test wall-clock ratchet.
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanSleepCountsUsesGoSyntax(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "internal/example/example_test.go", `package example
import "time"
func testThing() {
	_ = "time.Sleep(ignored in a string)"
	// time.Sleep(ignored in a comment)
	time.Sleep(time.Millisecond)
}`)

	counts, err := scanSleepCounts(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := counts["internal/example/example_test.go"]; got != 1 {
		t.Fatalf("sleep count = %d, want 1", got)
	}
}

func TestEvaluateSleepRatchetRejectsNewAndIncreasedDebt(t *testing.T) {
	baseline := map[string]int{"internal/existing_test.go": 1}
	cases := []struct {
		name   string
		counts map[string]int
		fail   bool
	}{
		{name: "unchanged", counts: map[string]int{"internal/existing_test.go": 1}},
		{name: "reduced", counts: map[string]int{}},
		{name: "increased", counts: map[string]int{"internal/existing_test.go": 2}, fail: true},
		{name: "new file", counts: map[string]int{"internal/new_test.go": 1}, fail: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			violations := evaluateSleepRatchet(tc.counts, baseline)
			if tc.fail && len(violations) == 0 {
				t.Fatal("expected a determinism violation")
			}
			if !tc.fail && len(violations) != 0 {
				t.Fatalf("unexpected violations: %v", violations)
			}
		})
	}
}

func writeTestFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
