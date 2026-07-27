// tools_configure_clear.go — Clears selected runtime capture and server buffers.

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func (h *ToolHandler) toolConfigureClear(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Buffer string `json:"buffer"`
	}
	if len(args) > 0 {
		if resp, stop := mcp.ParseArgs(req, args, &params); stop {
			return resp
		}
	}

	buffer := params.Buffer
	if buffer == "" {
		buffer = "all"
	}

	cleared, ok := h.clearConfiguredBuffer(buffer)
	if !ok {
		return mcp.Fail(req, ErrInvalidParam, "Unknown buffer: "+buffer, "Use a valid buffer value", withParam("buffer"), withHint("all, network, websocket, actions, logs, inbox"))
	}

	responseData := map[string]any{"status": "ok", "buffer": buffer, "cleared": cleared}
	return mcp.Succeed(req, "Buffer cleared", responseData)
}

// clearConfiguredBuffer performs the actual buffer clearing and returns what was cleared.
// Returns (cleared, true) on success, or (nil, false) for an unknown buffer name.
func (h *ToolHandler) clearConfiguredBuffer(buffer string) (any, bool) {
	switch buffer {
	case "all":
		// ClearAll now clears extension logs too and returns the count.
		extensionLogsCleared := h.capture.ClearAll()
		h.server.logs.ClearEntries()
		cleared := map[string]any{
			"buffers":                "all",
			"extension_logs_cleared": extensionLogsCleared,
		}
		if h.server.pushInbox != nil {
			drained := h.server.pushInbox.DrainAll()
			cleared["push_events_drained"] = len(drained)
		}
		if h.annotationStore != nil {
			annotationCleared := h.annotationStore.ClearAll()
			cleared["annotations_cleared"] = map[string]int{
				"sessions":       annotationCleared.Sessions,
				"details":        annotationCleared.Details,
				"named_sessions": annotationCleared.NamedSessions,
				"waiters":        annotationCleared.Waiters,
			}
		}
		return cleared, true
	case "network":
		counts := h.capture.ClearNetworkBuffers()
		return map[string]int{"waterfall": counts.NetworkWaterfall, "bodies": counts.NetworkBodies}, true
	case "websocket":
		counts := h.capture.ClearWebSocketBuffers()
		return map[string]int{"events": counts.WebSocketEvents, "connections": counts.WebSocketStatus}, true
	case "actions":
		counts := h.capture.ClearActionBuffer()
		return map[string]int{"actions": counts.Actions}, true
	case "logs":
		logCount := h.server.logs.EntryCount()
		h.server.logs.ClearEntries()
		return map[string]int{"logs": logCount}, true
	case "inbox":
		if h.server.pushInbox != nil {
			drained := h.server.pushInbox.DrainAll()
			return map[string]int{"push_events": len(drained)}, true
		}
		return map[string]int{"push_events": 0}, true
	default:
		return nil, false
	}
}
