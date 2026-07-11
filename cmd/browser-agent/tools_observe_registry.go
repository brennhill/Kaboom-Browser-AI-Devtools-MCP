// Purpose: Central registry for observe mode handlers and mode metadata.
// Why: Keeps mode definitions discoverable in one place and decouples registry updates from dispatch logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/observehandler"
)

// obs wraps an observe.Deps-accepting function as a ModeHandler.
// *ToolHandler satisfies observe.Deps, but Go requires explicit adaptation.
func obs(fn func(observe.Deps, JSONRPCRequest, json.RawMessage) JSONRPCResponse) ModeHandler {
	return func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return fn(h, req, args)
	}
}

// obsLocal wraps a observehandler.Deps-accepting function as a ModeHandler.
func obsLocal(fn func(observehandler.Deps, JSONRPCRequest, json.RawMessage) JSONRPCResponse) ModeHandler {
	return func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return fn(h, req, args)
	}
}

// observeHandlers maps observe mode names to their handler functions.
var observeHandlers = map[string]ModeHandler{
	// Delegated to internal/tools/observe
	"errors":            obs(observe.GetBrowserErrors),
	"logs":              obs(observe.GetBrowserLogs),
	"extension_logs":    obs(observe.GetExtensionLogs),
	"network_waterfall": obs(observe.GetNetworkWaterfall),
	"network_bodies":    obs(observe.GetNetworkBodies),
	"websocket_events":  obs(observe.GetWSEvents),
	"websocket_status":  obs(observe.GetWSStatus),
	"actions":           obs(observe.GetEnhancedActions),
	"vitals":            obs(observe.GetWebVitals),
	"page":              obs(observe.GetPageInfo),
	"tabs":              obs(observe.GetTabs),
	"history":           obs(observe.AnalyzeHistory),
	"pilot":             obs(observe.ObservePilot),
	"timeline":          obs(observe.GetSessionTimeline),
	"error_bundles":     obs(observe.GetErrorBundles),
	"screenshot":        obs(observe.GetScreenshot),
	"storage":           obs(observe.GetStorage),
	"indexeddb":         obs(observe.GetIndexedDB),
	"summarized_logs":   obs(observe.GetSummarizedLogs),
	"transients":        obs(observe.GetTransients),
	// Annotations (canonical home; also available via analyze for backwards compat)
	"annotations":       method((*ToolHandler).toolGetAnnotations),
	"annotation_detail": method((*ToolHandler).toolGetAnnotationDetail),
	"draw_history":      method((*ToolHandler).toolListDrawHistory),
	"draw_session":      method((*ToolHandler).toolGetDrawSession),
	// Delegated to cmd/browser-agent/internal/observehandler
	"page_inventory": obsLocal(observehandler.HandlePageInventory),
	"inbox":          obsLocal(observehandler.HandleInbox),
	"site_menus":     obsLocal(observehandler.HandleSiteMenus),
	// Local handlers (ToolHandler-dependent)
	"command_result":    method((*ToolHandler).toolObserveCommandResult),
	"pending_commands":  method((*ToolHandler).toolObservePendingCommands),
	"failed_commands":   method((*ToolHandler).toolObserveFailedCommands),
	"saved_videos":      method((*ToolHandler).toolObserveSavedVideos),
	"recordings":        method((*ToolHandler).toolGetRecordings),
	"recording_actions": method((*ToolHandler).toolGetRecordingActions),
	"playback_results":  method((*ToolHandler).toolGetPlaybackResults),
	"log_diff_report":   method((*ToolHandler).toolGetLogDiffReport),
}

// observeValueAliases maps shorthand names to their canonical observe mode names with deprecation metadata.
var observeValueAliases = map[string]modeValueAlias{
	"network": {Canonical: "network_waterfall", DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
	"ws":      {Canonical: "websocket_events", DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
}

// serverSideObserveModes is a package-level alias to the extracted registry.
var serverSideObserveModes = observehandler.ServerSideObserveModes

// getValidObserveModes returns a sorted, comma-separated list of valid observe modes.
func getValidObserveModes() string { return sortedMapKeys(observeHandlers) }
