// deps.go — Explicit dependencies for configure-local handlers.
// Why: Keeps configure handlers independent of the composition root without
// forcing that root to expose a broad adapter-method surface.

package toolconfigure

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/noise"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// Deps groups the exact callbacks used by configure-local handlers. Function
// fields make ownership explicit at composition and keep tests free of fake
// host objects that mirror ToolHandler.
type Deps struct {
	NoiseConfig             func() *noise.NoiseConfig
	ConsoleEntries          func() []types.LogEntry
	NetworkBodies           func() []types.NetworkBody
	AllWebSocketEvents      func() []types.WebSocketEvent
	ToolsList               func() []mcp.MCPTool
	GetToolModuleExamples   func(toolName string) any
	HasCapture              func() bool
	GetSecurityMode         func() (string, bool, []string)
	SetSecurityMode         func(mode string, rewrites []string)
	GetTelemetryMode        func() string
	SetTelemetryMode        func(mode string)
	InteractActionSetJitter func(ms int)
	InteractActionGetJitter func() int
}
