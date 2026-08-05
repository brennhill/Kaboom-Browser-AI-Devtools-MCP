// check_go_test_determinism_test.go — Regression tests for the unit-test wall-clock ban.
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

func TestSleepViolationsRequireAbsoluteZero(t *testing.T) {
	if violations := sleepViolations(map[string]int{}); len(violations) != 0 {
		t.Fatalf("zero sleeps must pass: %v", violations)
	}
	violations := sleepViolations(map[string]int{
		"internal/z_test.go": 2,
		"internal/a_test.go": 1,
	})
	want := []string{
		"internal/a_test.go: 1 time.Sleep call(s)",
		"internal/z_test.go: 2 time.Sleep call(s)",
	}
	if len(violations) != len(want) {
		t.Fatalf("violations = %v, want %v", violations, want)
	}
	for i := range want {
		if violations[i] != want[i] {
			t.Fatalf("violations[%d] = %q, want %q", i, violations[i], want[i])
		}
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
