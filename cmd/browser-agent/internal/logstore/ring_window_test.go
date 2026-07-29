// ring_window_test.go — Regression tests for allocation-free log-window eviction.
// Docs: docs/features/feature/backend-log-ingestion/index.md

package logstore

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestEntryWindowSteadyStateAppendReusesStorage(t *testing.T) {
	window := newEntryWindow(3)
	now := time.Unix(10, 0)
	for i := 0; i < 3; i++ {
		window.append(types.LogEntry{"sequence": i}, now)
	}

	allocs := testing.AllocsPerRun(1000, func() {
		window.append(types.LogEntry(nil), now)
	})
	if allocs != 0 {
		t.Fatalf("steady-state append allocated %.2f times per run, want 0", allocs)
	}
}

func TestEntryWindowSnapshotPreservesEvictionOrderAndTimestamps(t *testing.T) {
	window := newEntryWindow(3)
	for i := 1; i <= 5; i++ {
		window.append(types.LogEntry{"sequence": i}, time.Unix(int64(i), 0))
	}

	entries, addedAt := window.snapshot()
	for i, want := range []int{3, 4, 5} {
		if got := entries[i]["sequence"]; got != want {
			t.Fatalf("entries[%d].sequence = %v, want %d", i, got, want)
		}
		if got := addedAt[i].Unix(); got != int64(want) {
			t.Fatalf("addedAt[%d] = %d, want %d", i, got, want)
		}
	}
}
