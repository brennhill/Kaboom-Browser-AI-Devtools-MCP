// interact_page.go — Page-level and browser-state action execution.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"encoding/json"
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract/pagescripts"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/menus"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
	"net/url"
	"strings"
	"time"
)

const (
	// navigatePageSummaryWait is the time to wait for the page summary content
	// extraction after navigation. The extension-side query uses a 4s timeout,
	// so this must be slightly longer to allow for round-trip overhead.
	navigatePageSummaryWait = 5 * time.Second
)

// handleContentExtraction is the shared handler for get_readable, get_markdown, and page_summary.
// All three use the same pattern: gate checks, timeout validation, create a pending query with
// the dedicated query type, and wait for the content script to respond.
func (h *PageActions) HandleContentExtraction(req mcp.JSONRPCRequest, args json.RawMessage, queryType string, correlationPrefix string) mcp.JSONRPCResponse {
	var params struct {
		TabID     int `json:"tab_id,omitempty"`
		TimeoutMs int `json:"timeout_ms,omitempty"`
	}
	mcp.LenientUnmarshal(args, &params)

	if params.TimeoutMs <= 0 {
		params.TimeoutMs = 10_000
	}
	if params.TimeoutMs > 30_000 {
		params.TimeoutMs = 30_000
	}

	return h.runtime.newCommand(queryType).
		correlationPrefix(correlationPrefix).
		reason(queryType).
		queryType(queryType).
		buildParams(map[string]any{
			"timeout_ms": params.TimeoutMs,
		}).
		tabID(params.TabID).
		guards(h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking).
		queuedMessage(queryType+" queued").
		execute(req, args)
}

func (h *PageActions) HandleGetReadable(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.HandleContentExtraction(req, args, "get_readable", "readable")
}

func (h *PageActions) HandleGetMarkdown(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.HandleContentExtraction(req, args, "get_markdown", "markdown")
}

func (h *PageActions) enrichNavigateResponse(
	resp mcp.JSONRPCResponse,
	req mcp.JSONRPCRequest,
	tabID int,
) mcp.JSONRPCResponse {
	var result mcp.MCPToolResult
	if json.Unmarshal(resp.Result, &result) != nil || result.IsError {
		return resp
	}
	captureStore := h.deps.Capture()
	_, _, tabURL := captureStore.Extension().GetTrackingStatus()
	tabTitle := captureStore.Extension().GetTrackedTabTitle()
	vitals := captureStore.Performance().Entries()
	correlationID := toolresp.NewCorrelationID("nav_content")
	query := queries.PendingQuery{
		Type: "page_summary",
		Params: mcp.SafeMarshal(map[string]any{
			"timeout_ms": 4000,
		}, "{}"),
		TabID: tabID, CorrelationID: correlationID,
	}
	if enqueueResponse, blocked := h.deps.EnqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResponse
	}
	var textContent string
	command, found := captureStore.Queries().WaitForCommand(correlationID, navigatePageSummaryWait)
	if found && command.Status != "pending" && command.Result != nil {
		var summary map[string]any
		if json.Unmarshal(command.Result, &summary) == nil {
			textContent, _ = summary["main_content_preview"].(string)
		}
	}
	if len(result.Content) > 0 {
		enrichment := map[string]any{
			"url": tabURL, "title": tabTitle, "text_content": textContent,
		}
		if len(vitals) > 0 {
			enrichment["vitals"] = vitals[len(vitals)-1]
		}
		result.Content = append(result.Content, mcp.MCPContentBlock{
			Type: "text", Text: "Page content:\n" + string(mcp.SafeMarshal(enrichment, "{}")),
		})
	}
	resp.Result = mcp.SafeMarshal(result, "{}")
	return resp
}

// handleExplorePage handles interact(what="explore_page").
// Creates a pending query for the extension to return combined page metadata,
// interactive elements, readable text, and navigation links in one response.
// If url is provided, the extension navigates first before collecting data.
// Screenshot is appended server-side after the extension returns.
// Post-processes the result to separate menus from ungrouped page elements.
func (h *PageActions) HandleExplorePage(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		URL         string `json:"url,omitempty"`
		TabID       int    `json:"tab_id,omitempty"`
		VisibleOnly bool   `json:"visible_only,omitempty"`
		Limit       int    `json:"limit,omitempty"`
		Verbose     bool   `json:"verbose,omitempty"`
	}
	if len(args) > 0 {
		if resp, stop := mcp.ParseArgs(req, args, &params); stop {
			return resp
		}
	}

	// Validate URL scheme — only http/https allowed (#341 security review)
	if params.URL != "" {
		parsed, err := url.Parse(params.URL)
		if err != nil || parsed.Scheme == "" {
			return mcp.Fail(req, mcp.ErrInvalidParam, "Invalid URL: "+params.URL, "Provide a valid http or https URL", mcp.WithParam("url"))
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return mcp.Fail(req, mcp.ErrInvalidParam, "Only http and https URLs are allowed, got: "+parsed.Scheme, "Use an http or https URL", mcp.WithParam("url"))
		}
	}

	resp := h.runtime.newCommand("explore_page").
		correlationPrefix("explore_page").
		reason("explore_page").
		queryType("explore_page").
		queryParams(args).
		tabID(params.TabID).
		guards(h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking).
		recordAction("explore_page", params.URL, nil).
		queuedMessage("Explore page queued").
		execute(req, args)

	// Post-process: enrich with structured site_menus if the command completed
	if !act.IsErrorResponse(resp) && !isResponseQueued(resp) {
		resp = enrichExploreWithMenus(resp)
		resp = h.AppendScreenshotToResponse(resp, req)
		// After enrichment: menu discovery reads bbox and landmark_tag.
		resp = toolresp.ProjectElementsInResponse(resp, params.Verbose)
	}

	return resp
}

// enrichExploreWithMenus post-processes an explore_page response to add a
// site_menus section. Elements claimed by menus are removed from the
// interactive_elements list so there is no overlap.
func enrichExploreWithMenus(resp mcp.JSONRPCResponse) mcp.JSONRPCResponse {
	return mcp.MutateToolResult(resp, func(r *mcp.MCPToolResult) {
		if len(r.Content) == 0 || r.Content[0].Type != "text" {
			return
		}

		text := r.Content[0].Text
		jsonStart := strings.Index(text, "{")
		if jsonStart < 0 {
			return
		}

		var data map[string]any
		if err := json.Unmarshal([]byte(text[jsonStart:]), &data); err != nil {
			return
		}

		elementsRaw, ok := data["interactive_elements"].([]any)
		if !ok || len(elementsRaw) == 0 {
			return
		}

		// Parse elements into RawElement for the heuristic
		rawElements := parseRawMenuElements(elementsRaw)

		cfg := menus.DefaultConfig()
		menuResult := menus.Discover(rawElements, cfg)

		claimedIndices := menuResult.ClaimedIndices()

		// Filter interactive_elements to remove menu items
		if len(claimedIndices) > 0 {
			data["interactive_elements"] = filterClaimedElements(elementsRaw, claimedIndices)
			if count, ok := data["interactive_count"].(float64); ok {
				data["interactive_count"] = count - float64(len(claimedIndices))
			}
		}

		data["site_menus"] = menuResult

		dataJSON, err := json.Marshal(data)
		if err != nil {
			return
		}
		r.Content[0].Text = text[:jsonStart] + string(dataJSON)
	})
}

func parseRawMenuElements(elementsRaw []any) []menus.RawElement {
	rawElements := make([]menus.RawElement, 0, len(elementsRaw))
	for _, eRaw := range elementsRaw {
		eMap, ok := eRaw.(map[string]any)
		if !ok {
			continue
		}
		bbox := menus.BBox{}
		if bboxMap, ok := eMap["bbox"].(map[string]any); ok {
			bbox.X, _ = bboxMap["x"].(float64)
			bbox.Y, _ = bboxMap["y"].(float64)
			bbox.Width, _ = bboxMap["width"].(float64)
			bbox.Height, _ = bboxMap["height"].(float64)
		}
		idx, _ := eMap["index"].(float64)
		label, _ := eMap["label"].(string)
		tag, _ := eMap["tag"].(string)
		role, _ := eMap["role"].(string)
		href, _ := eMap["href"].(string)
		visible := true
		if v, ok := eMap["visible"].(bool); ok {
			visible = v
		}
		landmarkTag, _ := eMap["landmark_tag"].(string)
		landmarkRole, _ := eMap["landmark_role"].(string)
		rawElements = append(rawElements, menus.RawElement{
			Text:       label,
			Href:       href,
			Tag:        tag,
			Role:       role,
			BBox:       bbox,
			ParentTag:  landmarkTag,
			ParentRole: landmarkRole,
			Visible:    visible,
			Index:      int(idx),
		})
	}
	return rawElements
}

func filterClaimedElements(elementsRaw []any, claimedIndices map[int]bool) []any {
	filtered := make([]any, 0, len(elementsRaw))
	for _, eRaw := range elementsRaw {
		eMap, ok := eRaw.(map[string]any)
		if !ok {
			filtered = append(filtered, eRaw)
			continue
		}
		idx, _ := eMap["index"].(float64)
		if !claimedIndices[int(idx)] {
			filtered = append(filtered, eRaw)
		}
	}
	return filtered
}

// HandleClipboardRead reads clipboard text through the bounded page script, which
// classifies permission, focus, navigation, and context-destruction outcomes itself
// instead of hanging until the injected executor reports a generic execution_timeout.
func (h *PageActions) HandleClipboardRead(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	resp := h.storage.queueExecuteScript(req, args, "exec", "clipboard_read", "Clipboard read queued", scriptCommand{script: pagescripts.ClipboardRead, world: "main"})

	// Record AI action only on success (queueExecuteScript handles guards).
	if !act.IsErrorResponse(resp) {
		h.deps.RecordAIAction("clipboard_read", "", nil)
	}

	return resp
}

// handleClipboardWrite writes text to the clipboard via navigator.clipboard.writeText().
func (h *PageActions) HandleClipboardWrite(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Text string `json:"text"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}
	if resp, blocked := toolresp.RequireString(req, params.Text, "text", "Add the 'text' parameter with the content to write to clipboard"); blocked {
		return resp
	}

	// JSON-encode the text to safely embed it in the script
	textBytes, _ := json.Marshal(params.Text)

	script := `(async () => {
  try {
    await navigator.clipboard.writeText(` + string(textBytes) + `);
    return { success: true };
  } catch (e) {
    return { error: 'clipboard_write_failed', message: e.message };
  }
})()`

	resp := h.storage.queueExecuteScript(req, args, "exec", "clipboard_write", "Clipboard write queued", scriptCommand{script: script, world: "main"})

	// Record AI action only on success (queueExecuteScript handles guards).
	if !act.IsErrorResponse(resp) {
		h.deps.RecordAIAction("clipboard_write", "", map[string]any{"text_length": len(params.Text)})
	}

	return resp
}

// handleDrawModeStart queues a draw_mode query for the extension to activate draw mode.
func (h *PageActions) HandleDrawModeStart(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		TabID        int    `json:"tab_id,omitempty"`
		AnnotSession string `json:"annot_session,omitempty"`
	}
	if len(args) > 0 {
		if resp, stop := mcp.ParseArgs(req, args, &params); stop {
			return resp
		}
	}

	if resp, blocked := checkGuards(req, h.deps.RequirePilot, h.deps.RequireExtension); blocked {
		return resp
	}
	if resp, blocked := h.deps.RequireTabTracking(req); blocked {
		return resp
	}

	correlationID := toolresp.NewCorrelationID("draw")

	queryParams := map[string]string{"action": "start"}
	if params.AnnotSession != "" {
		queryParams["annot_session"] = params.AnnotSession
	}
	// Error impossible: map contains only string values
	queryParamsJSON, _ := json.Marshal(queryParams)

	query := queries.PendingQuery{
		Type:          "draw_mode",
		Params:        queryParamsJSON,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := h.deps.EnqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	// Mark draw started AFTER the query is queued, so WaitForSession's timestamp
	// baseline is never set before the command that triggers the session exists.
	h.deps.MarkDrawStarted()

	// Record AI action
	h.deps.RecordAIAction("draw_mode_start", "", nil)

	return mcp.Succeed(req, "Draw mode activated", map[string]any{
		"status":         "queued",
		"correlation_id": correlationID,
		"message":        "Draw mode activation queued. The user can now draw annotations on the page. Use analyze({what: 'annotations', wait: true}) to block until the user finishes drawing.",
	})
}

// handleWaitForStable is the named handler for the standalone wait_for_stable action.
// It injects default stability_ms and timeout_ms if not provided, then delegates
// to the standard DOM primitive dispatch.
func (h *PageActions) HandleWaitForStable(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		StabilityMs int `json:"stability_ms,omitempty"`
		TimeoutMs   int `json:"timeout_ms,omitempty"`
		TabID       int `json:"tab_id,omitempty"`
	}
	if len(args) > 0 {
		if resp, stop := mcp.ParseArgs(req, args, &params); stop {
			return resp
		}
	}

	// Apply defaults
	if params.StabilityMs <= 0 {
		params.StabilityMs = 500
	}
	if params.TimeoutMs <= 0 {
		params.TimeoutMs = 5000
	}

	// Rewrite args with defaults injected
	rawArgs := make(map[string]any)
	if len(args) > 0 {
		if err := json.Unmarshal(args, &rawArgs); err != nil {
			return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments", "Provide a valid JSON object")
		}
	}
	rawArgs["stability_ms"] = params.StabilityMs
	rawArgs["timeout_ms"] = params.TimeoutMs
	enrichedArgs, _ := json.Marshal(rawArgs)

	return h.dom.HandleDOMPrimitive(req, enrichedArgs, "wait_for_stable")
}

// handleAutoDismissOverlays is the named handler for the standalone auto_dismiss_overlays action.
// It delegates to the DOM primitive dispatch, which runs consent framework selectors
// followed by the existing dismiss_top_overlay multi-strategy approach on the extension side.
func (h *PageActions) HandleAutoDismissOverlays(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.dom.HandleDOMPrimitive(req, args, "auto_dismiss_overlays")
}

// queueComposableAutoDismiss queues an auto_dismiss_overlays command as a side effect.
// Used when auto_dismiss=true is passed as a composable param on navigate.
func (h *PageActions) QueueComposableAutoDismiss(req mcp.JSONRPCRequest) {
	dismissArgs := marshalQueryParams(map[string]any{"action": "auto_dismiss_overlays"})
	correlationID := toolresp.NewCorrelationID("dom_auto_dismiss_overlays")

	query := queries.PendingQuery{
		Type:          "dom_action",
		Params:        dismissArgs,
		CorrelationID: correlationID,
	}
	if _, blocked := h.deps.EnqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return
	}
}

// queueComposableActionDiff queues an action_diff command as a side effect.
// Used when action_diff=true is passed as a composable param on any mutating action.
// The extension instruments a MutationObserver after the main action, captures mutations,
// and returns a structured summary of what changed (overlays, toasts, form errors, etc.).
func (h *PageActions) QueueComposableActionDiff(req mcp.JSONRPCRequest) {
	diffArgs := marshalQueryParams(map[string]any{
		"action":     "action_diff",
		"timeout_ms": 3000,
	})
	correlationID := toolresp.NewCorrelationID("dom_action_diff")

	query := queries.PendingQuery{
		Type:          "dom_action",
		Params:        diffArgs,
		CorrelationID: correlationID,
	}
	if _, blocked := h.deps.EnqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return
	}
}

// queueComposableWaitForStable queues a wait_for_stable command as a side effect.
// Used when wait_for_stable=true is passed as a composable param on navigate or click.
func (h *PageActions) QueueComposableWaitForStable(req mcp.JSONRPCRequest, stabilityMs int) {
	if stabilityMs <= 0 {
		stabilityMs = 500
	}
	timeoutMs := 5000

	stableArgs := marshalQueryParams(map[string]any{
		"action":       "wait_for_stable",
		"stability_ms": stabilityMs,
		"timeout_ms":   timeoutMs,
	})
	correlationID := toolresp.NewCorrelationID("dom_wait_for_stable")

	query := queries.PendingQuery{
		Type:          "dom_action",
		Params:        stableArgs,
		CorrelationID: correlationID,
	}
	if _, blocked := h.deps.EnqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return
	}
}

// queueComposableSubtitle queues a subtitle command as a side effect of another action.
func (h *PageActions) QueueComposableSubtitle(req mcp.JSONRPCRequest, text string) {
	subtitleArgs := marshalQueryParams(map[string]any{"text": text})
	subtitleQuery := queries.PendingQuery{
		Type:          "subtitle",
		Params:        subtitleArgs,
		CorrelationID: toolresp.NewCorrelationID("subtitle"),
	}
	if _, blocked := h.deps.EnqueuePendingQuery(req, subtitleQuery, queries.AsyncCommandTimeout); blocked {
		return
	}
}

var validStorageTypes = map[string]string{
	"localStorage":   "localStorage",
	"sessionStorage": "sessionStorage",
}

type scriptExecutionTarget struct {
	TabID     int    `json:"tab_id,omitempty"`
	TimeoutMs int    `json:"timeout_ms,omitempty"`
	World     string `json:"world,omitempty"`
}

type storageCommandParams struct {
	scriptExecutionTarget
	StorageType string `json:"storage_type"`
}

type keyedStorageCommandParams struct {
	storageCommandParams
	Key string `json:"key"`
}

type setStorageParams struct {
	keyedStorageCommandParams
	Value *string `json:"value"`
}

type cookieCommandParams struct {
	scriptExecutionTarget
	Name   string `json:"name"`
	Domain string `json:"domain,omitempty"`
	Path   string `json:"path,omitempty"`
}

type setCookieParams struct {
	cookieCommandParams
	Value *string `json:"value"`
}

// validateStorageType checks that storage_type is one of the valid storage types.
// Returns the JS expression (e.g. "localStorage") and true on success, or an error response and false on failure.
func validateStorageType(req mcp.JSONRPCRequest, storageType string) (string, mcp.JSONRPCResponse, bool) {
	storageExpr, ok := validStorageTypes[storageType]
	if !ok {
		return "", mcp.Fail(req, mcp.ErrInvalidParam, "Invalid 'storage_type' value: "+storageType, "Use 'localStorage' or 'sessionStorage'", mcp.WithParam("storage_type")), false
	}
	return storageExpr, mcp.JSONRPCResponse{}, true
}

func validateKeyedStorageParams(req mcp.JSONRPCRequest, params keyedStorageCommandParams) (string, mcp.JSONRPCResponse, bool) {
	storageExpr, resp, ok := validateStorageType(req, params.StorageType)
	if !ok {
		return "", resp, false
	}
	if resp, blocked := toolresp.RequireString(req, params.Key, "key", "Add the 'key' parameter and call again"); blocked {
		return "", resp, false
	}
	return storageExpr, mcp.JSONRPCResponse{}, true
}

func (h *StorageActions) HandleSetStorage(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params setStorageParams
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}

	storageExpr, errResp, ok := validateKeyedStorageParams(req, params.keyedStorageCommandParams)
	if !ok {
		return errResp
	}
	if params.Value == nil {
		return mcp.Fail(req, mcp.ErrMissingParam, "Required parameter 'value' is missing for set_storage action", "Add the 'value' parameter and call again", mcp.WithParam("value"))
	}

	script := fmt.Sprintf(`(() => { try { %s.setItem(%s, %s); return { ok: true, action: "set_storage", storage_type: %s, key: %s }; } catch (e) { return { ok: false, error: String((e && e.message) || e) }; } })()`,
		storageExpr, jsQuote(params.Key), jsQuote(*params.Value), jsQuote(params.StorageType), jsQuote(params.Key))
	return h.queueExecuteScript(req, args, "storage_set", "set_storage", "set_storage queued", scriptCommand{script: script, tabID: params.TabID, timeoutMs: params.TimeoutMs, world: params.World})
}

func (h *StorageActions) HandleDeleteStorage(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params keyedStorageCommandParams
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}

	storageExpr, errResp, ok := validateKeyedStorageParams(req, params)
	if !ok {
		return errResp
	}

	script := fmt.Sprintf(`(() => { try { %s.removeItem(%s); return { ok: true, action: "delete_storage", storage_type: %s, key: %s }; } catch (e) { return { ok: false, error: String((e && e.message) || e) }; } })()`,
		storageExpr, jsQuote(params.Key), jsQuote(params.StorageType), jsQuote(params.Key))
	return h.queueExecuteScript(req, args, "storage_del", "delete_storage", "delete_storage queued", scriptCommand{script: script, tabID: params.TabID, timeoutMs: params.TimeoutMs, world: params.World})
}

func (h *StorageActions) HandleClearStorage(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params storageCommandParams
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}

	storageExpr, errResp, ok := validateStorageType(req, params.StorageType)
	if !ok {
		return errResp
	}

	script := fmt.Sprintf(`(() => { try { %s.clear(); return { ok: true, action: "clear_storage", storage_type: %s }; } catch (e) { return { ok: false, error: String((e && e.message) || e) }; } })()`,
		storageExpr, jsQuote(params.StorageType))
	return h.queueExecuteScript(req, args, "storage_clear", "clear_storage", "clear_storage queued", scriptCommand{script: script, tabID: params.TabID, timeoutMs: params.TimeoutMs, world: params.World})
}

func (h *StorageActions) HandleSetCookie(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params setCookieParams
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}
	if resp, blocked := toolresp.RequireString(req, params.Name, "name", "Add the 'name' parameter and call again"); blocked {
		return resp
	}
	if params.Value == nil {
		return mcp.Fail(req, mcp.ErrMissingParam, "Required parameter 'value' is missing for set_cookie action", "Add the 'value' parameter and call again", mcp.WithParam("value"))
	}

	cookie := buildCookie(params.Name+"="+*params.Value, params.Path, params.Domain)

	script := fmt.Sprintf(`(() => { try { document.cookie = %s; return { ok: true, action: "set_cookie", name: %s }; } catch (e) { return { ok: false, error: String((e && e.message) || e) }; } })()`,
		jsQuote(cookie), jsQuote(params.Name))
	return h.queueExecuteScript(req, args, "cookie_set", "set_cookie", "set_cookie queued", scriptCommand{script: script, tabID: params.TabID, timeoutMs: params.TimeoutMs, world: params.World})
}

func (h *StorageActions) HandleDeleteCookie(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params cookieCommandParams
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}
	if resp, blocked := toolresp.RequireString(req, params.Name, "name", "Add the 'name' parameter and call again"); blocked {
		return resp
	}

	cookie := buildCookie(params.Name+"=; expires=Thu, 01 Jan 1970 00:00:00 GMT", params.Path, params.Domain)

	script := fmt.Sprintf(`(() => { try { document.cookie = %s; return { ok: true, action: "delete_cookie", name: %s }; } catch (e) { return { ok: false, error: String((e && e.message) || e) }; } })()`,
		jsQuote(cookie), jsQuote(params.Name))
	return h.queueExecuteScript(req, args, "cookie_del", "delete_cookie", "delete_cookie queued", scriptCommand{script: script, tabID: params.TabID, timeoutMs: params.TimeoutMs, world: params.World})
}

func buildCookie(nameValue, path, domain string) string {
	cookie := nameValue
	if path == "" {
		path = "/"
	}
	cookie += "; path=" + path
	if domain != "" {
		cookie += "; domain=" + domain
	}
	return cookie
}

// scriptCommand carries the execute_js parameters shared by every
// storage/cookie action built on queueExecuteScript.
type scriptCommand struct {
	script    string
	tabID     int
	timeoutMs int
	world     string
}

func (h *StorageActions) queueExecuteScript(
	req mcp.JSONRPCRequest,
	waitArgs json.RawMessage,
	correlationPrefix, reason, queuedMsg string,
	cmd scriptCommand,
) mcp.JSONRPCResponse {
	if cmd.world == "" {
		cmd.world = "auto"
	}
	if !act.ValidWorldValues[cmd.world] {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Invalid 'world' value: "+cmd.world, "Use 'auto' (default), 'main', or 'isolated'", mcp.WithParam("world"))
	}
	if cmd.timeoutMs <= 0 {
		cmd.timeoutMs = 5000
	}

	return h.runtime.newCommand(reason).
		correlationPrefix(correlationPrefix).
		reason(reason).
		queryType("execute").
		buildParams(map[string]any{
			"script":     cmd.script,
			"timeout_ms": cmd.timeoutMs,
			"world":      cmd.world,
			"reason":     reason,
		}).
		tabID(cmd.tabID).
		guards(h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking).
		cspGuard(cmd.world).
		queuedMessage(queuedMsg).
		execute(req, waitArgs)
}

func jsQuote(v string) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// ─────────────────────────────────────────────────────────────────────────────
// The `find` action: natural-language element lookup over the accessibility tree.
//
// Why this lives here: every other targeting path in this package starts from a CSS
// selector or a DOM-derived index. Neither can name a control whose semantics live in
// ARIA rather than markup — a canvas-drawn widget, an aria-label that differs from the
// visible text, a role overridden on a plain div. `find` asks Chrome's accessibility
// tree instead, which is the same view assistive technology sees. It sits beside
// HandleExplorePage because both answer "what is on this page and how do I name it".
// ─────────────────────────────────────────────────────────────────────────────

// axFindTimeoutDefaultMs bounds a full accessibility snapshot plus geometry resolution for
// the ranked candidates. A large page has thousands of AX nodes.
const axFindTimeoutDefaultMs = 10_000

// axFindTimeoutMaxMs caps what a caller may ask for, so one query cannot occupy the
// extension queue indefinitely.
const axFindTimeoutMaxMs = 30_000

// HandleFind resolves a natural-language description to accessibility candidates.
//
// Returns ranked candidates rather than one answer: an ambiguous query must stay ambiguous
// in the response, or the agent has no way to tell "the only match" from "the first of
// several plausible matches" and will blind-click whichever came back first.
func (h *PageActions) HandleFind(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Query     string `json:"query"`
		TabID     int    `json:"tab_id,omitempty"`
		TimeoutMs int    `json:"timeout_ms,omitempty"`
	}
	mcp.LenientUnmarshal(args, &params)

	if params.Query == "" {
		return mcp.Fail(req, mcp.ErrMissingParam,
			"find requires a 'query' describing the element",
			"Pass query='add to cart button' or query='search bar'.",
			mcp.WithParam("query"))
	}

	if params.TimeoutMs <= 0 {
		params.TimeoutMs = axFindTimeoutDefaultMs
	}
	if params.TimeoutMs > axFindTimeoutMaxMs {
		params.TimeoutMs = axFindTimeoutMaxMs
	}

	return h.runtime.newCommand("ax_find").
		correlationPrefix("ax_find").
		reason("find").
		queryType("ax_find").
		buildParams(map[string]any{
			"query":      params.Query,
			"timeout_ms": params.TimeoutMs,
		}).
		tabID(params.TabID).
		guards(h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking).
		queuedMessage("find queued").
		execute(req, args)
}
