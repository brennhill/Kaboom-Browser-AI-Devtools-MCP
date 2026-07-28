// Purpose: Declares the capture-state reader and session-owned list response.
// Docs: docs/features/feature/request-session-correlation/index.md

// types.go — Session comparison types.
// CaptureStateReader and SnapshotListEntry.
package session

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// CaptureStateReader abstracts reading current server state for snapshot capture.
type CaptureStateReader interface {
	GetConsoleErrors() []types.SnapshotError
	GetConsoleWarnings() []types.SnapshotError
	GetNetworkRequests() []types.SnapshotNetworkRequest
	GetWSConnections() []types.SnapshotWSConnection
	GetPerformance() *performance.PerformanceSnapshot
	GetCurrentPageURL() string
}

// SnapshotListEntry is a summary of a snapshot for list response.
type SnapshotListEntry struct {
	Name       string    `json:"name"`
	CapturedAt time.Time `json:"captured_at"`
	PageURL    string    `json:"page_url"`
	ErrorCount int       `json:"error_count"`
}
