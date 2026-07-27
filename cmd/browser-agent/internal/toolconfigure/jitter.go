// jitter.go — Configures randomized micro-delays (action jitter) before interact actions.
// Why: Prevents bot-detection by adding configurable random delays that simulate natural user input cadence.

package toolconfigure

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// HandleActionJitter handles configure(what="action_jitter").
func HandleActionJitter(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		ActionJitterMs *int `json:"action_jitter_ms"`
	}
	mcp.LenientUnmarshal(args, &params)

	if params.ActionJitterMs != nil {
		v := *params.ActionJitterMs
		if v < 0 {
			v = 0
		}
		if v > 5000 {
			v = 5000
		}
		d.InteractActionSetJitter(v)
	}
	actionMs := d.InteractActionGetJitter()

	result := map[string]any{
		"action_jitter_ms": actionMs,
	}
	return mcp.Succeed(req, "Action jitter configured", result)
}
