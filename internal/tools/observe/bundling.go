// Purpose: Assembles error bundles by correlating console errors with surrounding network/action context.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/hints"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// timedEntry pairs a log entry with its parsed timestamp.
type timedEntry struct {
	data map[string]any
	ts   time.Time
}

// bundleContext holds all data sources needed for window-joining bundles.
type bundleContext struct {
	networkBodies    []capture.NetworkBody
	waterfallEntries []capture.NetworkWaterfallEntry
	actions          []capture.EnhancedAction
	logs             []timedEntry
	windowSeconds    int
}

// GetErrorBundles assembles pre-joined debugging context around each recent error.
func GetErrorBundles(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit         int    `json:"limit"`
		WindowSeconds int    `json:"window_seconds"`
		URL           string `json:"url"`
		Scope         string `json:"scope"`
		Summary       bool   `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)
	if params.Scope == "" {
		params.Scope = "current_page"
	}
	var paramHint string
	if params.Scope != "current_page" && params.Scope != "all" {
		paramHint = "Unknown scope " + params.Scope + " ignored (using default=current_page). Valid values: current_page, all."
		params.Scope = "current_page"
	}
	if params.Limit <= 0 {
		params.Limit = 5
	}
	if params.WindowSeconds <= 0 {
		params.WindowSeconds = 3
	}
	if params.WindowSeconds > 10 {
		params.WindowSeconds = 10
	}
	if params.Scope == "" {
		params.Scope = "current_page"
	}

	_, trackedTabID, trackedTabURL := deps.GetCapture().GetTrackingStatus()
	if params.URL == "" && params.Scope == "current_page" && trackedTabURL != "" {
		params.URL = trackedTabURL
	}

	errors, logs := collectErrorsAndLogs(deps, params.Limit, params.URL, params.Scope, trackedTabID)

	cap := deps.GetCapture()
	_, trackedTabID, _ = cap.GetTrackingStatus()

	networkBodies := cap.GetNetworkBodies()
	waterfallEntries := cap.GetNetworkWaterfallEntries()
	actions := cap.GetAllEnhancedActions()

	// Apply scope filtering to context buffers so bundles only include
	// network/action entries from the tracked tab, not global state.
	if params.Scope == "current_page" && trackedTabID != 0 {
		networkBodies = filterNetworkBodiesByTab(networkBodies, trackedTabID)
		waterfallEntries = filterWaterfallByTab(waterfallEntries, trackedTabID, cap)
		actions = filterActionsByTab(actions, trackedTabID)
	}

	ctx := bundleContext{
		networkBodies:    networkBodies,
		waterfallEntries: waterfallEntries,
		actions:          actions,
		logs:             logs,
		windowSeconds:    params.WindowSeconds,
	}

	bundles := buildBundles(errors, ctx)

	var newestEntry time.Time
	if len(errors) > 0 {
		newestEntry = errors[0].ts
	}

	if params.Summary {
		summaryResp := buildErrorBundlesSummary(bundles, newestEntry, BuildResponseMetadata(cap, newestEntry))
		if paramHint != "" {
			summaryResp["param_hint"] = paramHint
		}
		return mcp.Succeed(req, "Error bundles", summaryResp)
	}

	response := map[string]any{
		"bundles":  bundles,
		"count":    len(bundles),
		"metadata": BuildResponseMetadata(cap, newestEntry),
	}
	if paramHint != "" {
		response["param_hint"] = paramHint
	}
	if len(bundles) == 0 {
		response["hint"] = hints.ErrorBundles()
	}
	return mcp.Succeed(req, "Error bundles", response)
}

// collectErrorsAndLogs extracts errors and logs from the log buffer snapshot.
func collectErrorsAndLogs(deps Deps, limit int, urlFilter, scope string, trackedTabID int) ([]timedEntry, []timedEntry) {
	entries, _ := deps.GetLogEntries()

	var errors, logs []timedEntry
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		ts := parseEntryTimestamp(entry)
		if ts.IsZero() {
			continue
		}
		entryType, _ := entry["type"].(string)
		if entryType == "lifecycle" || entryType == "tracking" || entryType == "extension" {
			continue
		}
		if scope == "current_page" && trackedTabID != 0 {
			entryTabID, _ := entry["tabId"].(float64)
			if int(entryTabID) != trackedTabID {
				continue
			}
		}
		level, _ := entry["level"].(string)
		if level == "error" {
			if urlFilter != "" {
				entryURL, _ := entry["url"].(string)
				if !ContainsIgnoreCase(entryURL, urlFilter) {
					continue
				}
			}
			if len(errors) < limit {
				errors = append(errors, timedEntry{data: entry, ts: ts})
			}
		} else {
			logs = append(logs, timedEntry{data: entry, ts: ts})
		}
	}
	return errors, logs
}

// parseEntryTimestamp parses the timestamp from a log entry using util.ParseTimestamp.
// Checks both "timestamp" (daemon-generated entries) and "ts" (extension-generated entries).
func parseEntryTimestamp(entry map[string]any) time.Time {
	tsStr, _ := entry["timestamp"].(string)
	if tsStr == "" {
		tsStr, _ = entry["ts"].(string)
	}
	return util.ParseTimestamp(tsStr)
}

// buildBundles creates a debugging bundle for each error by window-joining related data.
func buildBundles(errors []timedEntry, ctx bundleContext) []map[string]any {
	window := time.Duration(ctx.windowSeconds) * time.Second
	bundles := make([]map[string]any, 0, len(errors))

	for _, e := range errors {
		windowStart := e.ts.Add(-window)
		bundles = append(bundles, map[string]any{
			"error":                  errorEntryToMap(e.data),
			"network":                matchNetworkBodies(ctx.networkBodies, windowStart, e.ts),
			"waterfall":              matchWaterfall(ctx.waterfallEntries, windowStart, e.ts),
			"actions":                matchActions(ctx.actions, windowStart, e.ts),
			"logs":                   matchLogs(ctx.logs, windowStart, e.ts),
			"context_window_seconds": ctx.windowSeconds,
		})
	}
	return bundles
}

func errorEntryToMap(data map[string]any) map[string]any {
	return map[string]any{
		"message": data["message"], "source": data["source"],
		"url": data["url"], "line": data["line"],
		"column": data["column"], "stack": data["stack"],
		"timestamp": data["timestamp"],
	}
}

func matchNetworkBodies(bodies []capture.NetworkBody, start, end time.Time) []map[string]any {
	matched := make([]map[string]any, 0)
	for _, nb := range bodies {
		nbTs := util.ParseTimestamp(nb.Timestamp)
		if nbTs.IsZero() || !nbTs.After(start) || nbTs.After(end) {
			continue
		}
		matched = append(matched, map[string]any{
			"method": nb.Method, "url": nb.URL, "status": nb.Status,
			"duration": nb.Duration, "content_type": nb.ContentType,
			"response_body": nb.ResponseBody, "timestamp": nb.Timestamp,
		})
	}
	return matched
}

func matchWaterfall(entries []capture.NetworkWaterfallEntry, start, end time.Time) []map[string]any {
	matched := make([]map[string]any, 0)
	for _, w := range entries {
		if w.Timestamp.IsZero() || !w.Timestamp.After(start) || w.Timestamp.After(end) {
			continue
		}
		matched = append(matched, map[string]any{
			"url": w.URL, "initiator_type": w.InitiatorType,
			"duration_ms": w.Duration, "transfer_size": w.TransferSize,
			"timestamp": w.Timestamp.Format(time.RFC3339),
		})
	}
	return matched
}

func matchActions(actions []capture.EnhancedAction, start, end time.Time) []map[string]any {
	matched := make([]map[string]any, 0)
	for _, a := range actions {
		aTs := time.UnixMilli(a.Timestamp)
		if !aTs.After(start) || aTs.After(end) {
			continue
		}
		actionMap := map[string]any{"type": a.Type, "timestamp": aTs.Format(time.RFC3339)}
		if a.URL != "" {
			actionMap["url"] = a.URL
		}
		if css, ok := a.Selectors["css"].(string); ok {
			actionMap["selector"] = css
		}
		if a.Value != "" {
			actionMap["value"] = a.Value
		}
		matched = append(matched, actionMap)
	}
	return matched
}

func matchLogs(logs []timedEntry, start, end time.Time) []map[string]any {
	matched := make([]map[string]any, 0)
	for _, l := range logs {
		if !l.ts.After(start) || l.ts.After(end) {
			continue
		}
		matched = append(matched, map[string]any{
			"level": l.data["level"], "message": l.data["message"],
			"timestamp": l.data["timestamp"],
		})
	}
	return matched
}

// filterNetworkBodiesByTab returns only network bodies from the specified tab.
func filterNetworkBodiesByTab(bodies []capture.NetworkBody, tabID int) []capture.NetworkBody {
	filtered := make([]capture.NetworkBody, 0, len(bodies))
	for _, nb := range bodies {
		if nb.TabID == tabID {
			filtered = append(filtered, nb)
		}
	}
	return filtered
}

// filterWaterfallByTab returns only waterfall entries from the tracked page.
// NetworkWaterfallEntry lacks a TabID, so we match on the tracked tab's URL via capture.
func filterWaterfallByTab(entries []capture.NetworkWaterfallEntry, tabID int, cap *capture.Capture) []capture.NetworkWaterfallEntry {
	_, _, trackedURL := cap.GetTrackingStatus()
	if trackedURL == "" {
		return entries
	}
	filtered := make([]capture.NetworkWaterfallEntry, 0, len(entries))
	for _, w := range entries {
		if w.PageURL != "" && !ContainsIgnoreCase(w.PageURL, trackedURL) {
			continue
		}
		filtered = append(filtered, w)
	}
	return filtered
}

// filterActionsByTab returns only actions from the specified tab.
func filterActionsByTab(actions []capture.EnhancedAction, tabID int) []capture.EnhancedAction {
	filtered := make([]capture.EnhancedAction, 0, len(actions))
	for _, a := range actions {
		if a.TabID == tabID {
			filtered = append(filtered, a)
		}
	}
	return filtered
}
