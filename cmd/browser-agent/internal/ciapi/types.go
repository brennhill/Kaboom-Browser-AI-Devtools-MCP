// types.go — CI snapshot and test-boundary HTTP payload contracts.
// Docs: docs/features/feature/ci-infrastructure/index.md

package ciapi

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// SnapshotResponse is the aggregated state returned by GET /snapshot.
type SnapshotResponse struct {
	Timestamp       string                 `json:"timestamp"`
	TestID          string                 `json:"test_id,omitempty"`
	Logs            []mcp.LogEntry         `json:"logs"`
	WebSocket       []types.WebSocketEvent `json:"websocket_events"`
	NetworkBodies   []types.NetworkBody    `json:"network_bodies"`
	EnhancedActions []types.EnhancedAction `json:"enhanced_actions,omitempty"`
	Stats           SnapshotStats          `json:"stats"`
}

// SnapshotStats summarizes the snapshot contents.
type SnapshotStats struct {
	TotalLogs       int `json:"total_logs"`
	ErrorCount      int `json:"error_count"`
	WarningCount    int `json:"warning_count"`
	NetworkFailures int `json:"network_failures"`
	WSConnections   int `json:"ws_connections"`
}

// TestBoundaryRequest is the request body for POST /test-boundary.
type TestBoundaryRequest struct {
	TestID string `json:"test_id"`
	Action string `json:"action"`
}
