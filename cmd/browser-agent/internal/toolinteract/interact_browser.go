// interact_browser.go — Every interact action that drives the browser chrome or the tab's
// rendering surface rather than the page's DOM: navigate/refresh/back/forward, tab
// open/switch/activate/close, highlight, execute_js and the zoom_region capture, plus
// subtitles and the shared queueBrowserAction helper they all funnel through.
// Why one file: this was four files by topic, but the call graph makes it one —
// every handler here funnels through queueBrowserAction, and applySwitchTabTracking
// (formerly interact_tracking.go) has exactly one caller,
// handleSwitchTab, so it is a private continuation of switch_tab
// rather than a tracking subsystem of its own.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/serverdefaults"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// browserActionHandler is a browser-action entry point bound to its receiver at call time.
type browserActionHandler func(*BrowserActions, mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse

// browserActionHandlers is the browser-action table.
//
// A table rather than a switch: the switch's cyclomatic complexity rose with every action
// added, so the two environment-pinning actions would have meant raising a complexity budget
// instead of paying it. A function rather than a package-level map, because a package-level
// map is mutable state any importer could rewrite.
func browserActionHandlers() map[string]browserActionHandler {
	return map[string]browserActionHandler{
		"subtitle":          (*BrowserActions).handleSubtitle,
		"navigate":          (*BrowserActions).handleNavigate,
		"refresh":           (*BrowserActions).handleRefresh,
		"back":              (*BrowserActions).handleBack,
		"forward":           (*BrowserActions).handleForward,
		"new_tab":           (*BrowserActions).handleNewTab,
		"switch_tab":        (*BrowserActions).handleSwitchTab,
		"activate_tab":      (*BrowserActions).handleActivateTab,
		"close_tab":         (*BrowserActions).handleCloseTab,
		"highlight":         (*BrowserActions).handleHighlight,
		"execute_js":        (*BrowserActions).handleExecuteJS,
		"zoom_region":       (*BrowserActions).handleZoomRegion,
		"pin_environment":   (*BrowserActions).handlePinEnvironment,
		"unpin_environment": (*BrowserActions).handleUnpinEnvironment,
	}
}

// Handle is the sole cross-package browser-action boundary. Action-family
// implementations remain private so callers cannot couple to orchestration details.
func (h *BrowserActions) Handle(action string, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	handler, known := browserActionHandlers()[action]
	if !known {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Unsupported browser action", "Use a registered interact action", mcp.WithParam("action"))
	}
	return handler(h, req, args)
}

// stashPerfSnapshot saves the current performance snapshot as a "before" baseline
// for perf_diff computation, keyed by correlation ID.
func (h *BrowserActions) stashPerfSnapshot(correlationID string) {
	_, _, trackedURL := h.deps.Capture().Extension().GetTrackingStatus()
	u, err := url.Parse(trackedURL)
	if err != nil || u.Path == "" {
		return
	}
	if snap, ok := h.deps.Capture().Performance().ByURL(u.Path); ok {
		h.deps.Capture().Performance().StoreBefore(correlationID, snap)
	}
}

func (h *BrowserActions) resolveNavigateURL(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	const insecurePrefix = "kaboom-insecure://"
	if !strings.HasPrefix(strings.ToLower(trimmed), insecurePrefix) {
		return trimmed, nil
	}
	if h.deps.Capture() == nil {
		return "", fmt.Errorf("resolve insecure URL: capture not initialized. Initialize capture before using insecure mode")
	}

	mode, _, _ := h.deps.Capture().Extension().GetSecurityMode()
	if mode != syncruntime.SecurityModeInsecureProxy {
		return "", fmt.Errorf("resolve insecure URL: requires security_mode=insecure_proxy. Set security mode before navigating")
	}

	target := strings.TrimSpace(trimmed[len(insecurePrefix):])
	if target == "" {
		return "", fmt.Errorf("resolve insecure URL: target URL is empty. Provide a URL after the kaboom-insecure:// prefix")
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return "", fmt.Errorf("invalid kaboom-insecure target URL: %v", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("kaboom-insecure target URL must use http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("kaboom-insecure target URL must include host")
	}

	port := serverdefaults.Port
	if h.deps.GetListenPort != nil {
		port = h.deps.GetListenPort()
	}
	return fmt.Sprintf("http://127.0.0.1:%d/insecure-proxy?target=%s", port, url.QueryEscape(target)), nil
}

// browserActionOpts configures the queueBrowserAction helper.
type browserActionOpts struct {
	action         string          // Action name (e.g. "back", "forward", "activate_tab")
	correlationPfx string          // Correlation ID prefix (e.g. "back", "forward")
	params         json.RawMessage // Serialized action params; nil uses `{"action":"<action>"}`
	tabID          int             // Tab ID for the pending query (0 = default)
	skipTabGuard   bool            // If true, skip requireTabTracking guard
	queuedMsg      string          // Queued message for MaybeWaitForCommand
	recordAction   string          // Action type for canonical action recording (defaults to action)
	recordURL      string          // URL for canonical action recording
	recordExtra    map[string]any  // Extra details for canonical action recording
}

// queueBrowserAction is the shared helper for simple browser actions that follow
// the guard → correlate → arm evidence → enqueue → record → wait pattern.
// Uses commandBuilder to eliminate repeated boilerplate.
func (h *BrowserActions) queueBrowserAction(req mcp.JSONRPCRequest, args json.RawMessage, opts browserActionOpts) mcp.JSONRPCResponse {
	actionParams := opts.params
	if actionParams == nil {
		actionParams = marshalQueryParams(map[string]any{"action": opts.action})
	}

	recordAction := opts.recordAction
	if recordAction == "" {
		recordAction = opts.action
	}

	cmd := h.runtime.newCommand(opts.action).
		correlationPrefix(opts.correlationPfx).
		reason(opts.action).
		queryType("browser_action").
		queryParams(actionParams).
		tabID(opts.tabID).
		recordAction(recordAction, opts.recordURL, opts.recordExtra).
		queuedMessage(opts.queuedMsg)

	if opts.skipTabGuard {
		cmd.guards(h.deps.RequirePilot, h.deps.RequireExtension)
	} else {
		cmd.guards(h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking)
	}

	return cmd.execute(req, args)
}

func (h *BrowserActions) handleSubtitle(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Text *string `json:"text"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}

	if params.Text == nil {
		return mcp.Fail(req, mcp.ErrMissingParam, "Required parameter 'text' is missing for subtitle action", "Add the 'text' parameter with subtitle text, or empty string to clear", mcp.WithParam("text"))
	}

	queuedMsg := "Subtitle set"
	if *params.Text == "" {
		queuedMsg = "Subtitle cleared"
	}

	return h.runtime.newCommand("subtitle").
		correlationPrefix("subtitle").
		reason("subtitle").
		queryType("subtitle").
		queryParams(args).
		queuedMessage(queuedMsg).
		execute(req, args)
}

func (h *BrowserActions) handleNavigate(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		URL            string `json:"url"`
		TabID          int    `json:"tab_id,omitempty"`
		IncludeContent bool   `json:"include_content,omitempty"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}

	if resp, blocked := toolresp.RequireString(req, params.URL, "url", "Add the 'url' parameter and call again"); blocked {
		return resp
	}
	resolvedURL, err := h.resolveNavigateURL(params.URL)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInvalidParam,
			err.Error(),
			"Enable configure(what='security_mode', mode='insecure_proxy', confirm=true), or use a standard http(s) URL.",
			mcp.WithParam("url"))
	}

	actionParams := make(map[string]any)
	mcp.LenientUnmarshal(args, &actionParams)
	actionParams["action"] = "navigate"
	// Ensure required URL is present even if caller used alias forms.
	actionParams["url"] = resolvedURL
	actionPayload := marshalQueryParams(actionParams)

	resp := h.runtime.newCommand("navigate").
		correlationPrefix("nav").
		reason("navigate").
		queryType("browser_action").
		queryParams(actionPayload).
		tabID(params.TabID).
		guards(h.deps.RequirePilot, h.deps.RequireExtension).
		preEnqueue(h.stashPerfSnapshot).
		recordAction("navigate", resolvedURL, map[string]any{
			"target_url":    resolvedURL,
			"requested_url": params.URL,
		}).
		queuedMessage("Navigate queued").
		execute(req, args)

	// If include_content is requested and navigate succeeded, enrich with page content.
	if params.IncludeContent {
		resp = h.page.enrichNavigateResponse(resp, req, params.TabID)
	}

	// Include blocked_actions/blocked_reason when CSP restricts — omit entirely
	// when CSP is clear to avoid wasting tokens on normal pages. (#262)
	resp = h.deps.InjectCSPBlockedActions(resp)

	return resp
}

func (h *BrowserActions) handleRefresh(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		TabID int `json:"tab_id,omitempty"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}

	return h.runtime.newCommand("refresh").
		correlationPrefix("refresh").
		reason("refresh").
		queryType("browser_action").
		buildParams(map[string]any{"action": "refresh"}).
		tabID(params.TabID).
		guards(h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking).
		preEnqueue(h.stashPerfSnapshot).
		recordAction("refresh", "", nil).
		queuedMessage("Refresh queued").
		execute(req, args)
}

func (h *BrowserActions) handleBack(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.queueBrowserAction(req, args, browserActionOpts{
		action:         "back",
		correlationPfx: "back",
		queuedMsg:      "Back queued",
	})
}

func (h *BrowserActions) handleForward(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.queueBrowserAction(req, args, browserActionOpts{
		action:         "forward",
		correlationPfx: "forward",
		queuedMsg:      "Forward queued",
	})
}

func (h *BrowserActions) handleNewTab(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		URL string `json:"url"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}

	resolvedURL := params.URL
	if params.URL != "" {
		rewriteURL, err := h.resolveNavigateURL(params.URL)
		if err != nil {
			return mcp.Fail(req, mcp.ErrInvalidParam,
				err.Error(),
				"Enable configure(what='security_mode', mode='insecure_proxy', confirm=true), or use a standard http(s) URL.",
				mcp.WithParam("url"))
		}
		resolvedURL = rewriteURL
	}

	actionParams := make(map[string]any)
	mcp.LenientUnmarshal(args, &actionParams)
	actionParams["action"] = "new_tab"
	if resolvedURL != "" {
		actionParams["url"] = resolvedURL
	}
	actionPayload := marshalQueryParams(actionParams)

	return h.runtime.newCommand("new_tab").
		correlationPrefix("newtab").
		reason("new_tab").
		queryType("browser_action").
		queryParams(actionPayload).
		guards(h.deps.RequirePilot, h.deps.RequireExtension).
		recordAction("new_tab", resolvedURL, map[string]any{
			"target_url":    resolvedURL,
			"requested_url": params.URL,
		}).
		queuedMessage("New tab queued").
		execute(req, args)
}

func (h *BrowserActions) handleSwitchTab(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		TabID      int   `json:"tab_id,omitempty"`
		TabIndex   *int  `json:"tab_index,omitempty"`
		SetTracked *bool `json:"set_tracked,omitempty"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}
	if params.TabID <= 0 && params.TabIndex == nil {
		return mcp.Fail(req, mcp.ErrMissingParam,
			"switch_tab requires tab_id or tab_index",
			"Provide tab_id from observe(what='tabs') or tab_index from your tab list ordering.",
			mcp.WithParam("tab_id"),
			mcp.WithHint("Alternative: provide tab_index"))
	}
	if params.TabIndex != nil && *params.TabIndex < 0 {
		return mcp.Fail(req, mcp.ErrInvalidParam,
			"tab_index must be >= 0",
			"Provide a non-negative tab_index (0-based).",
			mcp.WithParam("tab_index"))
	}

	// Default set_tracked to true so subsequent commands target the new tab.
	setTracked := params.SetTracked == nil || *params.SetTracked

	actionParams := make(map[string]any)
	mcp.LenientUnmarshal(args, &actionParams)
	actionParams["action"] = "switch_tab"
	actionPayload := marshalQueryParams(actionParams)

	// No requireTabTracking gate: switch_tab IS how you establish tracking
	// for an existing tab. The handler calls applySwitchTabTracking on success.
	resp, correlationID := h.runtime.newCommand("switch_tab").
		correlationPrefix("switchtab").
		reason("switch_tab").
		queryType("browser_action").
		queryParams(actionPayload).
		tabID(params.TabID).
		guards(h.deps.RequirePilot, h.deps.RequireExtension).
		recordAction("switch_tab", "", map[string]any{
			"tab_id":    params.TabID,
			"tab_index": params.TabIndex,
		}).
		queuedMessage("Switch tab queued").
		executeWithCorrelation(req, args)

	// After the command completes, update tracked tab state so subsequent
	// commands target the newly activated tab. See issue #271.
	// NOTE: In async mode (sync=false), tracking update is deferred to
	// extension-side persistTrackedTab via the next /sync heartbeat.
	// Server-side update only occurs in sync mode because MaybeWaitForCommand
	// returns immediately when sync=false, so GetCommandResult has no result yet.
	if setTracked && correlationID != "" {
		h.applySwitchTabTracking(correlationID)
	}

	return resp
}

func (h *BrowserActions) handleActivateTab(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.queueBrowserAction(req, args, browserActionOpts{
		action:         "activate_tab",
		correlationPfx: "activate",
		queuedMsg:      "Activate tab queued",
	})
}

func (h *BrowserActions) handleCloseTab(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		TabID int `json:"tab_id,omitempty"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}

	actionParams := make(map[string]any)
	mcp.LenientUnmarshal(args, &actionParams)
	actionParams["action"] = "close_tab"
	actionPayload := marshalQueryParams(actionParams)

	// NOTE: close_tab is gated even with explicit tab_id.
	// Future: allow bypass when tab_id is explicitly provided.
	return h.queueBrowserAction(req, args, browserActionOpts{
		action:         "close_tab",
		correlationPfx: "closetab",
		params:         actionPayload,
		tabID:          params.TabID,
		queuedMsg:      "Close tab queued",
		recordExtra:    map[string]any{"tab_id": params.TabID},
	})
}

func (h *BrowserActions) handleHighlight(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Selector   string `json:"selector"`
		DurationMs int    `json:"duration_ms,omitempty"`
		TabID      int    `json:"tab_id,omitempty"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}

	if resp, blocked := toolresp.RequireString(req, params.Selector, "selector", "Add the 'selector' parameter"); blocked {
		return resp
	}

	return h.runtime.newCommand("highlight").
		correlationPrefix("highlight").
		reason("highlight").
		queryType("highlight").
		queryParams(args).
		tabID(params.TabID).
		guards(h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking).
		recordAction("highlight", "", map[string]any{"selector": params.Selector}).
		queuedMessage("Highlight queued").
		execute(req, args)
}

func (h *BrowserActions) handleExecuteJS(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Script    string `json:"script"`
		TimeoutMs int    `json:"timeout_ms,omitempty"`
		TabID     int    `json:"tab_id,omitempty"`
		World     string `json:"world,omitempty"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}

	if resp, blocked := toolresp.RequireString(req, params.Script, "script", "Add the 'script' parameter and call again"); blocked {
		return resp
	}

	if params.World == "" {
		params.World = "auto"
	}
	if !act.ValidWorldValues[params.World] {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Invalid 'world' value: "+params.World, "Use 'auto' (default, tries main then isolated), 'main' (page JS access), or 'isolated' (bypasses CSP, DOM only)", mcp.WithParam("world"))
	}

	return h.runtime.newCommand("execute_js").
		correlationPrefix("exec").
		reason("execute_js").
		queryType("execute").
		queryParams(args).
		tabID(params.TabID).
		guards(h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking).
		cspGuard(params.World).
		recordAction("execute_js", "", map[string]any{"script_preview": act.TruncateToLen(params.Script, 100)}).
		queuedMessage("Command queued").
		execute(req, args)
}

// applySwitchTabTracking extracts tab_id/url/title from a completed switch_tab
// response and updates the server-side tracked tab state.
// Only updates on success (status=complete, success=true, tab_id present).
//
// NOTE: This only runs in synchronous mode (the default). In async mode
// (background=true), server-side tracking is NOT immediately updated.
// The extension-side persistTrackedTab handles async retarget via the
// next /sync heartbeat. See issue #271.
func (h *BrowserActions) applySwitchTabTracking(correlationID string) {
	cmd, found := h.deps.Capture().Queries().GetCommandResult(correlationID)
	if !found || cmd == nil || cmd.Status != "complete" {
		return
	}

	var result map[string]any
	if err := json.Unmarshal(cmd.Result, &result); err != nil {
		return
	}

	success, _ := result["success"].(bool)
	if !success {
		return
	}

	tabIDFloat, _ := result["tab_id"].(float64)
	tabID := int(tabIDFloat)
	if tabID <= 0 {
		return
	}

	tabURL, _ := result["url"].(string)
	tabTitle, _ := result["title"].(string)
	h.deps.Capture().Extension().UpdateTrackedTab(tabID, tabURL, tabTitle)
}

// =============================================================================
// ZOOM REGION
// =============================================================================

// zoomRegionTimeout bounds a clipped capture. The renderer work is a single Page.captureScreenshot
// plus one write to the screenshots directory, so a slower answer than this means the debugger
// never attached rather than that the image is still rendering.
const zoomRegionTimeout = 20 * time.Second

type zoomRegionParams struct {
	X      *float64 `json:"x"`
	Y      *float64 `json:"y"`
	Width  float64  `json:"width"`
	Height float64  `json:"height"`
	Scale  float64  `json:"scale,omitempty"`
	TabID  int      `json:"tab_id,omitempty"`
}

// handleZoomRegion captures one rectangle of the viewport, optionally supersampled.
//
// It lives with the browser actions rather than the DOM actions because it never touches an
// element: it reads pixels out of the tab's rendering surface, the way observe's screenshot does.
// The capture takes the raw pending-query path instead of the async command builder because the
// extension answers it with the image itself, which must be lifted into an MCP image block
// instead of being spent as a megabyte of base64 inside a text result.
func (h *BrowserActions) handleZoomRegion(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params zoomRegionParams
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}
	if resp, failed := validateZoomRegionParams(req, params); failed {
		return resp
	}
	if resp, blocked := checkGuardsWithOpts(req, []func(*mcp.StructuredError){mcp.WithAction("zoom_region")},
		h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking); blocked {
		return resp
	}
	return h.captureZoomRegion(req, params)
}

func validateZoomRegionParams(req mcp.JSONRPCRequest, params zoomRegionParams) (mcp.JSONRPCResponse, bool) {
	problem := act.ValidateZoomRegion(act.GestureTarget{
		HasX:   params.X != nil,
		HasY:   params.Y != nil,
		Width:  params.Width,
		Height: params.Height,
		Scale:  params.Scale,
	})
	if problem == nil {
		return mcp.JSONRPCResponse{}, false
	}
	return act.GestureParamFailure(req, "zoom_region", problem), true
}

func (h *BrowserActions) captureZoomRegion(req mcp.JSONRPCRequest, params zoomRegionParams) mcp.JSONRPCResponse {
	store := h.deps.Capture()
	if store == nil {
		return mcp.Fail(req, mcp.ErrNoData, "Capture is not initialized", "Restart the Kaboom daemon and retry.")
	}
	query := queries.PendingQuery{
		Type:   "cdp_action",
		Params: marshalQueryParams(zoomRegionQueryFields(params)),
		TabID:  params.TabID,
	}
	queryID, err := store.Queries().CreatePendingQueryWithTimeout(query, zoomRegionTimeout, req.ClientID)
	if err != nil {
		return mcp.Fail(req, mcp.ErrExtError, "Command queue full: "+err.Error(), "Wait for in-flight commands to complete, then retry.")
	}
	raw, err := store.Queries().WaitForResult(queryID, zoomRegionTimeout)
	if err != nil {
		return mcp.Fail(req, mcp.ErrExtTimeout, "zoom_region capture timeout: "+err.Error(),
			"Confirm the extension is connected and the tab is not held by a performance trace, then retry.")
	}
	return zoomRegionResponse(req, raw)
}

func zoomRegionQueryFields(params zoomRegionParams) map[string]any {
	fields := map[string]any{
		"action": "zoom_region",
		"x":      *params.X,
		"y":      *params.Y,
		"width":  params.Width,
		"height": params.Height,
	}
	if params.Scale > 0 {
		fields["scale"] = params.Scale
	}
	return fields
}

// zoomRegionResponse turns the extension's payload into a text block plus a viewable image.
func zoomRegionResponse(req mcp.JSONRPCRequest, raw json.RawMessage) mcp.JSONRPCResponse {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil || payload == nil {
		return mcp.Fail(req, mcp.ErrInvalidJSON, "Could not read the zoom_region result: "+string(raw),
			"Check the extension logs; the capture may have been interrupted.")
	}
	if message, ok := payload["error"].(string); ok && strings.TrimSpace(message) != "" {
		return mcp.Fail(req, mcp.ErrExtError, "zoom_region capture failed: "+strings.TrimSpace(message),
			"Confirm the region is inside the viewport and the tab allows debugger attachment.")
	}
	dataURL := act.ExtractCapturedDataURL(payload)
	resp := mcp.Succeed(req, "Region captured", payload)
	base64Data, mimeType := util.SplitDataURL(dataURL)
	return mcp.AppendImageToResponse(resp, base64Data, mimeType)
}

// ─────────────────────────────────────────────────────────────────────────────
// Environment pinning.
//
// A recorded session replays deterministically only if the environment it ran in is held
// still: the clock, the timezone, the reported location, the viewport and the randomness
// source. Pinning is opt-in per session — an unpinned tab stamps nothing on its actions, and
// the generated artifact then states plainly that it inherits the machine's environment.
// ─────────────────────────────────────────────────────────────────────────────

// environmentPinParams is the caller's request. Every knob is optional; the extension
// reports back exactly which of them the browser actually accepted.
type environmentPinParams struct {
	Environment map[string]any `json:"environment"`
	TabID       int            `json:"tab_id,omitempty"`
}

// handlePinEnvironment holds the tab's environment still for the rest of the session.
func (h *BrowserActions) handlePinEnvironment(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params environmentPinParams
	mcp.LenientUnmarshal(args, &params)

	if len(params.Environment) == 0 {
		// Refused rather than treated as a no-op: a caller that believes it pinned the clock
		// and did not will read the resulting test as deterministic when it is not.
		return mcp.Fail(req, mcp.ErrMissingParam,
			"pin_environment requires an 'environment' object naming at least one knob",
			`Pass environment={"timezone_id":"UTC","clock_epoch_ms":1700000000000,"random_seed":"run-1"}.`,
			mcp.WithParam("environment"))
	}

	return h.runtime.newCommand("env_pin").
		correlationPrefix("env_pin").
		reason("pin_environment").
		queryType("env_pin").
		buildParams(map[string]any{"environment": params.Environment}).
		tabID(params.TabID).
		guards(h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking).
		queuedMessage("pin_environment queued").
		execute(req, args)
}

// handleUnpinEnvironment releases every override pin_environment installed.
func (h *BrowserActions) handleUnpinEnvironment(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params environmentPinParams
	mcp.LenientUnmarshal(args, &params)

	return h.runtime.newCommand("env_unpin").
		correlationPrefix("env_unpin").
		reason("unpin_environment").
		queryType("env_unpin").
		buildParams(map[string]any{}).
		tabID(params.TabID).
		guards(h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking).
		queuedMessage("unpin_environment queued").
		execute(req, args)
}
