// filtering_test.go — Verifies canonical observe filter rankings.
// Docs: docs/features/feature/observe/index.md

package core

import "testing"

func TestLogLevelRank(t *testing.T) {
	t.Parallel()
	for level, want := range map[string]int{
		"debug": 0,
		"log":   1,
		"info":  2,
		"warn":  3,
		"error": 4,
		"other": -1,
	} {
		if got := LogLevelRank(level); got != want {
			t.Errorf("LogLevelRank(%q) = %d, want %d", level, got, want)
		}
	}
}
