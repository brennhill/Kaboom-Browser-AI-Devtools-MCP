// runner.go — Constructed ownership and narrow collaborators for bridge mode.
// Purpose: Keeps bridge dependencies instance-scoped and explicit.

package bridge

import (
	"encoding/json"
	"io"

	internbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
)

// Identity describes the stable MCP identity exposed by one runner.
type Identity struct {
	Version            string
	ServerName         string
	ServerInstructions string
}

// Transport owns diagnostic and MCP transport output.
type Transport struct {
	MaxBodySize int64
	Stderrf     func(string, ...any)
	Debugf      func(string, ...any)
	Write       func([]byte, internbridge.StdioFraming)
	Sync        func()
	SetStderr   func(io.Writer)
}

// Protocol owns negotiated MCP state and fast-path content.
type Protocol struct {
	GetFraming          func() internbridge.StdioFraming
	StoreFraming        func(internbridge.StdioFraming)
	SetCapabilities     func(push.ClientCapabilities)
	ExtractCapabilities func(json.RawMessage) push.ClientCapabilities
	NegotiateVersion    func(json.RawMessage) string
	Resources           func() []mcp.MCPResource
	ResourceTemplates   func() []any
	ResolveResource     func(string) (string, string, bool)
}

// Lifecycle owns process operations needed to start and recycle the daemon.
type Lifecycle struct {
	ProcessArgv0         func(string) string
	StopServerForUpgrade func(int) bool
	FindProcessOnPort    func(int) ([]int, error)
	IsProcessAlive       func(int) bool
	AppendExitDiagnostic func(string, map[string]any) string
}

// Runner owns a bridge session and its change-coupled collaborators.
type Runner struct {
	identity  Identity
	transport Transport
	protocol  Protocol
	lifecycle Lifecycle
}

// NewRunner constructs an independent bridge runtime.
func NewRunner(identity Identity, transport Transport, protocol Protocol, lifecycle Lifecycle) *Runner {
	return &Runner{identity: identity, transport: transport, protocol: protocol, lifecycle: lifecycle}
}
