// Purpose: The network-stream observe modes — network_waterfall, network_bodies, websocket_events, websocket_status.
// Why: All four read request and socket traffic out of the capture store and share the URL-filter and
// summary-mode shape; their summary builders sit with the handlers that call them.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/buffers"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/hints"
)

// GetNetworkBodies returns captured HTTP response bodies with optional filtering.
// #lizard forgives
func GetNetworkBodies(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit     int    `json:"limit"`
		URL       string `json:"url"`
		Method    string `json:"method"`
		StatusMin int    `json:"status_min"`
		StatusMax int    `json:"status_max"`
		BodyPath  string `json:"body_path"`
		Summary   bool   `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)
	params.Limit = clampLimit(params.Limit, 100)

	allBodies := deps.GetCapture().GetNetworkBodies()
	var bodyFilterErr error
	filtered := buffers.ReverseFilterLimit(allBodies, func(b types.NetworkBody) bool {
		if bodyFilterErr != nil {
			return false
		}
		if params.URL != "" && !ContainsIgnoreCase(b.URL, params.URL) {
			return false
		}
		if params.Method != "" && !ContainsIgnoreCase(b.Method, params.Method) {
			return false
		}
		if params.StatusMin > 0 && b.Status < params.StatusMin {
			return false
		}
		if params.StatusMax > 0 && b.Status > params.StatusMax {
			return false
		}
		_, include, err := ApplyNetworkBodyFilter(b, params.BodyPath)
		if err != nil {
			bodyFilterErr = err
			return false
		}
		return include
	}, params.Limit)

	if bodyFilterErr != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrInvalidParam,
			"Invalid network body filter: "+bodyFilterErr.Error(),
			"Use a valid body_path syntax like data.items[0].id",
			mcp.WithParam("body_path"),
		)}
	}

	// Re-apply body filter to transform matched entries (extract body_path).
	if params.BodyPath != "" {
		for i, b := range filtered {
			filteredBody, _, _ := ApplyNetworkBodyFilter(b, params.BodyPath)
			filtered[i] = filteredBody
		}
	}
	var newestTS time.Time
	if len(allBodies) > 0 {
		newestTS, _ = time.Parse(time.RFC3339, allBodies[len(allBodies)-1].Timestamp)
	}

	waterfallCount := len(deps.GetCapture().NetworkWaterfall().Entries())
	responseMeta := BuildResponseMetadata(deps.GetCapture(), newestTS)
	hintFilters := hints.NetworkBodiesFilters{
		URL:       params.URL,
		Method:    params.Method,
		StatusMin: params.StatusMin,
		StatusMax: params.StatusMax,
		BodyPath:  params.BodyPath,
	}
	if params.Summary {
		summary := buildNetworkBodiesSummary(filtered, responseMeta)
		if len(filtered) == 0 {
			summary["hint"] = hints.NetworkBodies(waterfallCount, len(allBodies), hintFilters)
		}
		return mcp.Succeed(req, "Network bodies", summary)
	}

	response := map[string]any{
		"entries":  filtered,
		"count":    len(filtered),
		"metadata": responseMeta,
	}

	if len(filtered) == 0 {
		response["hint"] = hints.NetworkBodies(waterfallCount, len(allBodies), hintFilters)
	}

	return mcp.Succeed(req, "Network bodies", response)
}

// GetWSEvents returns captured WebSocket events with optional filtering.
func GetWSEvents(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit        int    `json:"limit"`
		URL          string `json:"url"`
		ConnectionID string `json:"connection_id"`
		Direction    string `json:"direction"`
		Summary      bool   `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)

	var paramHint string
	if params.Direction != "" && params.Direction != "incoming" && params.Direction != "outgoing" {
		paramHint = "Unknown direction " + params.Direction + " ignored (using default=all). Valid values: incoming, outgoing."
		params.Direction = ""
	}

	params.Limit = clampLimit(params.Limit, 100)

	allEvents := deps.GetCapture().GetAllWebSocketEvents()
	filtered := buffers.ReverseFilterLimit(allEvents, func(evt types.WebSocketEvent) bool {
		if params.URL != "" && !ContainsIgnoreCase(evt.URL, params.URL) {
			return false
		}
		if params.ConnectionID != "" && evt.ID != params.ConnectionID {
			return false
		}
		if params.Direction != "" && evt.Direction != params.Direction {
			return false
		}
		return true
	}, params.Limit)
	var newestTS time.Time
	if len(allEvents) > 0 {
		newestTS, _ = time.Parse(time.RFC3339, allEvents[len(allEvents)-1].Timestamp)
	}

	responseMeta := BuildResponseMetadata(deps.GetCapture(), newestTS)
	if params.Summary {
		summary := buildWSEventsSummary(filtered, responseMeta)
		if paramHint != "" {
			summary["param_hint"] = paramHint
		}
		if len(filtered) == 0 {
			summary["hint"] = hints.WSEvents(len(allEvents), params.URL)
		}
		return mcp.Succeed(req, "WebSocket events", summary)
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
		response["hint"] = hints.WSEvents(len(allEvents), params.URL)
	}

	return mcp.Succeed(req, "WebSocket events", response)
}

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

func refreshWaterfallIfStale(deps Deps) []types.NetworkWaterfallEntry {
	cap := deps.GetCapture()
	allEntries := cap.NetworkWaterfall().Entries()
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
		Entries []types.NetworkWaterfallEntry `json:"entries"`
		PageURL string                        `json:"page_url"`
	}
	if err := json.Unmarshal(result, &waterfallResult); err == nil && len(waterfallResult.Entries) > 0 {
		cap.NetworkWaterfall().Add(waterfallResult.Entries, waterfallResult.PageURL)
		return cap.NetworkWaterfall().Entries()
	}
	return allEntries
}

func filterWaterfallEntries(allEntries []types.NetworkWaterfallEntry, urlFilter string, limit int) []map[string]any {
	matched := buffers.ReverseFilterLimit(allEntries, func(entry types.NetworkWaterfallEntry) bool {
		return urlFilter == "" || (entry.URL != "" && ContainsIgnoreCase(entry.URL, urlFilter))
	}, limit)

	entries := make([]map[string]any, len(matched))
	for i, entry := range matched {
		entries[i] = waterfallEntryToMap(entry)
	}
	return entries
}

func waterfallEntryToMap(entry types.NetworkWaterfallEntry) map[string]any {
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

func waterfallSummaryEntry(entry types.NetworkWaterfallEntry) map[string]any {
	url := entry.URL
	if len(url) > 80 {
		url = url[:80] + "..."
	}
	return map[string]any{"url": url, "ms": entry.Duration, "type": entry.InitiatorType}
}

func filterWaterfallSummaryEntries(allEntries []types.NetworkWaterfallEntry, urlFilter string, limit int) []map[string]any {
	matched := buffers.ReverseFilterLimit(allEntries, func(entry types.NetworkWaterfallEntry) bool {
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

	filter := types.WebSocketStatusFilter{
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

func buildWSStatusSummary(status types.WebSocketStatusResponse, metadata ResponseMetadata) map[string]any {
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

// buildNetworkBodiesSummary returns {total, by_status_group, by_method, top_urls, metadata}.
func buildNetworkBodiesSummary(bodies []types.NetworkBody, meta ResponseMetadata) map[string]any {
	byStatus := make(map[string]int)
	byMethod := make(map[string]int)
	seenURLs := make(map[string]bool)
	urls := make([]string, 0)

	for _, b := range bodies {
		// Status grouping: 2xx, 3xx, 4xx, 5xx
		group := statusGroup(b.Status)
		byStatus[group]++

		method := b.Method
		if method == "" {
			method = "GET"
		}
		byMethod[method]++

		url := b.URL
		if len([]rune(url)) > 80 {
			url = string([]rune(url)[:80]) + "..."
		}
		if !seenURLs[url] {
			seenURLs[url] = true
			urls = append(urls, url)
		}
	}

	topN := 5
	if len(urls) < topN {
		topN = len(urls)
	}

	return map[string]any{
		"total":           len(bodies),
		"by_status_group": byStatus,
		"by_method":       byMethod,
		"recent_urls":     urls[:topN],
		"metadata":        meta,
	}
}

// statusGroup converts an HTTP status code to a group string (2xx, 3xx, etc.).

func statusGroup(status int) string {
	switch {
	case status >= 200 && status < 300:
		return "2xx"
	case status >= 300 && status < 400:
		return "3xx"
	case status >= 400 && status < 500:
		return "4xx"
	case status >= 500 && status < 600:
		return "5xx"
	default:
		return "other"
	}
}

// buildWSEventsSummary returns {total, by_direction, by_event_type, connection_count, metadata}.

func buildWSEventsSummary(events []types.WebSocketEvent, meta ResponseMetadata) map[string]any {
	byDirection := make(map[string]int)
	byEvent := make(map[string]int)
	connIDs := make(map[string]bool)

	for _, e := range events {
		if e.Direction != "" {
			byDirection[e.Direction]++
		}
		if e.Event != "" {
			byEvent[e.Event]++
		}
		if e.ID != "" {
			connIDs[e.ID] = true
		}
	}

	return map[string]any{
		"total":            len(events),
		"by_direction":     byDirection,
		"by_event_type":    byEvent,
		"connection_count": len(connIDs),
		"metadata":         meta,
	}
}

// buildActionsSummary returns {total, by_type, time_range, metadata}.
