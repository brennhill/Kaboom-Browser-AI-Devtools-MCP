// deps.go — Dependency injection interface for the toolconfigure sub-package.
// Purpose: Declares the interface that configure handlers need from the main package.
// Why: Decouples configure handlers from the main package's god object without circular imports.

package toolconfigure

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/noise"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// Deps provides all dependencies the configure-local handlers need.
// *ToolHandler in cmd/browser-agent/ satisfies this interface.
//
// The tutorial and netrecord sub-packages declare their own, narrower Deps —
// see tutorial.Deps and netrecord.NetworkBodyProvider. This interface covers
// only the handlers that remain here (noise, capabilities, security mode,
// telemetry, action jitter).
type Deps interface {
	// NoiseConfig returns the noise filtering configuration.
	NoiseConfig() *noise.NoiseConfig

	// ConsoleEntries returns console entries for noise auto-detection.
	ConsoleEntries() []types.LogEntry

	// NetworkBodies returns captured network bodies for noise auto-detection.
	NetworkBodies() []types.NetworkBody

	// AllWebSocketEvents returns captured WebSocket events.
	AllWebSocketEvents() []types.WebSocketEvent

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
}
