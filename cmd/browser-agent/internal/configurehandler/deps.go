// deps.go — Dependency injection interface for the configurehandler sub-package.
// Purpose: Declares the interface that configure handlers need from the main package.
// Why: Decouples configure handlers from the main package's god object without circular imports.

package configurehandler

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/audit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/noise"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// Deps provides all dependencies the configure-local handlers need.
// *ToolHandler in cmd/browser-agent/ satisfies this interface.
type Deps interface {
	// NoiseConfig returns the noise filtering configuration.
	NoiseConfig() *noise.NoiseConfig

	// ConsoleEntries returns console entries for noise auto-detection.
	ConsoleEntries() []noise.LogEntry

	// NetworkBodies returns captured network bodies for noise auto-detection.
	NetworkBodies() []types.NetworkBody

	// AllWebSocketEvents returns captured WebSocket events.
	AllWebSocketEvents() []capture.WebSocketEvent

	// GetTrackingStatus returns (enabled, tabID, tabURL) for the tracked tab.
	GetTrackingStatus() (bool, int, string)

	// GetPilotStatus returns the AI Web Pilot status.
	GetPilotStatus() any

	// IsExtensionConnected returns whether the extension is connected.
	IsExtensionConnected() bool

	// ToolsList returns the list of registered MCP tools.
	ToolsList() []mcp.MCPTool

	// GetToolModuleExamples returns examples for a tool module by name.
	GetToolModuleExamples(toolName string) any

	// HasCapture returns whether the capture subsystem is initialized.
	HasCapture() bool

	// GetSecurityMode returns current security mode, production parity, and rewrites.
	GetSecurityMode() (string, bool, []string)

	// SetSecurityMode updates the security mode.
	SetSecurityMode(mode string, rewrites []string)

	// GetTelemetryMode returns the current telemetry mode.
	GetTelemetryMode() string

	// SetTelemetryMode updates the telemetry mode.
	SetTelemetryMode(mode string)

	// InteractActionSetJitter sets the action jitter in milliseconds.
	InteractActionSetJitter(ms int)

	// InteractActionGetJitter returns the current action jitter in milliseconds.
	InteractActionGetJitter() int

	// GetActiveCodebase returns the active project directory (empty if none set).
	GetActiveCodebase() string

	// StartTestBoundary records a test boundary as active (keyed by test id).
	StartTestBoundary(testID string)

	// EndTestBoundary clears a boundary and reports whether it was active.
	EndTestBoundary(testID string) bool

	// AuditTrailReady reports whether the audit trail is initialized.
	AuditTrailReady() bool

	// QueryAuditLog returns audit entries matching the filter.
	QueryAuditLog(filter audit.Filter) []audit.Entry

	// ClearAuditLog clears the audit trail (and its session map) and returns the count cleared.
	ClearAuditLog() int
}
