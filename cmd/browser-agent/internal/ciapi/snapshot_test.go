// snapshot_test.go — Characterization tests for CI snapshot shaping.
// Docs: docs/features/feature/ci-infrastructure/index.md

package ciapi

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"testing"
	"time"
)

func TestComputeSnapshotStatsCountsFailuresAndConnections(t *testing.T) {
	stats := ComputeSnapshotStats(
		[]types.LogEntry{{"level": "error"}, {"level": "warning"}},
		[]types.WebSocketEvent{{URL: "wss://one"}, {URL: "wss://one"}, {URL: "wss://two"}},
		[]types.NetworkBody{{Status: 200}, {Status: 503}},
	)

	if stats.TotalLogs != 2 || stats.ErrorCount != 1 || stats.WarningCount != 1 {
		t.Fatalf("unexpected log stats: %#v", stats)
	}
	if stats.NetworkFailures != 1 || stats.WSConnections != 2 {
		t.Fatalf("unexpected transport stats: %#v", stats)
	}
}

func TestFilterLogsSinceIgnoresMalformedTimestamps(t *testing.T) {
	since := time.Date(2026, time.July, 27, 10, 0, 0, 0, time.UTC)
	logs := []types.LogEntry{
		{"ts": since.Add(-time.Second).Format(time.RFC3339Nano)},
		{"ts": "invalid"},
		{"ts": since.Add(time.Second).Format(time.RFC3339Nano)},
	}

	filtered := FilterLogsSince(logs, since)
	if len(filtered) != 1 || filtered[0]["ts"] != logs[2]["ts"] {
		t.Fatalf("unexpected filtered logs: %#v", filtered)
	}
}
