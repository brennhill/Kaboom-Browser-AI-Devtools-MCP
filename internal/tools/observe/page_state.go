// Purpose: The live-page observe modes — page, storage, indexeddb, screenshot — plus analyze's accessibility audit.
// Why: Unlike the buffer readers, each of these round-trips to the tracked tab to ask the page what it looks
// like right now, so they share the tracking gate and the "no tab is being tracked" error path.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/a11ysummary"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/idbquery"
)

// GetPageInfo returns information about the currently tracked page.
func GetPageInfo(deps Deps, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	cap := deps.GetCapture()
	enabled, tabID, trackedURL := cap.GetTrackingStatus()
	trackedTitle := cap.GetTrackedTabTitle()

	pageURL := resolvePageURL(cap, trackedURL)
	pageTitle := resolvePageTitle(deps, trackedTitle)

	cspRestricted, cspLevel := cap.GetCSPStatus()
	tabStatus := cap.GetTabStatus()

	// Each state getter acquires c.mu.RLock independently. Between calls, state
	// could change (e.g., extension disconnects between GetTabStatus and
	// IsExtensionConnected). This is acceptable for an advisory readiness signal
	// — the next observe(what:"page") call will correct any inconsistency.
	extensionConnected := cap.IsExtensionConnected()

	// Use IsPilotEnabled (defaults false) instead of IsPilotActionAllowed (defaults
	// true during startup). This avoids briefly reporting page_ready_for_commands=true
	// before the first extension sync confirms pilot status.
	pilotEnabled := cap.IsPilotEnabled()

	// page_ready_for_commands is true when all four conditions hold:
	//   1. extensionConnected — WebSocket link to extension is live
	//   2. pilotEnabled       — AI Web Pilot is enabled in extension settings
	//   3. enabled            — a tab is actively being tracked
	//   4. tabStatus=="complete" — the tracked tab has finished loading
	pageReady := extensionConnected && pilotEnabled && enabled && tabStatus == "complete"

	// Tab focus state: is the tracked tab the active (foreground) tab?
	tabActive, tabActiveKnown := cap.IsTrackedTabActive()

	result := map[string]any{
		"url":                     pageURL,
		"title":                   pageTitle,
		"tracked":                 enabled,
		"csp_restricted":          cspRestricted,
		"csp_level":               cspLevel,
		"tab_status":              tabStatus,
		"page_ready_for_commands": pageReady,
		"metadata":                BuildResponseMetadata(cap, time.Now()),
	}
	if tabID > 0 {
		result["tab_id"] = tabID
	}
	if tabActiveKnown {
		result["is_active"] = tabActive
	}

	// Include blocked_actions/blocked_reason when CSP restricts — omit entirely
	// when CSP is clear to avoid wasting tokens on normal pages. (#262)
	if cspRestricted {
		if actions, reason := capture.CSPBlockedActions(cspLevel); actions != nil {
			result["blocked_actions"] = actions
			result["blocked_reason"] = reason
		}
	}

	return mcp.Succeed(req, "Page info", result)
}

func resolvePageURL(cap *capture.Capture, trackedURL string) string {
	if trackedURL != "" {
		return trackedURL
	}
	waterfallEntries := cap.NetworkWaterfall().Entries()
	if len(waterfallEntries) > 0 {
		return waterfallEntries[len(waterfallEntries)-1].PageURL
	}
	return ""
}

func resolvePageTitle(deps Deps, trackedTitle string) string {
	if trackedTitle != "" {
		return trackedTitle
	}
	entries, _ := deps.GetLogEntries()
	for i := len(entries) - 1; i >= 0; i-- {
		if title, ok := entries[i]["title"].(string); ok && title != "" {
			return title
		}
	}
	return ""
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
func GetStorage(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	params := parseStorageParams(args)
	cap := deps.GetCapture()
	enabled, _, _ := cap.GetTrackingStatus()
	if !enabled {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrNoData,
			"No tab is being tracked. Open the Kaboom extension popup and click 'Track This Tab'.",
			"Track a tab first, then call observe with what='storage'.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)}
	}

	queryID, qerr := cap.Queries().CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "state_capture",
			Params: json.RawMessage(`{"action":"capture"}`),
		},
		10*time.Second,
		"",
	)
	if qerr != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrQueueFull,
			"Command queue full: "+qerr.Error(),
			"Wait for in-flight commands to complete, then retry.",
			mcp.WithRecoveryToolCall(map[string]any{"tool": "observe", "arguments": map[string]any{"what": "pending_commands"}}),
		)}
	}

	result, err := cap.Queries().WaitForResult(queryID, 10*time.Second)
	if err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrExtTimeout,
			"Storage capture timeout: "+err.Error(),
			"Ensure the extension is connected and the page has loaded.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)}
	}

	var stateResult map[string]any
	if err := json.Unmarshal(result, &stateResult); err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrInvalidJSON,
			"Failed to parse storage result: "+err.Error(),
			"Check extension logs for errors",
		)}
	}

	if errMsg, ok := stateResult["error"].(string); ok {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrExtError,
			"Storage capture failed: "+errMsg,
			"Check that the tab is accessible.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)}
	}

	response := map[string]any{
		"url":      stateResult["url"],
		"metadata": BuildResponseMetadata(cap, time.Now()),
	}

	includeLocal := params.StorageType == "" || params.StorageType == "local"
	includeSession := params.StorageType == "" || params.StorageType == "session"
	includeCookies := params.StorageType == "" || params.StorageType == "cookies"

	if params.Summary {
		if includeLocal {
			if v, ok := stateResult["localStorage"].(map[string]any); ok {
				response["local_storage"] = summarizeStorageMap(filterStorageMap(v, params.Key))
			}
		}
		if includeSession {
			if v, ok := stateResult["sessionStorage"].(map[string]any); ok {
				response["session_storage"] = summarizeStorageMap(filterStorageMap(v, params.Key))
			}
		}
		if includeCookies {
			if v, ok := stateResult["cookies"].([]any); ok {
				response["cookies"] = summarizeCookies(filterCookies(v, params.Key))
			}
		}
	} else {
		if includeLocal {
			if v, ok := stateResult["localStorage"].(map[string]any); ok {
				response["local_storage"] = filterStorageMap(v, params.Key)
			}
		}
		if includeSession {
			if v, ok := stateResult["sessionStorage"].(map[string]any); ok {
				response["session_storage"] = filterStorageMap(v, params.Key)
			}
		}
		if includeCookies {
			if v, ok := stateResult["cookies"].([]any); ok {
				response["cookies"] = filterCookies(v, params.Key)
			}
		}
	}

	// IndexedDB listing is best-effort (skip if storage_type filter excludes it)
	if params.StorageType == "" {
		if indexeddb, err := idbquery.Listing(cap); err != nil {
			response["indexeddb"] = map[string]any{
				"supported": false,
				"databases": []any{},
			}
			response["indexeddb_error"] = err.Error()
		} else {
			response["indexeddb"] = indexeddb
		}
	}

	return mcp.Succeed(req, "Browser storage", response)
}

// GetIndexedDB returns rows from one IndexedDB object store.
func GetIndexedDB(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Database string `json:"database"`
		Store    string `json:"store"`
		Limit    int    `json:"limit"`
	}
	mcp.LenientUnmarshal(args, &params)

	if params.Database == "" {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrMissingParam,
			"Required parameter 'database' is missing for observe(what='indexeddb')",
			"Add the 'database' parameter and call again.",
			mcp.WithParam("database"),
		)}
	}
	if params.Store == "" {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrMissingParam,
			"Required parameter 'store' is missing for observe(what='indexeddb')",
			"Add the 'store' parameter and call again.",
			mcp.WithParam("store"),
		)}
	}
	params.Limit = clampLimit(params.Limit, 100)

	cap := deps.GetCapture()
	enabled, _, _ := cap.GetTrackingStatus()
	if !enabled {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrNoData,
			"No tab is being tracked. Open the Kaboom extension popup and click 'Track This Tab'.",
			"Track a tab first, then call observe with what='indexeddb'.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)}
	}

	storeData, err := idbquery.Entries(cap, params.Database, params.Store, params.Limit)
	if err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrExtError,
			"IndexedDB inspection failed: "+err.Error(),
			"Ensure the tab is accessible and the database/store names are correct.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)}
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
		"metadata": BuildResponseMetadata(cap, time.Now()),
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

// GetScreenshot captures a screenshot of the current page via the extension.
func GetScreenshot(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	cap := deps.GetCapture()
	enabled, _, _ := cap.GetTrackingStatus()
	if !enabled {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrNoData, "No tab is being tracked. Open the Kaboom extension popup and click 'Track This Tab' on the page you want to monitor. Check observe with what='pilot' for extension status.", "", mcp.WithHint(deps.DiagnosticHintString()))}
	}

	var params struct {
		Format        string `json:"format,omitempty"`
		Quality       int    `json:"quality,omitempty"`
		FullPage      bool   `json:"full_page,omitempty"`
		Selector      string `json:"selector,omitempty"`
		WaitForStable bool   `json:"wait_for_stable,omitempty"`
		SaveTo        string `json:"save_to,omitempty"`
	}
	mcp.LenientUnmarshal(args, &params)

	if params.Format != "" && params.Format != "png" && params.Format != "jpeg" {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrInvalidParam, "Invalid screenshot format: "+params.Format,
			"Use 'png' or 'jpeg'", mcp.WithParam("format"),
		)}
	}

	if params.Quality != 0 && (params.Quality < 1 || params.Quality > 100) {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrInvalidParam, fmt.Sprintf("Invalid quality: %d (must be 1-100)", params.Quality),
			"Use a value between 1 and 100", mcp.WithParam("quality"),
		)}
	}

	screenshotParams := map[string]any{}
	if params.Format != "" {
		screenshotParams["format"] = params.Format
	}
	if params.Quality > 0 {
		screenshotParams["quality"] = params.Quality
	}
	if params.FullPage {
		screenshotParams["full_page"] = true
	}
	if params.Selector != "" {
		screenshotParams["selector"] = params.Selector
	}
	if params.WaitForStable {
		screenshotParams["wait_for_stable"] = true
	}

	queryParams, _ := json.Marshal(screenshotParams)

	queryID, qerr := cap.Queries().CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "screenshot",
			Params: queryParams,
		},
		20*time.Second,
		"",
	)
	if qerr != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrQueueFull, "Command queue full: "+qerr.Error(), "Wait for in-flight commands to complete, then retry.",
			mcp.WithRecoveryToolCall(map[string]any{"tool": "observe", "arguments": map[string]any{"what": "pending_commands"}}),
		)}
	}

	result, err := cap.Queries().WaitForResult(queryID, 20*time.Second)
	if err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrExtTimeout, "Screenshot capture timeout: "+err.Error(), "Ensure the extension is connected and the page has loaded. Try refreshing the page, then retry.", mcp.WithHint(deps.DiagnosticHintString()))}
	}

	var screenshotResult map[string]any
	if err := json.Unmarshal(result, &screenshotResult); err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrInvalidJSON, "Failed to parse screenshot result: "+err.Error(), "Check extension logs for errors")}
	}

	if errMsg, ok := screenshotResult["error"].(string); ok {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrExtError, "Screenshot capture failed: "+errMsg, "Check that the tab is visible and accessible. The extension reported an error.", mcp.WithHint(deps.DiagnosticHintString()))}
	}

	// Extract data_url before building text block to avoid duplicating
	// the large base64 payload in both text and image content blocks.
	var dataURL string
	if du, ok := screenshotResult["data_url"].(string); ok && du != "" {
		dataURL = du
		delete(screenshotResult, "data_url")
	}

	// #386: save_to — copy screenshot to user-specified path
	if params.SaveTo != "" && dataURL != "" {
		if saveErr := saveScreenshotToPath(params.SaveTo, dataURL); saveErr != nil {
			screenshotResult["save_to_error"] = saveErr.Error()
		} else {
			screenshotResult["save_to"] = params.SaveTo
		}
	}

	// Build text response with file path info (backward compatible)
	resp := mcp.Succeed(req, "Screenshot captured", screenshotResult)

	// Append inline image content block if data_url was present
	if dataURL != "" {
		base64Data, mimeType := parseDataURL(dataURL)
		if base64Data != "" {
			resp = mcp.AppendImageToResponse(resp, base64Data, mimeType)
		}
	}

	return resp
}

// parseDataURL extracts the base64 data and MIME type from a data URL.
// Example: "data:image/jpeg;base64,/9j/4AAQ..." -> ("/9j/4AAQ...", "image/jpeg")
// Returns empty strings if the data URL format is invalid.
func parseDataURL(dataURL string) (base64Data, mimeType string) {
	if !strings.HasPrefix(dataURL, "data:") {
		return "", ""
	}
	// Format: data:<mimeType>;base64,<data>
	rest := dataURL[5:] // strip "data:"
	semicolonIdx := strings.Index(rest, ";")
	if semicolonIdx < 0 {
		return "", ""
	}
	mimeType = rest[:semicolonIdx]
	rest = rest[semicolonIdx+1:]
	if !strings.HasPrefix(rest, "base64,") {
		return "", ""
	}
	base64Data = rest[7:] // strip "base64,"
	return base64Data, mimeType
}

// saveScreenshotToPath saves a screenshot data URL to a user-specified file path (#386).
// Creates parent directories if needed. Only allows .png and .jpeg/.jpg extensions.
func saveScreenshotToPath(saveTo string, dataURL string) error {
	// Validate the path is absolute
	absPath, err := filepath.Abs(saveTo)
	if err != nil {
		return fmt.Errorf("invalid save_to path: %w", err)
	}

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(absPath))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return fmt.Errorf("save_to must have .png, .jpg, or .jpeg extension, got %q", ext)
	}

	// Decode the data URL
	b64Data, _ := parseDataURL(dataURL)
	if b64Data == "" {
		return fmt.Errorf("screenshot_save: invalid data URL format. Expected 'data:image/...;base64,...'")
	}

	imageData, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return fmt.Errorf("failed to decode image data: %w", err)
	}

	// Create parent directories
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write the file
	// #nosec G306 -- user-specified path for screenshot save
	if err := os.WriteFile(absPath, imageData, 0o644); err != nil {
		return fmt.Errorf("failed to write screenshot: %w", err)
	}

	return nil
}

// RunA11yAudit executes an accessibility audit via the extension.
func RunA11yAudit(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
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

	enabled, _, _ := deps.GetCapture().GetTrackingStatus()
	if !enabled {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrNoData, "No tab is being tracked. Open the Kaboom extension popup and click 'Track This Tab' on the page you want to monitor. Check observe with what='pilot' for extension status.", "", mcp.WithHint(deps.DiagnosticHintString()))}
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
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrInvalidJSON, "Failed to parse a11y result: "+err.Error(), "Check extension logs for errors")}
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
