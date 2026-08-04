// performance_trace.go — Queues Chrome CPU flamechart trace lifecycle commands.
// Docs: docs/features/feature/performance-trace/index.md

package analyzedispatch

import (
	"encoding/json"
	"fmt"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func HandlePerformanceTrace(d toolanalyze.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Action string `json:"action"`
		TabID  int    `json:"tab_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}
	if params.Action != "start" && params.Action != "stop" {
		return mcp.Fail(req, mcp.ErrInvalidParam, "performance_trace requires action=start or action=stop", "Choose one supported trace lifecycle action")
	}

	correlationID := toolresp.NewCorrelationID("performance-trace")
	query := queries.PendingQuery{Type: "performance_trace", Params: args, TabID: params.TabID, CorrelationID: correlationID}
	if enqueueResp, blocked := d.EnqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}
	return d.MaybeWaitForCommand(req, correlationID, args, fmt.Sprintf("Performance trace %s queued", params.Action))
}
