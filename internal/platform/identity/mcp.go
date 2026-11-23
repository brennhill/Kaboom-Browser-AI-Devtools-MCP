// Purpose: Canonical MCP server identity shared across runtime surfaces.
// Why: Prevents drift between daemon/server identity and emitted notification logger fields.

package identity

const (
	// MCPServerName is the canonical MCP server identity shown to clients and LLMs.
	MCPServerName = "kaboom-browser-devtools"
)
