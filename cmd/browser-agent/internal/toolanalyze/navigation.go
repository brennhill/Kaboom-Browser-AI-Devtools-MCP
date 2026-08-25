// navigation.go — Implements analyze modes for navigation discovery and page structure analysis.
// Why: Keeps queued DOM-structure analysis handlers separate from inspect and visual flows.
// Docs: docs/features/feature/analyze-tool/index.md

package toolanalyze

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// enqueueTabScopedAnalysis parses the shared tab_id argument, queues the named
// analyze query, and returns the blocked-enqueue or wait response.
func enqueueTabScopedAnalysis(d Deps, req mcp.JSONRPCRequest, args json.RawMessage, queryType, queuedMsg string) mcp.JSONRPCResponse {
	var params struct {
		TabID int `json:"tab_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
		}
	}

	correlationID := toolresp.NewCorrelationID(queryType)
	query := queries.PendingQuery{
		Type:          queryType,
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := d.EnqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	return d.MaybeWaitForCommand(req, correlationID, args, queuedMsg)
}

// HandleNavigation handles analyze(what="navigation") — SPA route discovery.
func HandleNavigation(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return enqueueTabScopedAnalysis(d, req, args, "navigation", "Navigation discovery queued")
}

// HandlePageStructure handles analyze(what="page_structure").
func HandlePageStructure(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return enqueueTabScopedAnalysis(d, req, args, "page_structure", "Page structure analysis queued")
}

// HandleLinkHealth handles analyze(what="link_health").
func HandleLinkHealth(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	correlationID := toolresp.NewCorrelationID("link_health")
	query := queries.PendingQuery{
		Type:          "link_health",
		Params:        args,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := d.EnqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	return d.MaybeWaitForCommand(req, correlationID, args, "Link health check initiated")
}
