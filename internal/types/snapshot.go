// Purpose: Defines named-snapshot structures for persisted point-in-time browser state comparisons.
// Why: Enables deterministic before/after diagnostics by preserving structured snapshot evidence.
// Docs: docs/features/feature/historical-snapshots/index.md

package types

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
)

// SnapshotError represents a console error or warning in a snapshot.
type SnapshotError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Count   int    `json:"count"`
}

// SnapshotNetworkRequest represents a network request in a snapshot.
type SnapshotNetworkRequest struct {
	Method       string `json:"method"`
	URL          string `json:"url"`
	Status       int    `json:"status"`
	Duration     int    `json:"duration,omitempty"`
	ResponseSize int    `json:"response_size,omitempty"`
	ContentType  string `json:"content_type,omitempty"`
}

// SnapshotWSConnection represents a WebSocket connection in a snapshot.
type SnapshotWSConnection struct {
	URL         string  `json:"url"`
	State       string  `json:"state"`
	MessageRate float64 `json:"message_rate,omitempty"`
}

// NamedSnapshot is a stored point-in-time browser state.
type NamedSnapshot struct {
	Name                 string                            `json:"name"`
	CapturedAt           time.Time                         `json:"captured_at"`
	URLFilter            string                            `json:"url,omitempty"`
	PageURL              string                            `json:"page_url"`
	ConsoleErrors        []SnapshotError                   `json:"console_errors"`
	ConsoleWarnings      []SnapshotError                   `json:"console_warnings"`
	NetworkRequests      []SnapshotNetworkRequest          `json:"network_requests"`
	WebSocketConnections []SnapshotWSConnection            `json:"websocket_connections"`
	Performance          *performance.PerformanceSnapshot  `json:"performance,omitempty"`
	PerformanceSamples   []performance.PerformanceSnapshot `json:"performance_samples,omitempty"`
}
