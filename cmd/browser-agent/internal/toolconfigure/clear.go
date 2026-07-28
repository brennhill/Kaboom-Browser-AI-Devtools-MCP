// clear.go — Clears selected runtime capture and server buffers.

package toolconfigure

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
)

// ClearTargets are the stores affected by configure(what="clear").
type ClearTargets struct {
	Capture     *capture.Capture
	ClearLogs   func() int
	Inbox       *push.PushInbox
	Annotations *annotation.Store
}

// HandleClear parses a clear request and clears the selected target.
func HandleClear(targets ClearTargets, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Buffer string `json:"buffer"`
	}
	if len(args) > 0 {
		if response, stop := mcp.ParseArgs(req, args, &params); stop {
			return response
		}
	}
	if params.Buffer == "" {
		params.Buffer = "all"
	}
	cleared, ok := clearBuffer(targets, params.Buffer)
	if !ok {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Unknown buffer: "+params.Buffer, "Use a valid buffer value", mcp.WithParam("buffer"), mcp.WithHint("all, network, websocket, actions, logs, inbox"))
	}
	return mcp.Succeed(req, "Buffer cleared", map[string]any{"status": "ok", "buffer": params.Buffer, "cleared": cleared})
}

func clearBuffer(targets ClearTargets, buffer string) (any, bool) {
	switch buffer {
	case "all":
		extensionLogsCleared := targets.Capture.ClearAll()
		_ = targets.ClearLogs()
		cleared := map[string]any{"buffers": "all", "extension_logs_cleared": extensionLogsCleared}
		if targets.Inbox != nil {
			cleared["push_events_drained"] = len(targets.Inbox.DrainAll())
		}
		if targets.Annotations != nil {
			counts := targets.Annotations.ClearAll()
			cleared["annotations_cleared"] = map[string]int{
				"sessions": counts.Sessions, "details": counts.Details,
				"named_sessions": counts.NamedSessions, "waiters": counts.Waiters,
			}
		}
		return cleared, true
	case "network":
		counts := targets.Capture.Telemetry().ClearNetworkBuffers()
		return map[string]int{"waterfall": counts.NetworkWaterfall, "bodies": counts.NetworkBodies}, true
	case "websocket":
		counts := targets.Capture.Telemetry().ClearWebSocketBuffers()
		return map[string]int{"events": counts.WebSocketEvents, "connections": counts.WebSocketStatus}, true
	case "actions":
		counts := targets.Capture.Telemetry().ClearActionBuffer()
		return map[string]int{"actions": counts.Actions}, true
	case "logs":
		return map[string]int{"logs": targets.ClearLogs()}, true
	case "inbox":
		if targets.Inbox == nil {
			return map[string]int{"push_events": 0}, true
		}
		return map[string]int{"push_events": len(targets.Inbox.DrainAll())}, true
	default:
		return nil, false
	}
}
