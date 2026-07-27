// Purpose: Dispatches observe tool modes and coordinates alias resolution, validation, and response augmentation.
// Why: Keeps observe entrypoint behavior explicit while mode registry and response helpers live in focused companion files.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolobserve"
)

// observeAliasParams references the shared default mode/action aliases.
var observeAliasParams = defaultModeActionAliases

// observeRegistry is the tool registry for observe dispatch.
var observeRegistry = toolRegistry{
	Handlers:  observeHandlers,
	AliasDefs: observeAliasParams,
	Resolution: modeResolution{
		ToolName:     "observe",
		ValidModes:   "", // populated lazily
		ValueAliases: observeValueAliases,
	},
	PreDispatch: func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage, _ string) (json.RawMessage, *JSONRPCResponse) {
		return h.maybeInjectSummary(args), nil
	},
	PostDispatch: func(h *ToolHandler, req JSONRPCRequest, resp JSONRPCResponse, what string) JSONRPCResponse {
		// Warn when extension is disconnected (except for server-side modes that don't need it)
		if !h.IsExtensionConnected() && !toolobserve.ServerSideObserveModes[what] {
			resp = toolobserve.PrependDisconnectWarning(resp)
		}
		// Piggyback alerts: append as second content block if any pending
		if alerts := h.drainAlerts(); len(alerts) > 0 {
			resp = toolobserve.AppendAlertsToResponse(resp, alerts)
		}
		return resp
	},
}

// toolObserve dispatches observe requests based on the 'what' parameter.
func (h *ToolHandler) toolObserve(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	reg := observeRegistry
	reg.Resolution.ValidModes = getValidObserveModes()
	return h.dispatchTool(req, args, reg)
}

func (h *ToolHandler) toolObserveInbox(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return toolobserve.HandleInbox(h, req, args)
}

func (h *ToolHandler) appendPushPiggyback(resp JSONRPCResponse) JSONRPCResponse {
	return toolobserve.AppendPushPiggyback(h, resp)
}

func (h *ToolHandler) prependDisconnectWarning(resp JSONRPCResponse) JSONRPCResponse {
	return toolobserve.PrependDisconnectWarning(resp)
}

func (h *ToolHandler) appendAlertsToResponse(resp JSONRPCResponse, alerts []Alert) JSONRPCResponse {
	return toolobserve.AppendAlertsToResponse(resp, alerts)
}
