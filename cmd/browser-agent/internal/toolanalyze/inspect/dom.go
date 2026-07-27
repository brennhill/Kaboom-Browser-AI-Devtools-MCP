// dom.go — Canonical DOM inspection query construction.
// Docs: docs/features/feature/query-dom/index.md

package inspect

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func HandleDOM(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Selector string `json:"selector"`
		TabID    int    `json:"tab_id"`
	}
	if len(args) > 0 {
		if response, stop := mcp.ParseArgs(req, args, &params); stop {
			return response
		}
	}
	queryArgs := args
	if params.Selector == "" {
		var raw map[string]any
		if json.Unmarshal(args, &raw) != nil || raw == nil {
			raw = make(map[string]any)
		}
		raw["selector"] = "*"
		if marshaled, err := json.Marshal(raw); err == nil {
			queryArgs = marshaled
		}
	}
	correlationID := toolresp.NewCorrelationID("dom")
	query := queries.PendingQuery{
		Type: "dom", Params: queryArgs, TabID: params.TabID, CorrelationID: correlationID,
	}
	if response, blocked := d.EnqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return response
	}
	return d.MaybeWaitForCommand(req, correlationID, queryArgs, "DOM query queued")
}
