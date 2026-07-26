// Purpose: Implements configure state/session handlers behind thin wrappers.
// Why: Keep the top-level configure router focused on dispatch only.

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
)

func (h *configureSessionHandler) handleConfigureStore(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var compositeArgs struct {
		StoreAction string          `json:"store_action"`
		Action      string          `json:"action"`
		Namespace   string          `json:"namespace"`
		Key         string          `json:"key"`
		Data        json.RawMessage `json:"data"`
		Value       json.RawMessage `json:"value"`
	}
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &compositeArgs); stop {
			return resp
		}
	}

	action := compositeArgs.StoreAction
	if action == "" && isStoreAction(compositeArgs.Action) {
		action = compositeArgs.Action
	}
	if action == "" {
		action = "list"
	}

	namespace := compositeArgs.Namespace
	if namespace == "" {
		namespace = defaultStoreNamespace
	}

	data := compositeArgs.Data
	if len(data) == 0 && len(compositeArgs.Value) > 0 {
		data = compositeArgs.Value
	}

	// Ensure session store is initialized.
	if resp, blocked := h.deps.requireSessionStore(req); blocked {
		return resp
	}

	// Convert to SessionStoreArgs.
	storeArgs := persistence.SessionStoreArgs{
		Action:    action,
		Namespace: namespace,
		Key:       compositeArgs.Key,
		Data:      data,
	}

	result, err := h.sessionStoreImpl.HandleSessionStore(storeArgs)
	if err != nil {
		return fail(req, ErrInvalidParam, err.Error(), "Fix the request parameters and try again")
	}

	// Invalidate summary preference cache when response_mode is written.
	if namespace == "session" && compositeArgs.Key == "response_mode" {
		h.deps.invalidateSummaryPref()
	}

	// Sync active_codebase to server-level field for terminal CWD fallback.
	if compositeArgs.Key == "active_codebase" && action == "save" && h.server != nil {
		var path string
		if err := json.Unmarshal(data, &path); err == nil {
			h.server.SetActiveCodebase(path)
		}
	}

	// Parse result back to map for response.
	var responseData map[string]any
	if err := json.Unmarshal(result, &responseData); err != nil {
		responseData = map[string]any{"raw": string(result)}
	}

	return succeed(req, "Store operation complete", responseData)
}

func (h *configureSessionHandler) handleLoadSessionContext(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// If session store is initialized, use it.
	if h.sessionStoreImpl != nil {
		ctx := h.sessionStoreImpl.LoadSessionContext()
		responseData := map[string]any{
			"status":        "ok",
			"project_id":    ctx.ProjectID,
			"session_count": ctx.SessionCount,
			"baselines":     ctx.Baselines,
			"error_history": ctx.ErrorHistory,
		}
		if ctx.NoiseConfig != nil {
			responseData["noise_config"] = ctx.NoiseConfig
		}
		if ctx.APISchema != nil {
			responseData["api_schema"] = ctx.APISchema
		}
		if ctx.Performance != nil {
			responseData["performance"] = ctx.Performance
		}
		return succeed(req, "Session context loaded", responseData)
	}

	// Session store not initialized — return error, matching store behavior.
	return fail(req, ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")
}

func (h *ToolHandler) toolConfigureClear(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Buffer string `json:"buffer"`
	}
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	buffer := params.Buffer
	if buffer == "" {
		buffer = "all"
	}

	cleared, ok := h.clearConfiguredBuffer(buffer)
	if !ok {
		return fail(req, ErrInvalidParam, "Unknown buffer: "+buffer, "Use a valid buffer value", withParam("buffer"), withHint("all, network, websocket, actions, logs, inbox"))
	}

	responseData := map[string]any{"status": "ok", "buffer": buffer, "cleared": cleared}
	return succeed(req, "Buffer cleared", responseData)
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
