// react_profile.go — Queues opt-in React profiler lifecycle commands.
// Docs: docs/features/feature/react-performance-profiling/index.md

package analyzedispatch

import (
	"encoding/json"
	"fmt"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func HandleReactProfile(d toolanalyze.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Action string `json:"action"`
		TabID  int    `json:"tab_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}
	if params.Action != "start" && params.Action != "stop" {
		return mcp.Fail(req, mcp.ErrInvalidParam, "react_profile requires action=start or action=stop", "Choose one supported profile lifecycle action")
	}
	correlationID := toolresp.NewCorrelationID("react-profile")
	query := queries.PendingQuery{Type: "react_profile", Params: args, TabID: params.TabID, CorrelationID: correlationID}
	if enqueueResp, blocked := d.EnqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}
	return d.MaybeWaitForCommand(req, correlationID, args, fmt.Sprintf("React profile %s queued", params.Action))
}
