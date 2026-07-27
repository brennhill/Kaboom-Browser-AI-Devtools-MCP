// deps.go -- Dependency injection for bridge sub-package.
// Purpose: Declares the external dependencies the bridge needs from the main package.
// Why: Decouples the bridge from the main package's god object without circular imports.

package bridge

import (
	"encoding/json"
	"io"

	internbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
)

// Deps holds all external dependencies the bridge needs from the caller.
// Set once at startup via Init(), then read-only for the bridge lifetime.
type Deps struct {
	// Version is the binary version string (e.g. "0.8.1").
	Version string

	// MaxPostBodySize is the maximum HTTP response body size in bytes.
	MaxPostBodySize int64

	// MCPServerName is the canonical server identity shown to MCP clients.
	MCPServerName string

	// LegacyMCPServerNames are previous names accepted for compatibility.
	LegacyMCPServerNames []string

	// ServerInstructions is the instructions string sent in MCP initialize responses.
	ServerInstructions string

	// -- Logging --

	// Stderrf writes formatted log messages to the diagnostic stderr sink.
	Stderrf func(format string, args ...any)

	// Debugf writes formatted debug messages.
	Debugf func(format string, args ...any)

	// -- Stdout transport --

	// WriteMCPPayload writes a JSON-RPC payload to the MCP transport writer.
	WriteMCPPayload func(payload []byte, framing internbridge.StdioFraming)

	// SyncStdoutBestEffort flushes stdout (best-effort).
	SyncStdoutBestEffort func()

	// SetStderrSink redirects the stderr diagnostic sink to the given writer.
	SetStderrSink func(w io.Writer)

	// -- Push state --

	// GetBridgeFraming returns the current stdio framing mode.
	GetBridgeFraming func() internbridge.StdioFraming

	// StoreBridgeFraming saves the framing mode detected during MCP initialize.
	StoreBridgeFraming func(f internbridge.StdioFraming)

	// SetPushClientCapabilities stores capabilities extracted from MCP initialize.
	SetPushClientCapabilities func(caps push.ClientCapabilities)

	// ExtractClientCapabilities parses client capabilities from MCP initialize params.
	ExtractClientCapabilities func(rawParams json.RawMessage) push.ClientCapabilities

	// -- MCP content --

	// NegotiateProtocolVersion negotiates the MCP protocol version from initialize params.
	NegotiateProtocolVersion func(rawParams json.RawMessage) string

	// MCPResources returns the list of MCP resource definitions.
	MCPResources func() []mcp.MCPResource

	// MCPResourceTemplates returns the list of MCP resource template definitions.
	MCPResourceTemplates func() []any

	// ResolveResourceContent resolves a resource URI to canonical URI, text content, and found flag.
	ResolveResourceContent func(uri string) (canonicalURI string, text string, ok bool)

	// -- Daemon lifecycle --

	// DaemonProcessArgv0 returns the argv[0] name for spawned daemon processes.
	DaemonProcessArgv0 func(exePath string) string

	// StopServerForUpgrade stops a running daemon for version upgrade.
	StopServerForUpgrade func(port int) bool

	// FindProcessOnPort returns PIDs listening on the given port.
	FindProcessOnPort func(port int) ([]int, error)

	// IsProcessAlive checks if a process with the given PID is still running.
	IsProcessAlive func(pid int) bool

	// AppendExitDiagnostic writes an exit diagnostic entry.
	AppendExitDiagnostic func(event string, extra map[string]any) string
}

// deps is the package-level dependency holder, set once via Init().
var deps Deps

// Init sets the bridge dependencies. Must be called before any bridge function.
func Init(d Deps) {
	deps = d
}
