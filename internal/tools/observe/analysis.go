// Purpose: Builds waterfall summaries and lightweight observe diagnostics (WS status, vitals, tabs).
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/buffers"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/hints"
)

const wsStatusSummarySampleLimit = 10

// GetNetworkWaterfall returns network waterfall entries from the performance API.
func GetNetworkWaterfall(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit     int    `json:"limit"`
		URLFilter string `json:"url"`
		Summary   bool   `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)
	params.Limit = clampLimit(params.Limit, 100)

	allEntries := refreshWaterfallIfStale(deps)

	var newestTS time.Time
	if len(allEntries) > 0 {
		newestTS = allEntries[len(allEntries)-1].Timestamp
	}

	if params.Summary {
		entries := filterWaterfallSummaryEntries(allEntries, params.URLFilter, params.Limit)
		return mcp.Succeed(req, "Network waterfall", map[string]any{
			"entries":  entries,
			"count":    len(entries),
			"metadata": BuildResponseMetadata(deps.GetCapture(), newestTS),
		})
	}

	entries := filterWaterfallEntries(allEntries, params.URLFilter, params.Limit)
	response := map[string]any{
		"entries":  entries,
		"count":    len(entries),
		"metadata": BuildResponseMetadata(deps.GetCapture(), newestTS),
	}
	if len(entries) == 0 {
		response["hint"] = hints.NetworkWaterfall(params.URLFilter)
	}
	return mcp.Succeed(req, "Network waterfall", response)
}

func refreshWaterfallIfStale(deps Deps) []capture.NetworkWaterfallEntry {
	cap := deps.GetCapture()
	allEntries := cap.GetNetworkWaterfallEntries()
	if len(allEntries) > 0 && time.Since(allEntries[len(allEntries)-1].Timestamp) < 1*time.Second {
		return allEntries
	}

	queryID, qerr := cap.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "waterfall",
			Params: json.RawMessage(`{}`),
		},
		5*time.Second,
		"",
	)
	if qerr != nil {
		return allEntries
	}

	result, err := cap.WaitForResult(queryID, 5*time.Second)
	if err != nil || result == nil {
		return allEntries
	}

	var waterfallResult struct {
		Entries []capture.NetworkWaterfallEntry `json:"entries"`
		PageURL string                          `json:"page_url"`
	}
	if err := json.Unmarshal(result, &waterfallResult); err == nil && len(waterfallResult.Entries) > 0 {
		cap.AddNetworkWaterfallEntries(waterfallResult.Entries, waterfallResult.PageURL)
		return cap.GetNetworkWaterfallEntries()
	}
	return allEntries
}

func filterWaterfallEntries(allEntries []capture.NetworkWaterfallEntry, urlFilter string, limit int) []map[string]any {
	matched := buffers.ReverseFilterLimit(allEntries, func(entry capture.NetworkWaterfallEntry) bool {
		return urlFilter == "" || (entry.URL != "" && ContainsIgnoreCase(entry.URL, urlFilter))
	}, limit)

	entries := make([]map[string]any, len(matched))
	for i, entry := range matched {
		entries[i] = waterfallEntryToMap(entry)
	}
	return entries
}

func waterfallEntryToMap(entry capture.NetworkWaterfallEntry) map[string]any {
	return map[string]any{
		"url":               entry.URL,
		"initiator_type":    entry.InitiatorType,
		"duration_ms":       entry.Duration,
		"start_time":        entry.StartTime,
		"transfer_size":     entry.TransferSize,
		"decoded_body_size": entry.DecodedBodySize,
		"encoded_body_size": entry.EncodedBodySize,
		"timestamp":         entry.Timestamp,
		"page_url":          entry.PageURL,
	}
}

func waterfallSummaryEntry(entry capture.NetworkWaterfallEntry) map[string]any {
	url := entry.URL
	if len(url) > 80 {
		url = url[:80] + "..."
	}
	return map[string]any{"url": url, "ms": entry.Duration, "type": entry.InitiatorType}
}

func filterWaterfallSummaryEntries(allEntries []capture.NetworkWaterfallEntry, urlFilter string, limit int) []map[string]any {
	matched := buffers.ReverseFilterLimit(allEntries, func(entry capture.NetworkWaterfallEntry) bool {
		return urlFilter == "" || (entry.URL != "" && ContainsIgnoreCase(entry.URL, urlFilter))
	}, limit)

	entries := make([]map[string]any, len(matched))
	for i, entry := range matched {
		entries[i] = waterfallSummaryEntry(entry)
	}
	return entries
}

// GetWSStatus returns the current WebSocket connection status.
func GetWSStatus(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var arguments struct {
		URL          string `json:"url"`
		ConnectionID string `json:"connection_id"`
		Summary      bool   `json:"summary"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrInvalidJSON, "Invalid arguments JSON: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	filter := capture.WebSocketStatusFilter{
		URLFilter:    arguments.URL,
		ConnectionID: arguments.ConnectionID,
	}
	status := deps.GetCapture().GetWebSocketStatus(filter)
	metadata := BuildResponseMetadata(deps.GetCapture(), time.Now())

	if arguments.Summary {
		response := buildWSStatusSummary(status, metadata)
		if len(status.Connections) == 0 && len(status.Closed) == 0 {
			response["hint"] = hints.WSStatus()
		}
		return mcp.Succeed(req, "WebSocket status", response)
	}

	response := map[string]any{
		"connections":  status.Connections,
		"closed":       status.Closed,
		"active_count": len(status.Connections),
		"closed_count": len(status.Closed),
		"metadata":     metadata,
	}

	if len(status.Connections) == 0 && len(status.Closed) == 0 {
		response["hint"] = hints.WSStatus()
	}

	return mcp.Succeed(req, "WebSocket status", response)
}

func buildWSStatusSummary(status capture.WebSocketStatusResponse, metadata ResponseMetadata) map[string]any {
	activeURLs := make([]string, 0, wsStatusSummarySampleLimit)
	activeIDs := make([]string, 0, wsStatusSummarySampleLimit)
	closedURLs := make([]string, 0, wsStatusSummarySampleLimit)
	closedIDs := make([]string, 0, wsStatusSummarySampleLimit)

	activeURLSeen := map[string]struct{}{}
	closedURLSeen := map[string]struct{}{}

	for _, conn := range status.Connections {
		if len(activeIDs) < wsStatusSummarySampleLimit && conn.ID != "" {
			activeIDs = append(activeIDs, conn.ID)
		}
		if len(activeURLs) >= wsStatusSummarySampleLimit || conn.URL == "" {
			continue
		}
		if _, ok := activeURLSeen[conn.URL]; ok {
			continue
		}
		activeURLSeen[conn.URL] = struct{}{}
		activeURLs = append(activeURLs, conn.URL)
	}

	for _, conn := range status.Closed {
		if len(closedIDs) < wsStatusSummarySampleLimit && conn.ID != "" {
			closedIDs = append(closedIDs, conn.ID)
		}
		if len(closedURLs) >= wsStatusSummarySampleLimit || conn.URL == "" {
			continue
		}
		if _, ok := closedURLSeen[conn.URL]; ok {
			continue
		}
		closedURLSeen[conn.URL] = struct{}{}
		closedURLs = append(closedURLs, conn.URL)
	}

	return map[string]any{
		"active_count":          len(status.Connections),
		"closed_count":          len(status.Closed),
		"active_urls":           activeURLs,
		"closed_urls":           closedURLs,
		"active_connection_ids": activeIDs,
		"closed_connection_ids": closedIDs,
		"metadata":              metadata,
	}
}

// GetWebVitals returns Core Web Vitals metrics from performance snapshots.
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
