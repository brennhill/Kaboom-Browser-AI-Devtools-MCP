// Purpose: The cross-stream observe modes — error_bundles and timeline — that join several buffers on time.
// Why: Both answer "what else was happening when X happened" by windowing errors, logs, network and actions
// together, so they share the tab filters and timestamp parsing that single-stream modes do not need.
// Docs: docs/features/feature/observe/index.md

package timeline

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/core"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"sort"
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
	networkBodies    []types.NetworkBody
	waterfallEntries []types.NetworkWaterfallEntry
	actions          []types.EnhancedAction
	logs             []timedEntry
	windowSeconds    int
}

type errorBundlesParams struct {
	Limit         int    `json:"limit"`
	WindowSeconds int    `json:"window_seconds"`
	URL           string `json:"url"`
	Scope         string `json:"scope"`
	Summary       bool   `json:"summary"`
}

// GetErrorBundles assembles pre-joined debugging context around each recent error.
func GetErrorBundles(deps core.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	params, paramHint := normalizeErrorBundlesParams(args)

	_, trackedTabID, trackedTabURL := deps.Capture.Extension().GetTrackingStatus()
	if params.URL == "" && params.Scope == "current_page" && trackedTabURL != "" {
		params.URL = trackedTabURL
	}

	errors, logs := collectErrorsAndLogs(deps, params.Limit, params.URL, params.Scope, trackedTabID)
	ctx := bundleContextFromCapture(deps.Capture, params, logs)

	bundles := buildBundles(errors, ctx)

	var newestEntry time.Time
	if len(errors) > 0 {
		newestEntry = errors[0].ts
	}

	if params.Summary {
		summaryResp := buildErrorBundlesSummary(bundles, newestEntry, core.BuildResponseMetadata(deps.Capture, newestEntry))
		attachParamHint(summaryResp, paramHint)
		return mcp.Succeed(req, "Error bundles", summaryResp)
	}

	response := map[string]any{
		"bundles":  bundles,
		"count":    len(bundles),
		"metadata": core.BuildResponseMetadata(deps.Capture, newestEntry),
	}
	attachParamHint(response, paramHint)
	if len(bundles) == 0 {
		response["hint"] = hints.ErrorBundles()
	}
	return mcp.Succeed(req, "Error bundles", response)
}

func normalizeErrorBundlesParams(args json.RawMessage) (errorBundlesParams, string) {
	var params errorBundlesParams
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
	return params, paramHint
}

func attachParamHint(response map[string]any, paramHint string) {
	if paramHint != "" {
		response["param_hint"] = paramHint
	}
}

func bundleContextFromCapture(cap *capture.Capture, params errorBundlesParams, logs []timedEntry) bundleContext {
	_, trackedTabID, _ := cap.Extension().GetTrackingStatus()
	networkBodies := cap.Telemetry().NetworkBodies().Snapshot().Bodies
	waterfallEntries := cap.Telemetry().NetworkWaterfall().Entries()
	actions := cap.Telemetry().Actions().Snapshot().Actions

	// Apply scope filtering to context buffers so bundles only include
	// network/action entries from the tracked tab, not global state.
	if params.Scope == "current_page" && trackedTabID != 0 {
		networkBodies = filterNetworkBodiesByTab(networkBodies, trackedTabID)
		waterfallEntries = filterWaterfallByTab(waterfallEntries, trackedTabID, cap)
		actions = filterActionsByTab(actions, trackedTabID)
	}

	return bundleContext{
		networkBodies:    networkBodies,
		waterfallEntries: waterfallEntries,
		actions:          actions,
		logs:             logs,
		windowSeconds:    params.WindowSeconds,
	}
}

// collectErrorsAndLogs extracts errors and logs from the log buffer snapshot.
func collectErrorsAndLogs(deps core.Deps, limit int, urlFilter, scope string, trackedTabID int) ([]timedEntry, []timedEntry) {
	entries, _ := deps.LogEntries()

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
				if !core.ContainsIgnoreCase(entryURL, urlFilter) {
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

func matchNetworkBodies(bodies []types.NetworkBody, start, end time.Time) []map[string]any {
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

func matchWaterfall(entries []types.NetworkWaterfallEntry, start, end time.Time) []map[string]any {
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

func matchActions(actions []types.EnhancedAction, start, end time.Time) []map[string]any {
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
func filterNetworkBodiesByTab(bodies []types.NetworkBody, tabID int) []types.NetworkBody {
	filtered := make([]types.NetworkBody, 0, len(bodies))
	for _, nb := range bodies {
		if nb.TabID == tabID {
			filtered = append(filtered, nb)
		}
	}
	return filtered
}

// filterWaterfallByTab returns only waterfall entries from the tracked page.
// NetworkWaterfallEntry lacks a TabID, so we match on the tracked tab's URL via capture.
func filterWaterfallByTab(entries []types.NetworkWaterfallEntry, tabID int, cap *capture.Capture) []types.NetworkWaterfallEntry {
	_, _, trackedURL := cap.Extension().GetTrackingStatus()
	if trackedURL == "" {
		return entries
	}
	filtered := make([]types.NetworkWaterfallEntry, 0, len(entries))
	for _, w := range entries {
		if w.PageURL != "" && !core.ContainsIgnoreCase(w.PageURL, trackedURL) {
			continue
		}
		filtered = append(filtered, w)
	}
	return filtered
}

// filterActionsByTab returns only actions from the specified tab.
func filterActionsByTab(actions []types.EnhancedAction, tabID int) []types.EnhancedAction {
	filtered := make([]types.EnhancedAction, 0, len(actions))
	for _, a := range actions {
		if a.TabID == tabID {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

type timelineEntry struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Summary   string `json:"summary"`
	Data      any    `json:"data,omitempty"`
}

type timelineIncludes struct {
	actions bool
	errors  bool
	network bool
	ws      bool
}

func parseTimelineIncludes(include []string) timelineIncludes {
	if len(include) == 0 {
		return timelineIncludes{actions: true, errors: true, network: true, ws: true}
	}
	var inc timelineIncludes
	for _, v := range include {
		switch v {
		case "actions":
			inc.actions = true
		case "errors":
			inc.errors = true
		case "network":
			inc.network = true
		case "websocket":
			inc.ws = true
		}
	}
	return inc
}

// GetSessionTimeline returns a merged, time-sorted timeline of all captured events.
func GetSessionTimeline(deps core.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit   int      `json:"limit"`
		Include []string `json:"include"`
		Summary bool     `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)
	if params.Limit <= 0 {
		params.Limit = 50
	}
	if params.Limit > core.MaxObserveLimit {
		params.Limit = core.MaxObserveLimit
	}

	inc := parseTimelineIncludes(params.Include)
	entries := collectTimelineEntries(deps, inc)

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp > entries[j].Timestamp
	})

	if params.Summary {
		summary := buildTimelineSummary(entries)
		summary["metadata"] = core.BuildResponseMetadata(deps.Capture, time.Now())
		return mcp.Succeed(req, "Timeline", summary)
	}

	if len(entries) > params.Limit {
		entries = entries[:params.Limit]
	}

	response := map[string]any{
		"entries":  entries,
		"count":    len(entries),
		"metadata": core.BuildResponseMetadata(deps.Capture, time.Now()),
	}
	if len(entries) == 0 {
		response["hint"] = hints.Timeline()
	}
	return mcp.Succeed(req, "Timeline", response)
}

func collectTimelineEntries(deps core.Deps, inc timelineIncludes) []timelineEntry {
	cap := deps.Capture
	entries := make([]timelineEntry, 0)
	if inc.actions {
		entries = append(entries, collectTimelineActions(cap)...)
	}
	if inc.errors {
		entries = append(entries, collectTimelineErrors(deps)...)
	}
	if inc.network {
		entries = append(entries, collectTimelineNetwork(cap.Telemetry().NetworkWaterfall().Entries())...)
	}
	if inc.ws {
		entries = append(entries, collectTimelineWebSocket(cap.Telemetry().WebSockets().Snapshot().Events)...)
	}
	return entries
}

func collectTimelineActions(cap *capture.Capture) []timelineEntry {
	actions := cap.Telemetry().Actions().Snapshot().Actions
	entries := make([]timelineEntry, 0, len(actions))
	for _, a := range actions {
		ts := time.UnixMilli(a.Timestamp).Format(time.RFC3339Nano)
		selector := ""
		if css, ok := a.Selectors["css"].(string); ok {
			selector = css
		}
		entries = append(entries, timelineEntry{
			Timestamp: ts,
			Type:      "action",
			Summary:   a.Type + " on " + selector,
		})
	}
	return entries
}

func collectTimelineErrors(deps core.Deps) []timelineEntry {
	logEntries, _ := deps.LogEntries()
	entries := make([]timelineEntry, 0)
	for _, entry := range logEntries {
		level, _ := entry["level"].(string)
		if level != "error" {
			continue
		}
		ts := core.LogEntryTimestamp(entry)
		msg, _ := entry["message"].(string)
		if len(msg) > 80 {
			msg = msg[:80] + "..."
		}
		entries = append(entries, timelineEntry{
			Timestamp: ts,
			Type:      "error",
			Summary:   msg,
		})
	}
	return entries
}

func collectTimelineNetwork(networkEntries []types.NetworkWaterfallEntry) []timelineEntry {
	entries := make([]timelineEntry, 0, len(networkEntries))
	for _, n := range networkEntries {
		var ts string
		if !n.Timestamp.IsZero() {
			ts = n.Timestamp.Format(time.RFC3339Nano)
		} else {
			ts = time.Now().Add(-time.Duration(n.StartTime) * time.Millisecond).Format(time.RFC3339Nano)
		}
		entries = append(entries, timelineEntry{
			Timestamp: ts,
			Type:      "network",
			Summary:   n.InitiatorType + " " + n.URL,
		})
	}
	return entries
}

func collectTimelineWebSocket(wsEvents []types.WebSocketEvent) []timelineEntry {
	entries := make([]timelineEntry, 0, len(wsEvents))
	for _, ws := range wsEvents {
		summary := ws.Event
		if ws.Direction != "" {
			summary += " (" + ws.Direction + ")"
		}
		entries = append(entries, timelineEntry{
			Timestamp: ws.Timestamp,
			Type:      "websocket",
			Summary:   summary,
		})
	}
	return entries
}

func buildTimelineSummary(entries []timelineEntry) map[string]any {
	counts := make(map[string]int)
	var first, last string
	for _, e := range entries {
		counts[e.Type]++
		if first == "" || e.Timestamp < first {
			first = e.Timestamp
		}
		if last == "" || e.Timestamp > last {
			last = e.Timestamp
		}
	}
	result := map[string]any{
		"counts_by_type": counts,
		"total":          len(entries),
	}
	if first != "" {
		result["time_range"] = map[string]string{"first": first, "last": last}
	}
	return result
}

// buildErrorBundlesSummary returns {total_bundles, unique_error_messages, newest_entry, metadata}.
func buildErrorBundlesSummary(bundles []map[string]any, newestEntry time.Time, meta core.ResponseMetadata) map[string]any {
	seen := make(map[string]bool)
	messages := make([]string, 0)

	for _, b := range bundles {
		errMap, ok := b["error"].(map[string]any)
		if !ok {
			continue
		}
		msg, _ := errMap["message"].(string)
		if msg != "" && !seen[msg] {
			seen[msg] = true
			messages = append(messages, msg)
		}
	}

	result := map[string]any{
		"total_bundles":         len(bundles),
		"unique_error_messages": messages,
		"metadata":              meta,
	}
	if !newestEntry.IsZero() {
		result["newest_entry"] = newestEntry.Format(time.RFC3339)
	}
	return result
}
