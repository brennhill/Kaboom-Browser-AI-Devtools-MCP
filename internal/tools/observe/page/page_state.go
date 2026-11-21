// Purpose: The live-page observe modes — page, storage, indexeddb — plus analyze's accessibility audit.
// Why: Unlike the buffer readers, each of these round-trips to the tracked tab to ask the page what it looks
// like right now, so they share the tracking gate and the "no tab is being tracked" error path.
// Docs: docs/features/feature/observe/index.md

package page

import (
	"context"
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/core"
	"sort"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/a11ysummary"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/idbquery"
)

// GetPageInfo adapts the pageinfo use case to observe(what="page").
func GetPageInfo(deps core.Deps, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	info := deps.PageInfo.Read()
	result := map[string]any{
		"url":                     info.URL,
		"title":                   info.Title,
		"tracked":                 info.Tracked,
		"csp_restricted":          info.CSPRestricted,
		"csp_level":               info.CSPLevel,
		"tab_status":              info.TabStatus,
		"page_ready_for_commands": info.ReadyForCommands,
		"metadata":                core.BuildResponseMetadata(deps.Capture, time.Now()),
	}
	if info.TabID > 0 {
		result["tab_id"] = info.TabID
	}
	if info.TabActiveKnown {
		result["is_active"] = info.TabActive
	}

	// Include blocked_actions/blocked_reason when CSP restricts — omit entirely
	// when CSP is clear to avoid wasting tokens on normal pages. (#262)
	if info.CSPRestricted {
		if actions, reason := syncruntime.CSPBlockedActions(info.CSPLevel); actions != nil {
			result["blocked_actions"] = actions
			result["blocked_reason"] = reason
		}
	}

	return mcp.Succeed(req, "Page info", result)
}

// summarizeStorageMap returns a summary of a key-value storage map.
func summarizeStorageMap(data map[string]any) map[string]any {
	keys := make([]string, 0, len(data))
	totalBytes := 0
	for k, v := range data {
		keys = append(keys, k)
		totalBytes += len(k)
		if s, ok := v.(string); ok {
			totalBytes += len(s)
		} else {
			b, _ := json.Marshal(v)
			totalBytes += len(b)
		}
	}
	sampleKeys := keys
	if len(sampleKeys) > 5 {
		sampleKeys = sampleKeys[:5]
	}
	return map[string]any{
		"key_count":   len(data),
		"total_bytes": totalBytes,
		"sample_keys": sampleKeys,
	}
}

// summarizeCookies returns a summary of a cookie array.
func summarizeCookies(cookies []any) map[string]any {
	names := make([]string, 0, len(cookies))
	totalBytes := 0
	for _, c := range cookies {
		if m, ok := c.(map[string]any); ok {
			if name, ok := m["name"].(string); ok {
				names = append(names, name)
			}
			b, _ := json.Marshal(m)
			totalBytes += len(b)
		}
	}
	sampleKeys := names
	if len(sampleKeys) > 5 {
		sampleKeys = sampleKeys[:5]
	}
	return map[string]any{
		"key_count":   len(cookies),
		"total_bytes": totalBytes,
		"sample_keys": sampleKeys,
	}
}

// storageParams holds parsed parameters for storage queries.
type storageParams struct {
	Summary     bool
	StorageType string // "local", "session", "cookies", or "" for all
	Key         string // specific key/cookie name filter
}

func parseStorageParams(args json.RawMessage) storageParams {
	p := storageParams{Summary: true}
	if len(args) == 0 {
		return p
	}
	var raw struct {
		Summary     *bool  `json:"summary"`
		StorageType string `json:"storage_type"`
		Key         string `json:"key"`
	}
	if json.Unmarshal(args, &raw) == nil {
		if raw.Summary != nil {
			p.Summary = *raw.Summary
		}
		p.StorageType = raw.StorageType
		p.Key = raw.Key
	}
	return p
}

// filterStorageMap filters a storage map by key. Returns nil if key not found.
func filterStorageMap(data map[string]any, key string) map[string]any {
	if key == "" {
		return data
	}
	if v, ok := data[key]; ok {
		return map[string]any{key: v}
	}
	return map[string]any{}
}

// filterCookies filters a cookie array by name.
func filterCookies(cookies []any, name string) []any {
	if name == "" {
		return cookies
	}
	var filtered []any
	for _, c := range cookies {
		if m, ok := c.(map[string]any); ok {
			if n, ok := m["name"].(string); ok && n == name {
				filtered = append(filtered, c)
			}
		}
	}
	return filtered
}

// GetStorage returns localStorage, sessionStorage, and cookies from the tracked tab.
func GetStorage(deps core.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	params := parseStorageParams(args)
	cap := deps.Capture
	enabled, _, _ := cap.Extension().GetTrackingStatus()
	if !enabled {
		return mcp.Fail(req,
			mcp.ErrNoData,
			"No tab is being tracked. Open the Kaboom extension popup and click 'Track This Tab'.",
			"Track a tab first, then call observe with what='storage'.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)
	}

	stateResult, errResp := fetchStorageState(deps, req)
	if errResp != nil {
		return *errResp
	}

	response := map[string]any{
		"url":      stateResult["url"],
		"metadata": core.BuildResponseMetadata(cap, time.Now()),
	}
	attachStorageSections(response, stateResult, params)

	// IndexedDB listing is best-effort (skip if storage_type filter excludes it)
	if params.StorageType == "" {
		attachIndexedDBListing(response, deps.Commands)
	}

	return mcp.Succeed(req, "Browser storage", response)
}

func (p storageParams) includesType(storageType string) bool {
	return p.StorageType == "" || p.StorageType == storageType
}

func fetchStorageState(deps core.Deps, req mcp.JSONRPCRequest) (map[string]any, *mcp.JSONRPCResponse) {
	queryID, qerr := deps.Commands.Enqueue(
		queries.PendingQuery{
			Type:   "state_capture",
			Params: json.RawMessage(`{"action":"capture"}`),
		},
		10*time.Second,
		"",
	)
	if qerr != nil {
		resp := queueFullResponse(req, qerr)
		return nil, &resp
	}

	result, err := deps.Commands.WaitQueryResult(context.Background(), queryID, 10*time.Second)
	if err != nil {
		resp := mcp.Fail(req,
			mcp.ErrExtTimeout,
			"Storage capture timeout: "+err.Error(),
			"Ensure the extension is connected and the page has loaded.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)
		return nil, &resp
	}

	var stateResult map[string]any
	if err := json.Unmarshal(result, &stateResult); err != nil {
		resp := mcp.Fail(req,
			mcp.ErrInvalidJSON,
			"Failed to parse storage result: "+err.Error(),
			"Check extension logs for errors",
		)
		return nil, &resp
	}

	if errMsg, ok := stateResult["error"].(string); ok {
		resp := mcp.Fail(req,
			mcp.ErrExtError,
			"Storage capture failed: "+errMsg,
			"Check that the tab is accessible.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)
		return nil, &resp
	}

	return stateResult, nil
}

func attachStorageSections(response map[string]any, stateResult map[string]any, params storageParams) {
	if params.includesType("local") {
		if v, ok := stateResult["localStorage"].(map[string]any); ok {
			response["local_storage"] = storageMapValue(v, params.Key, params.Summary)
		}
	}
	if params.includesType("session") {
		if v, ok := stateResult["sessionStorage"].(map[string]any); ok {
			response["session_storage"] = storageMapValue(v, params.Key, params.Summary)
		}
	}
	if params.includesType("cookies") {
		if v, ok := stateResult["cookies"].([]any); ok {
			response["cookies"] = cookiesValue(v, params.Key, params.Summary)
		}
	}
}

func storageMapValue(v map[string]any, key string, summary bool) map[string]any {
	filtered := filterStorageMap(v, key)
	if summary {
		return summarizeStorageMap(filtered)
	}
	return filtered
}

func cookiesValue(v []any, name string, summary bool) any {
	filtered := filterCookies(v, name)
	if summary {
		return summarizeCookies(filtered)
	}
	return filtered
}

func attachIndexedDBListing(response map[string]any, commands queries.QueryRunner) {
	indexeddb, err := idbquery.Listing(commands)
	if err != nil {
		response["indexeddb"] = map[string]any{
			"supported": false,
			"databases": []any{},
		}
		response["indexeddb_error"] = err.Error()
		return
	}
	response["indexeddb"] = indexeddb
}

// GetIndexedDB returns rows from one IndexedDB object store.
func GetIndexedDB(deps core.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Database string `json:"database"`
		Store    string `json:"store"`
		Limit    int    `json:"limit"`
	}
	mcp.LenientUnmarshal(args, &params)

	if params.Database == "" {
		return mcp.Fail(req,
			mcp.ErrMissingParam,
			"Required parameter 'database' is missing for observe(what='indexeddb')",
			"Add the 'database' parameter and call again.",
			mcp.WithParam("database"),
		)
	}
	if params.Store == "" {
		return mcp.Fail(req,
			mcp.ErrMissingParam,
			"Required parameter 'store' is missing for observe(what='indexeddb')",
			"Add the 'store' parameter and call again.",
			mcp.WithParam("store"),
		)
	}
	params.Limit = core.ClampLimit(params.Limit, 100)

	cap := deps.Capture
	enabled, _, _ := cap.Extension().GetTrackingStatus()
	if !enabled {
		return mcp.Fail(req,
			mcp.ErrNoData,
			"No tab is being tracked. Open the Kaboom extension popup and click 'Track This Tab'.",
			"Track a tab first, then call observe with what='indexeddb'.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)
	}

	storeData, err := idbquery.Entries(deps.Commands, params.Database, params.Store, params.Limit)
	if err != nil {
		return mcp.Fail(req,
			mcp.ErrExtError,
			"IndexedDB inspection failed: "+err.Error(),
			"Ensure the tab is accessible and the database/store names are correct.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)
	}

	entries, _ := storeData["entries"].([]any)
	count := len(entries)
	if c, ok := toInt(storeData["count"]); ok {
		count = c
	}

	response := map[string]any{
		"database": params.Database,
		"store":    params.Store,
		"entries":  entries,
		"count":    count,
		"limit":    params.Limit,
		"metadata": core.BuildResponseMetadata(cap, time.Now()),
	}
	if v, ok := storeData["object_stores"]; ok {
		response["object_stores"] = v
	}

	return mcp.Succeed(req, "IndexedDB entries", response)
}

// toInt reads a count out of the page's reply, which arrives as float64 through
// encoding/json but may be any numeric kind when the value is synthesized server-side.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

// RunA11yAudit executes an accessibility audit via the extension.
func RunA11yAudit(deps core.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Selector     string   `json:"selector"`
		Scope        string   `json:"scope"`
		Tags         []string `json:"tags"`
		ForceRefresh bool     `json:"force_refresh"`
		Frame        any      `json:"frame"`
		Summary      bool     `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)
	if params.Scope == "" && params.Selector != "" {
		params.Scope = params.Selector
	}

	enabled, _, _ := deps.Capture.Extension().GetTrackingStatus()
	if !enabled {
		return mcp.Fail(req, mcp.ErrNoData, "No tab is being tracked. Open the Kaboom extension popup and click 'Track This Tab' on the page you want to monitor. Check observe with what='pilot' for extension status.", "", mcp.WithHint(deps.DiagnosticHintString()))
	}

	result, err := deps.ExecuteA11yQuery(params.Scope, params.Tags, params.Frame, params.ForceRefresh)
	if err != nil {
		// Issue #276: return partial results with error field instead of hard failure.
		// This lets the caller know what happened while providing a usable response shape.
		partialResult := map[string]any{
			"violations":   []any{},
			"passes":       []any{},
			"incomplete":   []any{},
			"inapplicable": []any{},
			"error":        err.Error(),
			"partial":      true,
			"summary":      a11ysummary.BuildSummary(a11ysummary.Counts{}),
		}
		return mcp.Succeed(req, "A11y audit (partial — "+err.Error()+")", partialResult)
	}

	var auditResult map[string]any
	if err := json.Unmarshal(result, &auditResult); err != nil {
		return mcp.Fail(req, mcp.ErrInvalidJSON, "Failed to parse a11y result: "+err.Error(), "Check extension logs for errors")
	}

	if params.Summary {
		return mcp.Succeed(req, "A11y audit", buildA11ySummary(auditResult))
	}

	ensureA11ySummary(auditResult)
	return mcp.Succeed(req, "A11y audit", auditResult)
}

// ensureA11ySummary adds or normalizes the summary map on a11y results.
// It keeps canonical keys and legacy aliases synchronized.
func ensureA11ySummary(auditResult map[string]any) {
	a11ysummary.EnsureAuditSummary(auditResult)
}

// buildA11ySummary creates a compact representation of an a11y audit result.
func buildA11ySummary(auditResult map[string]any) map[string]any {
	passes, _ := auditResult["passes"].([]any)
	violations, _ := auditResult["violations"].([]any)
	incomplete, _ := auditResult["incomplete"].([]any)

	type issueInfo struct {
		rule     string
		severity string
		count    int
	}
	issues := make([]issueInfo, 0, len(violations))
	for _, v := range violations {
		vMap, ok := v.(map[string]any)
		if !ok {
			continue
		}
		rule, _ := vMap["id"].(string)
		impact, _ := vMap["impact"].(string)
		nodes, _ := vMap["nodes"].([]any)
		issues = append(issues, issueInfo{rule: rule, severity: impact, count: len(nodes)})
	}
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].count > issues[j].count
	})
	topN := 5
	if len(issues) < topN {
		topN = len(issues)
	}
	topIssues := make([]map[string]any, topN)
	for i := 0; i < topN; i++ {
		topIssues[i] = map[string]any{
			"rule":     issues[i].rule,
			"count":    issues[i].count,
			"severity": issues[i].severity,
		}
	}

	return map[string]any{
		"pass":       len(passes),
		"violations": len(violations),
		"incomplete": len(incomplete),
		"top_issues": topIssues,
	}
}

func queueFullResponse(req mcp.JSONRPCRequest, err error) mcp.JSONRPCResponse {
	return mcp.Fail(req,
		mcp.ErrQueueFull,
		"Command queue full: "+err.Error(),
		"Wait for in-flight commands to complete, then retry.",
		mcp.WithRecoveryToolCall(map[string]any{"tool": "observe", "arguments": map[string]any{"what": "pending_commands"}}),
	)
}
