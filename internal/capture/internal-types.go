// Purpose: Defines the internal performance snapshot/baseline store used by capture.
// Why: Isolates non-exported supporting state to keep core capture structs focused and readable.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
)

// PerformanceStore manages performance snapshots and baselines with LRU eviction.
type PerformanceStore struct {
	snapshots       map[string]performance.Snapshot
	snapshotOrder   []string
	baselines       map[string]performance.Baseline
	baselineOrder   []string
	beforeSnapshots map[string]performance.Snapshot // keyed by correlation_id, for perf_diff
}
