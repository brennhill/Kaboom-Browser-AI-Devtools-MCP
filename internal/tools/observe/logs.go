// Purpose: The console-stream observe modes — errors, logs, extension_logs — plus analyze's error_clusters.
// Why: These read the same log buffer and share its normalization, level filtering and summary shape. The
// summary builders live here rather than in a summary_builders_* file because each had exactly one caller,
// and that caller is in this file.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/buffers"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pagination"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/errorcluster"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/hints"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// GetBrowserErrors returns error-level log entries from the capture buffer.
func GetBrowserErrors(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit   int    `json:"limit"`
		URL     string `json:"url"`
		Scope   string `json:"scope"`
		Summary bool   `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)
	params.Limit = clampLimit(params.Limit, 100)
	if params.Scope == "" {
		params.Scope = "current_page"
	}
	var paramHint string
	if params.Scope != "current_page" && params.Scope != "all" {
		paramHint = "Unknown scope " + params.Scope + " ignored (using default=current_page). Valid values: current_page, all."
		params.Scope = "current_page"
	}

	_, trackedTabID, trackedTabURL := deps.GetCapture().GetTrackingStatus()
	if params.URL == "" && params.Scope == "current_page" && trackedTabURL != "" {
		params.URL = trackedTabURL
	}
	entries, _ := deps.GetLogEntries()

	noiseSuppressed := 0
	matched := buffers.ReverseFilterLimit(entries, func(entry map[string]any) bool {
		level, _ := entry["level"].(string)
		if level != "error" {
			return false
		}
		if deps.IsConsoleNoise(entry) {
			noiseSuppressed++
			return false
		}
		if params.Scope == "current_page" && trackedTabID != 0 {
			entryTabID, _ := entry["tabId"].(float64)
			if int(entryTabID) != trackedTabID {
				return false
			}
		}
		if params.URL != "" {
			entryURL, _ := entry["url"].(string)
			if !ContainsIgnoreCase(entryURL, params.URL) {
				return false
			}
		}
		return true
	}, params.Limit)

	errors := make([]map[string]any, len(matched))
	for i, entry := range matched {
		errors[i] = map[string]any{
			"message":   entry["message"],
			"source":    entry["source"],
			"url":       entry["url"],
			"line":      entry["line"],
			"column":    entry["column"],
			"stack":     entry["stack"],
			"timestamp": entry["ts"],
			"tab_id":    entry["tabId"],
		}
	}

	var newestTS time.Time
	if len(errors) > 0 {
		if ts, ok := errors[0]["timestamp"].(string); ok {
			newestTS, _ = time.Parse(time.RFC3339, ts)
		}
	}

	responseMeta := BuildResponseMetadata(deps.GetCapture(), newestTS)
	responseMeta.NoiseSuppressed = noiseSuppressed

	if params.Summary {
		summaryResp := buildErrorsSummary(errors, noiseSuppressed, responseMeta)
		if paramHint != "" {
			summaryResp["param_hint"] = paramHint
		}
		return mcp.Succeed(req, "Browser errors", summaryResp)
	}

	response := map[string]any{
		"errors":   errors,
		"count":    len(errors),
		"metadata": responseMeta,
		"scope":    params.Scope,
	}
	if paramHint != "" {
		response["param_hint"] = paramHint
	}
	if len(errors) == 0 {
		response["hint"] = hints.Errors(params.Scope)
	}
	return mcp.Succeed(req, "Browser errors", response)
}

// GetBrowserLogs returns console log entries with cursor-based pagination.
// #lizard forgives
func GetBrowserLogs(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit             int    `json:"limit"`
		Level             string `json:"level"`
		MinLevel          string `json:"min_level"`
		Source            string `json:"source"`
		URL               string `json:"url"`
		Scope             string `json:"scope"`
		AfterCursor       string `json:"after_cursor"`
		BeforeCursor      string `json:"before_cursor"`
		SinceCursor       string `json:"since_cursor"`
		RestartOnEviction bool   `json:"restart_on_eviction"`
		IncludeInternal   bool   `json:"include_internal"`
		IncludeExtension  bool   `json:"include_extension_logs"`
		ExtensionLimit    int    `json:"extension_limit"`
		Summary           bool   `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)

	// Quiet alias: level → min_level (threshold, not exact match).
	if params.Level != "" && params.MinLevel == "" {
		params.MinLevel = params.Level
		params.Level = ""
	}

	var paramHint string
	if params.MinLevel != "" && LogLevelRank(params.MinLevel) < 0 {
		paramHint = "Unknown min_level " + params.MinLevel + " ignored (using default=all). Valid values: debug, log, info, warn, error."
		params.MinLevel = ""
	}

	if params.Scope == "" {
		params.Scope = "current_page"
	}
	if params.Scope != "current_page" && params.Scope != "all" {
		hint := "Unknown scope " + params.Scope + " ignored (using default=current_page). Valid values: current_page, all."
		if paramHint != "" {
			paramHint += " " + hint
		} else {
			paramHint = hint
		}
		params.Scope = "current_page"
	}

	_, trackedTabID, trackedTabURL := deps.GetCapture().GetTrackingStatus()
	params.Limit = clampLimit(params.Limit, 100)

	// Default URL filter to the tracked page URL so logs are scoped to
	// the current page, not stale entries from previous navigations.
	if params.URL == "" && params.Scope == "current_page" && trackedTabURL != "" {
		params.URL = trackedTabURL
	}

	rawEntries, _ := deps.GetLogEntries()
	totalAdded := deps.GetLogTotalAdded()

	// Convert to []map[string]any for pagination package.
	allEntries := make([]map[string]any, len(rawEntries))
	for i, e := range rawEntries {
		allEntries[i] = e
	}

	enriched := pagination.EnrichLogEntries(allEntries, totalAdded)

	filtered := make([]pagination.LogEntryWithSequence, 0, len(enriched))
	noiseSuppressed := 0
	for _, e := range enriched {
		entryType, _ := e.Entry["type"].(string)
		if !params.IncludeInternal && isInternalLogType(entryType) {
			continue
		}

		if deps.IsConsoleNoise(e.Entry) {
			noiseSuppressed++
			continue
		}

		if params.Scope == "current_page" && trackedTabID != 0 {
			if !(params.IncludeInternal && isInternalLogType(entryType)) {
				entryTabID, _ := e.Entry["tabId"].(float64)
				if int(entryTabID) != trackedTabID {
					continue
				}
			}
		}

		level, _ := e.Entry["level"].(string)
		if level == "" && isInternalLogType(entryType) {
			level = "info"
		}
		if params.Level != "" && level != params.Level {
			continue
		}

		if params.MinLevel != "" && LogLevelRank(level) < LogLevelRank(params.MinLevel) {
			continue
		}

		if params.Source != "" {
			source, _ := e.Entry["source"].(string)
			if source != params.Source {
				continue
			}
		}

		if params.URL != "" {
			entryURL, _ := e.Entry["url"].(string)
			if !ContainsIgnoreCase(entryURL, params.URL) {
				continue
			}
		}

		filtered = append(filtered, e)
	}

	paginated, pMeta, err := pagination.ApplyLogCursorPagination(
		filtered,
		params.AfterCursor, params.BeforeCursor, params.SinceCursor,
		params.Limit,
		params.RestartOnEviction,
	)
	if err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrInvalidParam, err.Error(), "Check cursor format or use restart_on_eviction:true")}
	}

	logs := make([]map[string]any, len(paginated))
	for i, e := range paginated {
		logs[i] = normalizeBrowserLogEntry(e.Entry)
	}

	var newestTS time.Time
	if len(paginated) > 0 {
		last := paginated[len(paginated)-1]
		if ts := logEntryTimestamp(last.Entry); ts != "" {
			newestTS = util.ParseTimestamp(ts)
		}
	}

	isFirstPage := params.AfterCursor == "" && params.BeforeCursor == "" && params.SinceCursor == ""
	meta := BuildPaginatedMetadataWithSummary(deps.GetCapture(), newestTS, pMeta, isFirstPage, func() map[string]any {
		return quickLogsSummary(logs)
	})
	meta["scope"] = params.Scope
	if noiseSuppressed > 0 {
		meta["noise_suppressed"] = noiseSuppressed
	}

	if params.Summary {
		summaryResp := buildLogsSummary(logs, meta)
		if paramHint != "" {
			summaryResp["param_hint"] = paramHint
		}
		return mcp.Succeed(req, "Browser logs", summaryResp)
	}

	response := map[string]any{
		"logs":     logs,
		"count":    len(logs),
		"metadata": meta,
	}
	if paramHint != "" {
		response["param_hint"] = paramHint
	}
	if len(logs) == 0 {
		response["hint"] = hints.Logs(params.Scope, params.MinLevel)
	}

	if params.IncludeExtension {
		limit := params.ExtensionLimit
		if limit <= 0 {
			limit = params.Limit
		}
		limit = clampLimit(limit, 100)
		extLogs := buildExtensionLogEntries(deps.GetCapture().GetExtensionLogs(), limit, params.Level, params.MinLevel)
		response["extension_logs"] = extLogs
		response["extension_logs_count"] = len(extLogs)
	}

	return mcp.Succeed(req, "Browser logs", response)
}

func isInternalLogType(entryType string) bool {
	return entryType == "lifecycle" || entryType == "tracking" || entryType == "extension"
}

func logEntryTimestamp(entry map[string]any) string {
	if ts, ok := entry["ts"].(string); ok && ts != "" {
		return ts
	}
	if ts, ok := entry["timestamp"].(string); ok && ts != "" {
		return ts
	}
	return ""
}

func normalizeBrowserLogEntry(entry map[string]any) map[string]any {
	entryType, _ := entry["type"].(string)
	level, _ := entry["level"].(string)
	if level == "" && isInternalLogType(entryType) {
		level = "info"
	}

	message, _ := entry["message"].(string)
	if message == "" {
		if event, ok := entry["event"].(string); ok {
			message = event
		}
	}

	source, _ := entry["source"].(string)
	if source == "" && isInternalLogType(entryType) {
		source = "daemon"
	}

	normalized := map[string]any{
		"level":     level,
		"message":   message,
		"source":    source,
		"url":       entry["url"],
		"line":      entry["line"],
		"column":    entry["column"],
		"timestamp": logEntryTimestamp(entry),
		"tab_id":    entry["tabId"],
	}

	if entryType != "" {
		normalized["type"] = entryType
	}
	if event, ok := entry["event"]; ok {
		normalized["event"] = event
	}
	if pid, ok := entry["pid"]; ok {
		normalized["pid"] = pid
	}
	if port, ok := entry["port"]; ok {
		normalized["port"] = port
	}

	extras := make(map[string]any)
	for k, v := range entry {
		switch k {
		case "type", "level", "message", "source", "url", "line", "column", "ts", "timestamp", "tabId", "event", "pid", "port":
			// handled above
		default:
			extras[k] = v
		}
	}
	if len(extras) > 0 {
		normalized["data"] = extras
	}

	return normalized
}

func buildExtensionLogEntries(allLogs []capture.ExtensionLog, limit int, level string, minLevel string) []map[string]any {
	matched := buffers.ReverseFilterLimit(allLogs, func(entry capture.ExtensionLog) bool {
		if level != "" && entry.Level != level {
			return false
		}
		if minLevel != "" && LogLevelRank(entry.Level) < LogLevelRank(minLevel) {
			return false
		}
		return true
	}, limit)

	logs := make([]map[string]any, len(matched))
	for i, entry := range matched {
		logs[i] = map[string]any{
			"level":     entry.Level,
			"message":   entry.Message,
			"source":    entry.Source,
			"category":  entry.Category,
			"data":      entry.Data,
			"timestamp": entry.Timestamp,
		}
	}
	return logs
}

// GetExtensionLogs returns internal extension debug logs.
func GetExtensionLogs(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit int    `json:"limit"`
		Level string `json:"level"`
	}
	mcp.LenientUnmarshal(args, &params)
	params.Limit = clampLimit(params.Limit, 100)

	allLogs := deps.GetCapture().GetExtensionLogs()
	logs := buildExtensionLogEntries(allLogs, params.Limit, params.Level, "")

	var newestTS time.Time
	if len(allLogs) > 0 {
		newestTS = allLogs[len(allLogs)-1].Timestamp
	}

	return mcp.Succeed(req, "Extension logs", map[string]any{
		"logs":     logs,
		"count":    len(logs),
		"metadata": BuildResponseMetadata(deps.GetCapture(), newestTS),
	})
}

// AnalyzeErrors clusters error entries by normalized message for pattern detection.
func AnalyzeErrors(deps Deps, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	entries, _ := deps.GetLogEntries()
	result := errorcluster.Analyze(entries)

	return mcp.Succeed(req, "Error clusters", map[string]any{
		"clusters":    result,
		"total_count": len(result),
		"metadata":    BuildResponseMetadata(deps.GetCapture(), time.Now()),
	})
}

// buildErrorsSummary returns {total, by_source, top_messages, metadata}.
func buildErrorsSummary(errors []map[string]any, noiseSuppressed int, meta ResponseMetadata) map[string]any {
	bySource := make(map[string]int)
	msgCounts := make(map[string]int)

	for _, e := range errors {
		src, _ := e["source"].(string)
		if src == "" {
			src = "unknown"
		}
		bySource[src]++

		msg, _ := e["message"].(string)
		if msg != "" {
			msg = truncateRunes(msg, 100)
			msgCounts[msg]++
		}
	}

	// Build top messages sorted by frequency
	type msgCount struct {
		msg   string
		count int
	}
	ranked := make([]msgCount, 0, len(msgCounts))
	for msg, count := range msgCounts {
		ranked = append(ranked, msgCount{msg, count})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].count > ranked[j].count
	})
	topN := 5
	if len(ranked) < topN {
		topN = len(ranked)
	}
	topMessages := make([]map[string]any, topN)
	for i := 0; i < topN; i++ {
		topMessages[i] = map[string]any{"message": ranked[i].msg, "count": ranked[i].count}
	}

	result := map[string]any{
		"total":        len(errors),
		"by_source":    bySource,
		"top_messages": topMessages,
		"metadata":     meta,
	}
	if noiseSuppressed > 0 {
		result["noise_suppressed"] = noiseSuppressed
	}
	return result
}

// buildLogsSummary returns {total, by_level, by_source, metadata}.
func buildLogsSummary(logs []map[string]any, meta map[string]any) map[string]any {
	byLevel := make(map[string]int)
	bySource := make(map[string]int)

	for _, l := range logs {
		level, _ := l["level"].(string)
		if level == "" {
			level = "unknown"
		}
		byLevel[level]++

		src, _ := l["source"].(string)
		if src == "" {
			src = "unknown"
		}
		bySource[src]++
	}

	return map[string]any{
		"total":     len(logs),
		"by_level":  byLevel,
		"by_source": bySource,
		"metadata":  meta,
	}
}

// quickLogsSummary is a lightweight version for pagination header (just by_level + total).
func quickLogsSummary(logs []map[string]any) map[string]any {
	byLevel := make(map[string]int)
	for _, l := range logs {
		level, _ := l["level"].(string)
		if level == "" {
			level = "unknown"
		}
		byLevel[level]++
	}
	return map[string]any{
		"total":    len(logs),
		"by_level": byLevel,
	}
}

// truncateRunes truncates a string to maxRunes runes, avoiding mid-character splits.
func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes])
}
