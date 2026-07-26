// Purpose: Observe handlers for actions, transient UI elements, and pilot/performance status.

package observe

import (
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/buffers"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/hints"
)

// GetEnhancedActions returns captured user actions (clicks, inputs, navigations).
// Supports optional "type" filter to return only actions of a specific type.
func GetEnhancedActions(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit   int    `json:"limit"`
		LastN   int    `json:"last_n"`
		URL     string `json:"url"`
		Type    string `json:"type"`
		Summary bool   `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)
	params.Limit = clampLimit(params.Limit, 100)

	allActions := deps.GetCapture().GetAllEnhancedActions()
	filtered := buffers.ReverseFilterLimit(allActions, func(a capture.EnhancedAction) bool {
		if params.Type != "" && a.Type != params.Type {
			return false
		}
		if params.URL != "" && !ContainsIgnoreCase(a.URL, params.URL) {
			return false
		}
		return true
	}, params.Limit)

	// last_n: slice to only the N most recent entries (already sorted newest-first).
	if params.LastN > 0 && len(filtered) > params.LastN {
		filtered = filtered[:params.LastN]
	}
	var newestTS time.Time
	if len(allActions) > 0 {
		newestTS = time.UnixMilli(allActions[len(allActions)-1].Timestamp)
	}

	responseMeta := BuildResponseMetadata(deps.GetCapture(), newestTS)
	if params.Summary {
		return mcp.Succeed(req, "Enhanced actions", buildActionsSummary(filtered, responseMeta))
	}

	response := map[string]any{
		"entries":  filtered,
		"count":    len(filtered),
		"metadata": responseMeta,
	}
	if len(filtered) == 0 {
		response["hint"] = hints.Actions()
	}
	return mcp.Succeed(req, "Enhanced actions", response)
}

// GetTransients returns captured transient UI elements (toasts, alerts, snackbars).
// Filters enhanced actions for type == "transient" with optional classification and URL filters.
func GetTransients(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit          int    `json:"limit"`
		URL            string `json:"url"`
		Classification string `json:"classification"`
		Summary        bool   `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)

	var paramHint string
	validClassifications := map[string]bool{
		"alert": true, "toast": true, "snackbar": true, "notification": true,
		"tooltip": true, "banner": true, "flash": true,
	}
	if params.Classification != "" && !validClassifications[params.Classification] {
		paramHint = "Unknown classification " + params.Classification + " ignored (using default=all). Valid values: alert, toast, snackbar, notification, tooltip, banner, flash."
		params.Classification = ""
	}

	// Lower default than other handlers (50 vs 100): transients are less frequent than logs/actions.
	// MVP: duration_ms is always 0 — removal tracking is not yet implemented.
	params.Limit = clampLimit(params.Limit, 50)

	allActions := deps.GetCapture().GetAllEnhancedActions()
	filtered := buffers.ReverseFilterLimit(allActions, func(a capture.EnhancedAction) bool {
		if a.Type != "transient" {
			return false
		}
		if params.URL != "" && !ContainsIgnoreCase(a.URL, params.URL) {
			return false
		}
		if params.Classification != "" && a.Classification != params.Classification {
			return false
		}
		return true
	}, params.Limit)

	var newestTS time.Time
	if len(filtered) > 0 {
		newestTS = time.UnixMilli(filtered[0].Timestamp)
	}

	responseMeta := BuildResponseMetadata(deps.GetCapture(), newestTS)
	if params.Summary {
		summaryResp := buildTransientsSummary(filtered, responseMeta)
		if paramHint != "" {
			summaryResp["param_hint"] = paramHint
		}
		return mcp.Succeed(req, "Transient elements", summaryResp)
	}

	response := map[string]any{
		"entries":  filtered,
		"count":    len(filtered),
		"metadata": responseMeta,
	}
	if paramHint != "" {
		response["param_hint"] = paramHint
	}
	if len(filtered) == 0 {
		response["hint"] = hints.Transients(params.Classification)
	}
	return mcp.Succeed(req, "Transient elements", response)
}

// buildTransientsSummary returns {total, by_classification, metadata}.
func buildTransientsSummary(actions []capture.EnhancedAction, meta ResponseMetadata) map[string]any {
	byClassification := make(map[string]int)
	for _, a := range actions {
		cls := a.Classification
		if cls == "" {
			cls = "unknown"
		}
		byClassification[cls]++
	}

	return map[string]any{
		"total":             len(actions),
		"by_classification": byClassification,
		"metadata":          meta,
	}
}

// ObservePilot returns the current pilot/extension connection status.
func ObservePilot(deps Deps, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	status := deps.GetCapture().GetPilotStatus()
	if statusMap, ok := status.(map[string]any); ok {
		statusMap["metadata"] = BuildResponseMetadata(deps.GetCapture(), time.Now())
	}
	return mcp.Succeed(req, "Pilot status", status)
}

// CheckPerformance returns performance snapshots from the capture buffer.
func CheckPerformance(deps Deps, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	snapshots := deps.GetCapture().GetPerformanceSnapshots()
	return mcp.Succeed(req, "Performance", map[string]any{
		"snapshots": snapshots,
		"count":     len(snapshots),
	})
}
