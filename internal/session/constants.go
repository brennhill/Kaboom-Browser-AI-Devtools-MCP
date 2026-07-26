// Purpose: Defines snapshot name limits, capacity caps, and performance regression thresholds.
// Docs: docs/features/feature/request-session-correlation/index.md

// constants.go — Session comparison constants.
// All configuration values for session package.
package session

const (
	// Snapshot name constraints
	maxSnapshotNameLen    = 50
	maxConsolePerSnapshot = 50
	maxNetworkPerSnapshot = 100
	reservedSnapshotName  = "current"
)
