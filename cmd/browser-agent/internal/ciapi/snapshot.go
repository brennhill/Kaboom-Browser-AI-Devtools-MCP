// snapshot.go — Pure CI snapshot filtering and statistics.
// Docs: docs/features/feature/ci-infrastructure/index.md

package ciapi

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// FilterLogsSince returns entries whose RFC3339Nano ts field is after since.
func FilterLogsSince(logs []mcp.LogEntry, since time.Time) []mcp.LogEntry {
	result := make([]mcp.LogEntry, 0, len(logs))
	for _, entry := range logs {
		ts, ok := entry["ts"].(string)
		if !ok {
			continue
		}
		entryTime, err := time.Parse(time.RFC3339Nano, ts)
		if err == nil && entryTime.After(since) {
			result = append(result, entry)
		}
	}
	return result
}

// ComputeSnapshotStats computes summary statistics for a snapshot.
func ComputeSnapshotStats(logs []mcp.LogEntry, wsEvents []types.WebSocketEvent, networkBodies []types.NetworkBody) SnapshotStats {
	stats := SnapshotStats{TotalLogs: len(logs)}
	for _, entry := range logs {
		level, _ := entry["level"].(string)
		switch level {
		case "error":
			stats.ErrorCount++
		case "warn", "warning":
			stats.WarningCount++
		}
	}
	for _, body := range networkBodies {
		if body.Status >= 400 {
			stats.NetworkFailures++
		}
	}
	connections := make(map[string]bool)
	for _, event := range wsEvents {
		if event.URL != "" {
			connections[event.URL] = true
		}
	}
	stats.WSConnections = len(connections)
	return stats
}
