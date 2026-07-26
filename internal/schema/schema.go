// Purpose: Assembles the MCP tools/list response from the five per-tool schema definitions.

package schema

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema/configure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema/interact"
)

// AllTools returns all MCP tool definitions.
func AllTools() []mcp.MCPTool {
	return []mcp.MCPTool{
		observeToolSchema(),
		analyzeToolSchema(),
		generateToolSchema(),
		configure.ToolSchema(),
		interact.ToolSchema(),
	}
}
