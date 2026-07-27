// Purpose: Dispatches observe tool modes and coordinates alias resolution, validation, and response augmentation.
// Why: Keeps observe entrypoint behavior explicit while mode registry and response helpers live in focused companion files.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mediaapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/screenrec"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolobserve"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	observe "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"
	wiretypes "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func obs(fn func(observe.Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse) ModeHandler {
	return func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return fn(h, req, args)
	}
}

func obsLocal(fn func(toolobserve.Deps, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse) ModeHandler {
	return func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
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
	"transients": obs(observe.GetTransients),
	"annotations": func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return h.annotationAnalysis.GetAnnotations(req, args)
	},
	"annotation_detail": func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return h.annotationAnalysis.GetAnnotationDetail(req, args)
	},
	"draw_history": func(_ *ToolHandler, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
		dir, err := mediaapi.ScreenshotsDir()
		return annotation.ListDrawHistory(req, dir, err)
	},
	"draw_session": func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		dir, err := mediaapi.ScreenshotsDir()
		return annotation.LoadDrawSession(h.annotationStore, req, args, dir, err)
	},
	"page_inventory": obsLocal(toolobserve.HandlePageInventory), "inbox": obsLocal(toolobserve.HandleInbox),
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
	PreDispatch: func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage, _ string) (json.RawMessage, *mcp.JSONRPCResponse) {
		return h.maybeInjectSummary(args), nil
	},
	PostDispatch: func(h *ToolHandler, req mcp.JSONRPCRequest, resp mcp.JSONRPCResponse, what string) mcp.JSONRPCResponse {
		// Warn when extension is disconnected (except for server-side modes that don't need it)
		if !h.IsExtensionConnected() && !toolobserve.ServerSideObserveModes[what] {
			resp = toolobserve.PrependDisconnectWarning(resp)
		}
		// Piggyback alerts: append as second content block if any pending
		if alerts := h.alertBuffer.DrainAlerts(); len(alerts) > 0 {
			resp = toolobserve.AppendAlertsToResponse(resp, alerts)
		}
		return resp
	},
}

// toolObserve dispatches observe requests based on the 'what' parameter.
func (h *ToolHandler) toolObserve(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	reg := observeRegistry
	reg.Resolution.ValidModes = getValidObserveModes()
	return h.dispatchTool(req, args, reg)
}

func (h *ToolHandler) toolObserveInbox(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return toolobserve.HandleInbox(h, req, args)
}

func (h *ToolHandler) appendPushPiggyback(resp mcp.JSONRPCResponse) mcp.JSONRPCResponse {
	return toolobserve.AppendPushPiggyback(h, resp)
}

func (h *ToolHandler) prependDisconnectWarning(resp mcp.JSONRPCResponse) mcp.JSONRPCResponse {
	return toolobserve.PrependDisconnectWarning(resp)
}

func (h *ToolHandler) appendAlertsToResponse(resp mcp.JSONRPCResponse, alerts []wiretypes.Alert) mcp.JSONRPCResponse {
	return toolobserve.AppendAlertsToResponse(resp, alerts)
}

const annotationCommandWaitTimeout = 55 * time.Second

func (h *ToolHandler) toolObserveCommandResult(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		CorrelationID string `json:"correlation_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil && len(args) > 0 {
		return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}
	if response, blocked := toolresp.RequireString(req, params.CorrelationID, "correlation_id", "Add the 'correlation_id' parameter and call again"); blocked {
		return response
	}
	correlationID := params.CorrelationID
	if strings.HasPrefix(correlationID, "ann_") {
		command, found := h.capture.WaitForCommand(correlationID, annotationCommandWaitTimeout)
		if !found {
			return mcp.Fail(req, mcp.ErrNoData,
				"Annotation command not found: "+correlationID,
				"The command may have expired (10 min TTL). Start a new draw mode session.",
				mcp.WithFinal(true), h.Guards.DiagnosticHint())
		}
		return h.formatCommandResult(req, *command, correlationID)
	}
	command, found := h.capture.GetCommandResult(correlationID)
	if !found {
		return mcp.Fail(req, mcp.ErrNoData,
			"Command not found: "+correlationID,
			"The command may have already completed and been cleaned up (60s TTL), or the correlation_id is invalid. Use observe with what='pending_commands' to see active commands.",
			mcp.WithFinal(true), h.Guards.DiagnosticHint())
	}
	return h.formatCommandResult(req, *command, correlationID)
}

func (h *ToolHandler) toolObservePendingCommands(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	pending := h.capture.GetPendingCommands()
	completed := h.capture.GetCompletedCommands()
	failed := h.capture.GetFailedCommands()
	inProgress := h.capture.GetInProgressCommands()
	data := map[string]any{
		"pending": pending, "completed": completed, "failed": failed,
		"extension_in_progress": inProgress, "extension_in_progress_count": len(inProgress),
	}
	summary := fmt.Sprintf("Pending: %d, Completed: %d, Failed: %d, Extension in-progress: %d",
		len(pending), len(completed), len(failed), len(inProgress))
	return mcp.Succeed(req, summary, data)
}

func (h *ToolHandler) toolObserveFailedCommands(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	failed := h.capture.GetFailedCommands()
	data := map[string]any{"commands": failed, "count": len(failed)}
	if len(failed) == 0 {
		return mcp.Succeed(req, "No failed commands found", data)
	}
	return mcp.Succeed(req, fmt.Sprintf("Found %d failed/expired commands", len(failed)), data)
}

func (h *ToolHandler) toolObserveSavedVideos(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return screenrec.HandleObserveSavedVideos(req, args)
}

func (h *ToolHandler) toolGetRecordings(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.recordingHandler.Recordings(req, args)
}

func (h *ToolHandler) toolGetRecordingActions(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.recordingHandler.RecordingActions(req, args)
}

func (h *ToolHandler) toolGetPlaybackResults(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.recordingHandler.PlaybackResults(req, args)
}

func (h *ToolHandler) toolGetLogDiffReport(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.recordingHandler.LogDiffReport(req, args)
}
