// Purpose: The session-activity observe modes — actions, transients, history, vitals, tabs, pilot — plus analyze's performance check.
// Why: These describe what the user and the page did during the session rather than one telemetry stream,
// and they share the enhanced-action buffer and the tracking status.
// Docs: docs/features/feature/observe/index.md

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

type historyEntry struct {
	Timestamp string `json:"timestamp"`
	FromURL   string `json:"from_url,omitempty"`
	ToURL     string `json:"to_url"`
	Type      string `json:"type"`
}

// AnalyzeHistory extracts navigation history from captured user actions.
func AnalyzeHistory(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit   int  `json:"limit"`
		Summary bool `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)

	actions := deps.GetCapture().GetAllEnhancedActions()
	entries := buildHistoryEntries(actions)
	entries = limitHistoryEntries(entries, clampLimit(params.Limit, 0))

	responseMeta := BuildResponseMetadata(deps.GetCapture(), time.Now())
	if params.Summary {
		return mcp.Succeed(req, "History", buildHistorySummary(entries, responseMeta))
	}

	return mcp.Succeed(req, "History", map[string]any{
		"entries":  entries,
		"count":    len(entries),
		"metadata": responseMeta,
	})
}

func buildHistoryEntries(actions []capture.EnhancedAction) []historyEntry {
	entries := make([]historyEntry, 0)
	seenURLs := make(map[string]bool)

	for _, a := range actions {
		ts := time.UnixMilli(a.Timestamp).Format(time.RFC3339)
		if a.Type == "navigate" && a.ToURL != "" && !seenURLs[a.ToURL] {
			entries = append(entries, historyEntry{Timestamp: ts, FromURL: a.FromURL, ToURL: a.ToURL, Type: "navigate"})
			seenURLs[a.ToURL] = true
		}
		if a.URL != "" && !seenURLs[a.URL] {
			entries = append(entries, historyEntry{Timestamp: ts, ToURL: a.URL, Type: "page_visit"})
			seenURLs[a.URL] = true
		}
	}
	return entries
}

func limitHistoryEntries(entries []historyEntry, limit int) []historyEntry {
	if limit <= 0 || len(entries) <= limit {
		return entries
	}
	return entries[len(entries)-limit:]
}

func GetWebVitals(deps Deps, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	snapshots := deps.GetCapture().GetPerformanceSnapshots()
	vitals := buildVitalsMap(snapshots)
	return mcp.Succeed(req, "Web vitals", map[string]any{
		"metrics":  vitals,
		"metadata": BuildResponseMetadata(deps.GetCapture(), time.Now()),
	})
}

func buildVitalsMap(snapshots []capture.PerformanceSnapshot) map[string]any {
	if len(snapshots) == 0 {
		return map[string]any{"has_data": false}
	}
	latest := snapshots[len(snapshots)-1]
	vitals := map[string]any{
		"has_data":         true,
		"url":              latest.URL,
		"timestamp":        latest.Timestamp,
		"domContentLoaded": latest.Timing.DomContentLoaded,
		"load":             latest.Timing.Load,
	}
	if latest.Timing.LargestContentfulPaint != nil {
		vitals["lcp"] = *latest.Timing.LargestContentfulPaint
	}
	if latest.Timing.FirstContentfulPaint != nil {
		vitals["fcp"] = *latest.Timing.FirstContentfulPaint
	}
	if latest.CLS != nil {
		vitals["cls"] = *latest.CLS
	}
	return vitals
}

// GetTabs returns information about tracked browser tabs.

func GetTabs(deps Deps, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	cap := deps.GetCapture()
	enabled, tabID, tabURL := cap.GetTrackingStatus()

	tabs := []any{}
	if enabled && tabID > 0 {
		tabs = append(tabs, map[string]any{
			"id":      tabID,
			"url":     tabURL,
			"tracked": true,
			"active":  true,
		})
	}

	return mcp.Succeed(req, "Tabs", map[string]any{
		"tabs":            tabs,
		"tracking_active": enabled,
		"metadata":        BuildResponseMetadata(cap, time.Now()),
	})
}

func buildActionsSummary(actions []capture.EnhancedAction, meta ResponseMetadata) map[string]any {
	byType := make(map[string]int)
	var firstTS, lastTS int64
	hasTS := false

	for _, a := range actions {
		byType[a.Type]++
		if !hasTS || a.Timestamp < firstTS {
			firstTS = a.Timestamp
			hasTS = true
		}
		if a.Timestamp > lastTS {
			lastTS = a.Timestamp
		}
	}

	result := map[string]any{
		"total":    len(actions),
		"by_type":  byType,
		"metadata": meta,
	}
	if hasTS {
		result["time_range"] = map[string]string{
			"first": time.UnixMilli(firstTS).Format(time.RFC3339),
			"last":  time.UnixMilli(lastTS).Format(time.RFC3339),
		}
	}
	return result
}

// buildHistorySummary returns {total, by_type, unique_urls, metadata}.

func buildHistorySummary(entries []historyEntry, meta ResponseMetadata) map[string]any {
	byType := make(map[string]int)
	urls := make(map[string]bool)

	for _, e := range entries {
		if e.Type != "" {
			byType[e.Type]++
		}
		if e.ToURL != "" {
			urls[e.ToURL] = true
		}
	}

	return map[string]any{
		"total":       len(entries),
		"by_type":     byType,
		"unique_urls": len(urls),
		"metadata":    meta,
	}
}
