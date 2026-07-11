// page_inventory.go — observe(what:"page_inventory") handler.
// Why: Returns combined page info and interactive elements in a single extension query.

package observehandler

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// HandlePageInventory handles observe(what="page_inventory").
// Creates a pending query for the extension to return combined page info
// and interactive elements in one response.
func HandlePageInventory(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		TabID       int  `json:"tab_id"`
		VisibleOnly bool `json:"visible_only"`
		Limit       int  `json:"limit"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
		}
	}

	correlationID := newCorrelationID("page_inventory")

	query := queries.PendingQuery{
		Type:          "page_inventory",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := d.EnqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	return d.MaybeWaitForCommand(req, correlationID, args, "Page inventory queued")
}
