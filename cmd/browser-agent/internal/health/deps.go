// deps.go — Dependency injection for the health sub-package.
// Purpose: Declares the external dependencies health handlers need from the main package.
// Why: Decouples health/doctor logic from the main package's god object without circular imports.

package health

import "time"

// ServerDeps abstracts the Server fields needed by health response builders.
type ServerDeps interface {
	// GetTerminalPort returns the terminal server port, or 0 if not running.
	GetTerminalPort() int
	// GetConsoleStats returns console buffer entries, capacity, and drop count.
	GetConsoleStats() (entries int, capacity int, dropped int64)
}

// UpgradeProvider returns binary upgrade detection state.
type UpgradeProvider interface {
	// UpgradeInfo returns whether an upgrade is pending, the new version, and detection time.
	UpgradeInfo() (pending bool, newVersion string, detectedAt time.Time)
}

// LaunchModeInfo is the health payload projection of launchmode.Info.
type LaunchModeInfo struct {
	Mode          string
	Reason        string
	ParentProcess string
}

// SetupDeps holds dependencies for CLI setup doctor checks.
type SetupDeps struct {
	// Version is the server version string.
	Version string
	// PortKillHint returns a platform-appropriate command to kill a process on the given port.
	PortKillHint func(port int) string
	// FastPathTelemetryLogPath returns the path to the fast-path telemetry log.
	FastPathTelemetryLogPath func() (string, error)
}

// SetupCheckOptions configures thresholds for setup doctor checks.
type SetupCheckOptions struct {
	MinSamples      int
	MaxFailureRatio float64
}
