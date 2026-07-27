// Purpose: Dispatches observe tool modes and coordinates alias resolution, validation, and response augmentation.
// Why: Keeps observe entrypoint behavior explicit while mode registry and response helpers live in focused companion files.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolobserve"
	observe "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"
)

func obs(fn func(observe.Deps, JSONRPCRequest, json.RawMessage) JSONRPCResponse) ModeHandler {
	return func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return fn(h, req, args)
	}
}

func obsLocal(fn func(toolobserve.Deps, JSONRPCRequest, json.RawMessage) JSONRPCResponse) ModeHandler {
	return func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return fn(h, req, args)
	}
}

var observeHandlers = map[string]ModeHandler{
	"errors": obs(observe.GetBrowserErrors), "logs": obs(observe.GetBrowserLogs),
	"extension_logs": obs(observe.GetExtensionLogs), "network_waterfall": obs(observe.GetNetworkWaterfall),
	"network_bodies": obs(observe.GetNetworkBodies), "websocket_events": obs(observe.GetWSEvents),
	"websocket_status": obs(observe.GetWSStatus), "actions": obs(observe.GetEnhancedActions),
	"vitals": obs(observe.GetWebVitals), "page": obs(observe.GetPageInfo), "tabs": obs(observe.GetTabs),
	"history": obs(observe.AnalyzeHistory), "pilot": obs(observe.ObservePilot),
	"timeline": obs(observe.GetSessionTimeline), "error_bundles": obs(observe.GetErrorBundles),
	"screenshot": obs(observe.GetScreenshot), "storage": obs(observe.GetStorage),
	"indexeddb": obs(observe.GetIndexedDB), "summarized_logs": obs(observe.GetSummarizedLogs),
	"transients":        obs(observe.GetTransients),
	"annotations":       method((*ToolHandler).toolGetAnnotations),
	"annotation_detail": method((*ToolHandler).toolGetAnnotationDetail),
	"draw_history":      method((*ToolHandler).toolListDrawHistory),
	"draw_session":      method((*ToolHandler).toolGetDrawSession),
	"page_inventory":    obsLocal(toolobserve.HandlePageInventory), "inbox": obsLocal(toolobserve.HandleInbox),
	"site_menus":        obsLocal(toolobserve.HandleSiteMenus),
	"command_result":    method((*ToolHandler).toolObserveCommandResult),
	"pending_commands":  method((*ToolHandler).toolObservePendingCommands),
	"failed_commands":   method((*ToolHandler).toolObserveFailedCommands),
	"saved_videos":      method((*ToolHandler).toolObserveSavedVideos),
	"recordings":        method((*ToolHandler).toolGetRecordings),
	"recording_actions": method((*ToolHandler).toolGetRecordingActions),
	"playback_results":  method((*ToolHandler).toolGetPlaybackResults),
	"log_diff_report":   method((*ToolHandler).toolGetLogDiffReport),
}

var observeValueAliases = map[string]modeValueAlias{
	"network": {Canonical: "network_waterfall", DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
	"ws":      {Canonical: "websocket_events", DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
}

var serverSideObserveModes = toolobserve.ServerSideObserveModes

func getValidObserveModes() string { return sortedMapKeys(observeHandlers) }

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
