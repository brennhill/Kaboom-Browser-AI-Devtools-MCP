// interact_browser.go — Every interact action that drives the browser chrome rather
// than the page's DOM: navigate/refresh/back/forward, tab open/switch/activate/close,
// highlight and execute_js, plus the screenshot/subtitle aliases and the shared
// queueBrowserAction helper they all funnel through.
// Why one file: this was four files by topic, but the call graph makes it one —
// every handler here funnels through queueBrowserAction, and ApplySwitchTabTracking
// (formerly interact_tracking.go) has exactly one caller,
// HandleBrowserActionSwitchTabImpl, so it is a private continuation of switch_tab
// rather than a tracking subsystem of its own.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

const defaultPort = 7890

// stashPerfSnapshotImpl saves the current performance snapshot as a "before" baseline
// for perf_diff computation, keyed by correlation ID.
func (h *InteractActionHandler) stashPerfSnapshotImpl(correlationID string) {
	_, _, trackedURL := h.deps.Capture().GetTrackingStatus()
	u, err := url.Parse(trackedURL)
	if err != nil || u.Path == "" {
		return
	}
	if snap, ok := h.deps.Capture().GetPerformanceSnapshotByURL(u.Path); ok {
		h.deps.Capture().StoreBeforeSnapshot(correlationID, snap)
	}
}

func (h *InteractActionHandler) ResolveNavigateURLImpl(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	const insecurePrefix = "kaboom-insecure://"
	if !strings.HasPrefix(strings.ToLower(trimmed), insecurePrefix) {
		return trimmed, nil
	}
	if h.deps.Capture() == nil {
		return "", fmt.Errorf("resolve insecure URL: capture not initialized. Initialize capture before using insecure mode")
	}

	mode, _, _ := h.deps.Capture().GetSecurityMode()
	if mode != capture.SecurityModeInsecureProxy {
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

	port := defaultPort
	if h.deps.GetListenPort != nil {
		port = h.deps.GetListenPort()
	}
	return fmt.Sprintf("http://127.0.0.1:%d/insecure-proxy?target=%s", port, url.QueryEscape(target)), nil
}

// browserActionOpts configures the queueBrowserAction helper.
type browserActionOpts struct {
	action         string         // Action name (e.g. "back", "forward", "activate_tab")
	correlationPfx string         // Correlation ID prefix (e.g. "back", "forward")
	params         json.RawMessage // Serialized action params; nil uses `{"action":"<action>"}`
	tabID          int            // Tab ID for the pending query (0 = default)
	skipTabGuard   bool           // If true, skip requireTabTracking guard
	queuedMsg      string         // Queued message for MaybeWaitForCommand
	recordAction   string         // Action type for recordAIAction (defaults to action)
	recordURL      string         // URL for recordAIAction
	recordExtra    map[string]any // Extra details for recordAIAction
}

// queueBrowserAction is the shared helper for simple browser actions that follow
// the guard → correlate → arm evidence → enqueue → record → wait pattern.
// Uses commandBuilder to eliminate repeated boilerplate.
func (h *InteractActionHandler) queueBrowserAction(req JSONRPCRequest, args json.RawMessage, opts browserActionOpts) JSONRPCResponse {
	actionParams := opts.params
	if actionParams == nil {
		actionParams = buildQueryParams(map[string]any{"action": opts.action})
	}

	recordAction := opts.recordAction
	if recordAction == "" {
		recordAction = opts.action
	}

	cmd := h.newCommand(opts.action).
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

// handleScreenshotAliasImpl provides backward compatibility for clients that call
// interact({action:"screenshot"}). The canonical API remains observe({what:"screenshot"}).
func (h *InteractActionHandler) HandleScreenshotAliasImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.deps.GetScreenshot(req, args)
}

func (h *InteractActionHandler) HandleSubtitleImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Text *string `json:"text"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	if params.Text == nil {
		return fail(req, ErrMissingParam, "Required parameter 'text' is missing for subtitle action", "Add the 'text' parameter with subtitle text, or empty string to clear", withParam("text"))
	}

	queuedMsg := "Subtitle set"
	if *params.Text == "" {
		queuedMsg = "Subtitle cleared"
	}

	return h.newCommand("subtitle").
		correlationPrefix("subtitle").
		reason("subtitle").
		queryType("subtitle").
		queryParams(args).
		queuedMessage(queuedMsg).
		execute(req, args)
}

func (h *InteractActionHandler) HandleBrowserActionNavigateImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		URL            string `json:"url"`
		TabID          int    `json:"tab_id,omitempty"`
		IncludeContent bool   `json:"include_content,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	if resp, blocked := requireString(req, params.URL, "url", "Add the 'url' parameter and call again"); blocked {
		return resp
	}
	resolvedURL, err := h.ResolveNavigateURLImpl(params.URL)
	if err != nil {
		return fail(req, ErrInvalidParam,
			err.Error(),
			"Enable configure(what='security_mode', mode='insecure_proxy', confirm=true), or use a standard http(s) URL.",
			withParam("url"))
	}

	actionParams := make(map[string]any)
	lenientUnmarshal(args, &actionParams)
	actionParams["action"] = "navigate"
	// Ensure required URL is present even if caller used alias forms.
	actionParams["url"] = resolvedURL
	actionPayload := buildQueryParams(actionParams)

	resp := h.newCommand("navigate").
		correlationPrefix("nav").
		reason("navigate").
		queryType("browser_action").
		queryParams(actionPayload).
		tabID(params.TabID).
		guards(h.deps.RequirePilot, h.deps.RequireExtension).
		preEnqueue(h.stashPerfSnapshotImpl).
		recordAction("navigate", resolvedURL, map[string]any{
			"target_url":    resolvedURL,
			"requested_url": params.URL,
		}).
		queuedMessage("Navigate queued").
		execute(req, args)

	// If include_content is requested and navigate succeeded, enrich with page content.
	if params.IncludeContent {
		resp = h.deps.EnrichNavigateResponse(resp, req, params.TabID)
	}

	// Include blocked_actions/blocked_reason when CSP restricts — omit entirely
	// when CSP is clear to avoid wasting tokens on normal pages. (#262)
	resp = h.deps.InjectCSPBlockedActions(resp)

	return resp
}

func (h *InteractActionHandler) HandleBrowserActionRefreshImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID int `json:"tab_id,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	return h.newCommand("refresh").
		correlationPrefix("refresh").
		reason("refresh").
		queryType("browser_action").
		buildParams(map[string]any{"action": "refresh"}).
		tabID(params.TabID).
		guards(h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking).
		preEnqueue(h.stashPerfSnapshotImpl).
		recordAction("refresh", "", nil).
		queuedMessage("Refresh queued").
		execute(req, args)
}

func (h *InteractActionHandler) HandleBrowserActionBackImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.queueBrowserAction(req, args, browserActionOpts{
		action:         "back",
		correlationPfx: "back",
		queuedMsg:      "Back queued",
	})
}

func (h *InteractActionHandler) HandleBrowserActionForwardImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.queueBrowserAction(req, args, browserActionOpts{
		action:         "forward",
		correlationPfx: "forward",
		queuedMsg:      "Forward queued",
	})
}

func (h *InteractActionHandler) HandleBrowserActionNewTabImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		URL string `json:"url"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	resolvedURL := params.URL
	if params.URL != "" {
		rewriteURL, err := h.ResolveNavigateURLImpl(params.URL)
		if err != nil {
			return fail(req, ErrInvalidParam,
				err.Error(),
				"Enable configure(what='security_mode', mode='insecure_proxy', confirm=true), or use a standard http(s) URL.",
				withParam("url"))
		}
		resolvedURL = rewriteURL
	}

	actionParams := make(map[string]any)
	lenientUnmarshal(args, &actionParams)
	actionParams["action"] = "new_tab"
	if resolvedURL != "" {
		actionParams["url"] = resolvedURL
	}
	actionPayload := buildQueryParams(actionParams)

	return h.newCommand("new_tab").
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

func (h *InteractActionHandler) HandleBrowserActionSwitchTabImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID      int   `json:"tab_id,omitempty"`
		TabIndex   *int  `json:"tab_index,omitempty"`
		SetTracked *bool `json:"set_tracked,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}
	if params.TabID <= 0 && params.TabIndex == nil {
		return fail(req, ErrMissingParam,
			"switch_tab requires tab_id or tab_index",
			"Provide tab_id from observe(what='tabs') or tab_index from your tab list ordering.",
			withParam("tab_id"),
			withHint("Alternative: provide tab_index"))
	}
	if params.TabIndex != nil && *params.TabIndex < 0 {
		return fail(req, ErrInvalidParam,
			"tab_index must be >= 0",
			"Provide a non-negative tab_index (0-based).",
			withParam("tab_index"))
	}

	// Default set_tracked to true so subsequent commands target the new tab.
	setTracked := params.SetTracked == nil || *params.SetTracked

	actionParams := make(map[string]any)
	lenientUnmarshal(args, &actionParams)
	actionParams["action"] = "switch_tab"
	actionPayload := buildQueryParams(actionParams)

	// No requireTabTracking gate: switch_tab IS how you establish tracking
	// for an existing tab. The handler calls applySwitchTabTracking on success.
	resp, correlationID := h.newCommand("switch_tab").
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
		h.ApplySwitchTabTracking(correlationID)
	}

	return resp
}

func (h *InteractActionHandler) HandleActivateTabImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.queueBrowserAction(req, args, browserActionOpts{
		action:         "activate_tab",
		correlationPfx: "activate",
		queuedMsg:      "Activate tab queued",
	})
}

func (h *InteractActionHandler) HandleBrowserActionCloseTabImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID int `json:"tab_id,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	actionParams := make(map[string]any)
	lenientUnmarshal(args, &actionParams)
	actionParams["action"] = "close_tab"
	actionPayload := buildQueryParams(actionParams)

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

// validWorldValues delegates to the interact package.
var validWorldValues = act.ValidWorldValues

// truncateToLen delegates to the interact package.
var truncateToLen = act.TruncateToLen

func (h *InteractActionHandler) HandleHighlightImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Selector   string `json:"selector"`
		DurationMs int    `json:"duration_ms,omitempty"`
		TabID      int    `json:"tab_id,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	if resp, blocked := requireString(req, params.Selector, "selector", "Add the 'selector' parameter"); blocked {
		return resp
	}

	return h.newCommand("highlight").
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

func (h *InteractActionHandler) HandleExecuteJSImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Script    string `json:"script"`
		TimeoutMs int    `json:"timeout_ms,omitempty"`
		TabID     int    `json:"tab_id,omitempty"`
		World     string `json:"world,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	if resp, blocked := requireString(req, params.Script, "script", "Add the 'script' parameter and call again"); blocked {
		return resp
	}

	if params.World == "" {
		params.World = "auto"
	}
	if !validWorldValues[params.World] {
		return fail(req, ErrInvalidParam, "Invalid 'world' value: "+params.World, "Use 'auto' (default, tries main then isolated), 'main' (page JS access), or 'isolated' (bypasses CSP, DOM only)", withParam("world"))
	}

	return h.newCommand("execute_js").
		correlationPrefix("exec").
		reason("execute_js").
		queryType("execute").
		queryParams(args).
		tabID(params.TabID).
		guards(h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking).
		cspGuard(params.World).
		recordAction("execute_js", "", map[string]any{"script_preview": truncateToLen(params.Script, 100)}).
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
func (h *InteractActionHandler) ApplySwitchTabTracking(correlationID string) {
	cmd, found := h.deps.Capture().GetCommandResult(correlationID)
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
	h.deps.Capture().UpdateTrackedTab(tabID, tabURL, tabTitle)
}
