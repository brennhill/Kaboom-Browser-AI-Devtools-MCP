// response.go — Observe response decoration helpers (disconnect warnings + alert blocks).
// Why: Keeps post-dispatch response shaping reusable and independent from mode parsing/dispatch.

package toolobserve

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/streaming/alertbuf"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// PrependDisconnectWarning adds a warning to the first content block when the extension is disconnected.
func PrependDisconnectWarning(resp mcp.JSONRPCResponse) mcp.JSONRPCResponse {
	warning := "⚠ Extension is not connected — results may be stale or empty. Ensure the Kaboom extension shows 'Connected' and a tab is tracked.\n\n"
	return mcp.PrependWarningToResponse(resp, warning)
}

// AppendAlertsToResponse adds an alerts content block to an existing MCP response.
func AppendAlertsToResponse(resp mcp.JSONRPCResponse, alerts []types.Alert) mcp.JSONRPCResponse {
	return mcp.MutateToolResult(resp, func(r *mcp.MCPToolResult) {
		r.Content = append(r.Content, mcp.MCPContentBlock{
			Type: "text",
			Text: alertbuf.FormatAlertsBlock(alerts),
		})
	})
}
