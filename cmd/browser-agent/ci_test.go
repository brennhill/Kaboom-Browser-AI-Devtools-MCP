// Purpose: Tests for CI environment integration.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// ci_test.go — Unit tests for CI endpoint pure functions.
package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/ciapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestComputeSnapshotStats(t *testing.T) {
	t.Parallel()

	t.Run("empty inputs", func(t *testing.T) {
		stats := ciapi.ComputeSnapshotStats(nil, nil, nil)
		if stats.TotalLogs != 0 || stats.ErrorCount != 0 || stats.WarningCount != 0 ||
			stats.NetworkFailures != 0 || stats.WSConnections != 0 {
			t.Fatalf("empty inputs should produce zero stats, got %+v", stats)
		}
	})

	t.Run("counts errors and warnings", func(t *testing.T) {
		logs := []mcp.LogEntry{
			{"level": "error", "msg": "crash"},
			{"level": "error", "msg": "timeout"},
			{"level": "warn", "msg": "slow query"},
			{"level": "warning", "msg": "deprecated"},
			{"level": "info", "msg": "started"},
			{"level": "debug", "msg": "trace"},
		}
		stats := ciapi.ComputeSnapshotStats(logs, nil, nil)

		if stats.TotalLogs != 6 {
			t.Fatalf("TotalLogs = %d, want 6", stats.TotalLogs)
		}
		if stats.ErrorCount != 2 {
			t.Fatalf("ErrorCount = %d, want 2", stats.ErrorCount)
		}
		if stats.WarningCount != 2 {
			t.Fatalf("WarningCount = %d, want 2 (warn + warning)", stats.WarningCount)
		}
	})

	t.Run("counts network failures", func(t *testing.T) {
		bodies := []types.NetworkBody{
			{Status: 200, URL: "/ok"},
			{Status: 404, URL: "/missing"},
			{Status: 500, URL: "/error"},
			{Status: 302, URL: "/redirect"},
		}
		stats := ciapi.ComputeSnapshotStats(nil, nil, bodies)

		if stats.NetworkFailures != 2 {
			t.Fatalf("NetworkFailures = %d, want 2 (404 + 500)", stats.NetworkFailures)
		}
	})

	t.Run("counts unique WS connections", func(t *testing.T) {
		wsEvents := []types.WebSocketEvent{
			{URL: "ws://host/a", Event: "open"},
			{URL: "ws://host/a", Event: "message"},
			{URL: "ws://host/b", Event: "open"},
			{URL: "", Event: "close"}, // no URL, not counted
		}
		stats := ciapi.ComputeSnapshotStats(nil, wsEvents, nil)

		if stats.WSConnections != 2 {
			t.Fatalf("WSConnections = %d, want 2", stats.WSConnections)
		}
	})

	t.Run("combined stats", func(t *testing.T) {
		logs := []mcp.LogEntry{
			{"level": "error", "msg": "err1"},
			{"level": "info", "msg": "info1"},
		}
		wsEvents := []types.WebSocketEvent{
			{URL: "ws://host/a", Event: "open"},
		}
		bodies := []types.NetworkBody{
			{Status: 500, URL: "/fail"},
		}
		stats := ciapi.ComputeSnapshotStats(logs, wsEvents, bodies)

		if stats.TotalLogs != 2 || stats.ErrorCount != 1 || stats.NetworkFailures != 1 || stats.WSConnections != 1 {
			t.Fatalf("combined stats incorrect: %+v", stats)
		}
	})
}

func TestFilterLogsSince(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 2, 11, 10, 0, 0, 0, time.UTC)
	logs := []mcp.LogEntry{
		{"ts": base.Add(-2 * time.Second).Format(time.RFC3339Nano), "msg": "old"},
		{"ts": base.Add(-1 * time.Second).Format(time.RFC3339Nano), "msg": "before"},
		{"ts": base.Add(1 * time.Second).Format(time.RFC3339Nano), "msg": "after1"},
		{"ts": base.Add(2 * time.Second).Format(time.RFC3339Nano), "msg": "after2"},
	}

	t.Run("filters entries after since", func(t *testing.T) {
		result := ciapi.FilterLogsSince(logs, base)
		if len(result) != 2 {
			t.Fatalf("expected 2 entries after since, got %d", len(result))
		}
		if result[0]["msg"] != "after1" || result[1]["msg"] != "after2" {
			t.Fatalf("unexpected filtered entries: %v", result)
		}
	})

	t.Run("empty input returns empty", func(t *testing.T) {
		result := ciapi.FilterLogsSince(nil, base)
		if len(result) != 0 {
			t.Fatalf("expected 0 entries for nil input, got %d", len(result))
		}
	})

	t.Run("all before since returns empty", func(t *testing.T) {
		result := ciapi.FilterLogsSince(logs[:2], base)
		if len(result) != 0 {
			t.Fatalf("expected 0 entries when all before since, got %d", len(result))
		}
	})

	t.Run("all after since returns all", func(t *testing.T) {
		earlyTime := base.Add(-10 * time.Second)
		result := ciapi.FilterLogsSince(logs, earlyTime)
		if len(result) != 4 {
			t.Fatalf("expected 4 entries when all after since, got %d", len(result))
		}
	})

	t.Run("skips entries without ts field", func(t *testing.T) {
		logsWithMissing := []mcp.LogEntry{
			{"msg": "no timestamp"},
			{"ts": base.Add(1 * time.Second).Format(time.RFC3339Nano), "msg": "has ts"},
		}
		result := ciapi.FilterLogsSince(logsWithMissing, base)
		if len(result) != 1 {
			t.Fatalf("expected 1 entry (skip missing ts), got %d", len(result))
		}
	})

	t.Run("skips entries with invalid ts", func(t *testing.T) {
		logsWithBadTS := []mcp.LogEntry{
			{"ts": "not-a-timestamp", "msg": "bad ts"},
			{"ts": base.Add(1 * time.Second).Format(time.RFC3339Nano), "msg": "good ts"},
		}
		result := ciapi.FilterLogsSince(logsWithBadTS, base)
		if len(result) != 1 {
			t.Fatalf("expected 1 entry (skip bad ts), got %d", len(result))
		}
	})
}
