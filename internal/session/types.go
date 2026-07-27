// Purpose: Declares CaptureStateReader interface and snapshot data types (NamedSnapshot, SnapshotError, etc.).
// Docs: docs/features/feature/request-session-correlation/index.md

// types.go — Session comparison types.
// CaptureStateReader, NamedSnapshot, and related snapshot types.
package session

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	gastypes "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// CaptureStateReader abstracts reading current server state for snapshot capture.
type CaptureStateReader interface {
	GetConsoleErrors() []SnapshotError
	GetConsoleWarnings() []SnapshotError
	GetNetworkRequests() []SnapshotNetworkRequest
	GetWSConnections() []SnapshotWSConnection
	GetPerformance() *performance.PerformanceSnapshot
	GetCurrentPageURL() string
}

// Snapshot* types are aliases to canonical snapshot contract in internal/types.
type (
	SnapshotError          = gastypes.SnapshotError
	SnapshotNetworkRequest = gastypes.SnapshotNetworkRequest
	SnapshotWSConnection   = gastypes.SnapshotWSConnection
	NamedSnapshot          = gastypes.NamedSnapshot
)

// SnapshotListEntry is a summary of a snapshot for list response.
type SnapshotListEntry struct {
	Name       string    `json:"name"`
	CapturedAt time.Time `json:"captured_at"`
	PageURL    string    `json:"page_url"`
	ErrorCount int       `json:"error_count"`
}
