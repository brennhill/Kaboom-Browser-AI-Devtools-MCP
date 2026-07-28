// Purpose: Unit tests for the independently synchronized extension-log store.
// Why: Guards the extracted extension-log store behavior after Capture decomposition.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"testing"
	"time"
)

func TestExtensionLogStore_AddAppliesAmortizedEviction(t *testing.T) {
	t.Parallel()

	store := newExtensionLogStore(nil)
	total := MaxExtensionLogs + MaxExtensionLogs/2 + 1
	for i := 0; i < total; i++ {
		store.Add([]types.ExtensionLog{{
			Level:     "info",
			Message:   fmt.Sprintf("log-%d", i),
			Timestamp: time.Unix(int64(i), 0),
		}})
	}

	entries := store.Entries()
	if got := len(entries); got != MaxExtensionLogs {
		t.Fatalf("buffer length = %d, want %d", got, MaxExtensionLogs)
	}

	// After compaction we should retain the newest MaxExtensionLogs entries.
	expectedFirst := total - MaxExtensionLogs
	if got := entries[0].Message; got != fmt.Sprintf("log-%d", expectedFirst) {
		t.Fatalf("first kept log = %q, want %q", got, fmt.Sprintf("log-%d", expectedFirst))
	}
	if got := entries[len(entries)-1].Message; got != fmt.Sprintf("log-%d", total-1) {
		t.Fatalf("last kept log = %q, want %q", got, fmt.Sprintf("log-%d", total-1))
	}
}

func TestExtensionLogStore_EntriesReturnsDetachedCopy(t *testing.T) {
	t.Parallel()

	store := newExtensionLogStore(nil)
	store.Add([]types.ExtensionLog{{Level: "info", Message: "one"}, {Level: "warn", Message: "two"}})

	snap := store.Entries()
	if len(snap) != 2 {
		t.Fatalf("snapshot len = %d, want 2", len(snap))
	}
	snap[0].Message = "mutated"

	if got := store.Entries()[0].Message; got != "one" {
		t.Fatalf("buffer should remain unchanged, got %q", got)
	}
}

func TestExtensionLogStore_ClearReturnsCountAndEmpties(t *testing.T) {
	t.Parallel()

	store := newExtensionLogStore(nil)
	store.Add([]types.ExtensionLog{{Level: "info", Message: "one"}, {Level: "warn", Message: "two"}})

	count := store.Clear()
	if count != 2 {
		t.Fatalf("clear count = %d, want 2", count)
	}
	if entries := store.Entries(); len(entries) != 0 {
		t.Fatalf("buffer len after clear = %d, want 0", len(entries))
	}
}
