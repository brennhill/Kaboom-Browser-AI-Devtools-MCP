// runtime_reader_test.go — Tests runtime state projection into session snapshots.

package session

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestRuntimeStateReaderAggregatesConsoleErrors(t *testing.T) {
	t.Parallel()
	reader := NewRuntimeStateReader(func() []types.LogEntry {
		return []types.LogEntry{
			{"level": "error", "message": " broken "},
			{"level": "error", "message": "broken"},
			{"level": "warn", "message": "careful"},
		}
	}, nil)
	errors := reader.GetConsoleErrors()
	if len(errors) != 1 || errors[0].Message != "broken" || errors[0].Count != 2 {
		t.Fatalf("errors = %#v", errors)
	}
}
