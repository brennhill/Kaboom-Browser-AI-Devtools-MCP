// ring_window_bench_test.go — HTTP-ingest hot-path benchmarks for the bounded log window.
// Docs: docs/features/feature/backend-log-ingestion/index.md

package logstore

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// BenchmarkEntryWindowHTTPIngestBudget isolates the mutation performed for
// each validated HTTP log entry. It should remain comfortably below the
// repository's 0.5ms HTTP processing budget without steady-state allocations.
func BenchmarkEntryWindowHTTPIngestBudget(b *testing.B) {
	window := newEntryWindow(10_000)
	now := time.Unix(10, 0)
	entry := types.LogEntry{"level": "info", "message": "bounded ingest"}
	for i := 0; i < len(window.entries); i++ {
		window.append(entry, now)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		window.append(entry, now)
	}
}
