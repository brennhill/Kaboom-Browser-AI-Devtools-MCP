// postprocess.go — Applies shared validation metadata to MCP tool results.

package toolpostprocess

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// Apply appends unknown-parameter warnings to successful tool results and
// reports whether the result represents a tool error.
func Apply(response mcp.JSONRPCResponse, args json.RawMessage, inputSchema map[string]any) (mcp.JSONRPCResponse, bool) {
	var result mcp.MCPToolResult
	if len(response.Result) == 0 || json.Unmarshal(response.Result, &result) != nil {
		return response, false
	}
	if result.IsError {
		return response, true
	}
	warnings := mcp.ValidateParamsAgainstSchema(args, inputSchema)
	if !mcp.AppendWarningsToToolResult(&result, warnings) {
		return response, false
	}
	response.Result = mcp.SafeMarshal(&result, string(response.Result))
	return response, false
}
