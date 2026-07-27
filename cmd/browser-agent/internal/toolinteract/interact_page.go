// interact_page.go — Interact actions that read from or overlay the page as a whole
// rather than targeting one element: get_readable/get_markdown, explore_page,
// clipboard read/write, draw mode, and the composable pre/post-steps
// (wait_for_stable, auto_dismiss_overlays, action diff, subtitle).
// Why one file: five small files that shared nothing but a filename prefix each;
// what they actually share is the shape — one newCommand(...) dispatch per action
// with no state of their own.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"net/url"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/menus"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
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
func (h *InteractActionHandler) HandleContentExtraction(req JSONRPCRequest, args json.RawMessage, queryType string, correlationPrefix string) JSONRPCResponse {
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

	return h.newCommand(queryType).
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

func (h *InteractActionHandler) HandleGetReadable(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.HandleContentExtraction(req, args, "get_readable", "readable")
}

func (h *InteractActionHandler) HandleGetMarkdown(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.HandleContentExtraction(req, args, "get_markdown", "markdown")
}

// NavigatePageSummaryWait is exported for use by the main package's enrichNavigateResponse.
const NavigatePageSummaryWait = navigatePageSummaryWait

// handleExplorePage handles interact(what="explore_page").
// Creates a pending query for the extension to return combined page metadata,
// interactive elements, readable text, and navigation links in one response.
// If url is provided, the extension navigates first before collecting data.
// Screenshot is appended server-side after the extension returns.
// Post-processes the result to separate menus from ungrouped page elements.
func (h *InteractActionHandler) HandleExplorePage(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		URL         string `json:"url,omitempty"`
		TabID       int    `json:"tab_id,omitempty"`
		VisibleOnly bool   `json:"visible_only,omitempty"`
		Limit       int    `json:"limit,omitempty"`
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
			return mcp.Fail(req, ErrInvalidParam, "Invalid URL: "+params.URL, "Provide a valid http or https URL", withParam("url"))
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return mcp.Fail(req, ErrInvalidParam, "Only http and https URLs are allowed, got: "+parsed.Scheme, "Use an http or https URL", withParam("url"))
		}
	}

	resp := h.newCommand("explore_page").
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
	}

	return resp
}

// enrichExploreWithMenus post-processes an explore_page response to add a
// site_menus section. Elements claimed by menus are removed from the
// interactive_elements list so there is no overlap.
func enrichExploreWithMenus(resp JSONRPCResponse) JSONRPCResponse {
	return mcp.MutateToolResult(resp, func(r *MCPToolResult) {
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

		cfg := menus.DefaultConfig()
		menuResult := menus.Discover(rawElements, cfg)

		claimedIndices := menuResult.ClaimedIndices()

		// Filter interactive_elements to remove menu items
		if len(claimedIndices) > 0 {
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
			data["interactive_elements"] = filtered
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

// handleClipboardRead reads text from the clipboard via navigator.clipboard.readText().
func (h *InteractActionHandler) HandleClipboardRead(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	script := `(async () => {
  try {
    const text = await navigator.clipboard.readText();
    return { text };
  } catch (e) {
    return { error: 'clipboard_read_failed', message: e.message };
  }
})()`

	resp := h.queueExecuteScript(req, args, "exec", 0, 0, "main", script, "clipboard_read", "Clipboard read queued")

	// Record AI action only on success (queueExecuteScript handles guards).
	if !act.IsErrorResponse(resp) {
		h.deps.RecordAIAction("clipboard_read", "", nil)
	}

	return resp
}

// handleClipboardWrite writes text to the clipboard via navigator.clipboard.writeText().
func (h *InteractActionHandler) HandleClipboardWrite(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Text string `json:"text"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}
	if resp, blocked := requireString(req, params.Text, "text", "Add the 'text' parameter with the content to write to clipboard"); blocked {
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

	resp := h.queueExecuteScript(req, args, "exec", 0, 0, "main", script, "clipboard_write", "Clipboard write queued")

	// Record AI action only on success (queueExecuteScript handles guards).
	if !act.IsErrorResponse(resp) {
		h.deps.RecordAIAction("clipboard_write", "", map[string]any{"text_length": len(params.Text)})
	}

	return resp
}

// handleDrawModeStart queues a draw_mode query for the extension to activate draw mode.
func (h *InteractActionHandler) HandleDrawModeStart(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID        int    `json:"tab_id,omitempty"`
		AnnotSession string `json:"annot_session,omitempty"`
	}
	if len(args) > 0 {
		mcp.LenientUnmarshal(args, &params)
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
func (h *InteractActionHandler) HandleWaitForStable(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		StabilityMs int `json:"stability_ms,omitempty"`
		TimeoutMs   int `json:"timeout_ms,omitempty"`
		TabID       int `json:"tab_id,omitempty"`
	}
	mcp.LenientUnmarshal(args, &params)

	// Apply defaults
	if params.StabilityMs <= 0 {
		params.StabilityMs = 500
	}
	if params.TimeoutMs <= 0 {
		params.TimeoutMs = 5000
	}

	// Rewrite args with defaults injected
	var rawArgs map[string]any
	if err := json.Unmarshal(args, &rawArgs); err != nil {
		rawArgs = make(map[string]any)
	}
	rawArgs["stability_ms"] = params.StabilityMs
	rawArgs["timeout_ms"] = params.TimeoutMs
	enrichedArgs, _ := json.Marshal(rawArgs)

	return h.HandleDOMPrimitive(req, enrichedArgs, "wait_for_stable")
}

// handleAutoDismissOverlays is the named handler for the standalone auto_dismiss_overlays action.
// It delegates to the DOM primitive dispatch, which runs consent framework selectors
// followed by the existing dismiss_top_overlay multi-strategy approach on the extension side.
func (h *InteractActionHandler) HandleAutoDismissOverlays(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.HandleDOMPrimitive(req, args, "auto_dismiss_overlays")
}

// queueComposableAutoDismiss queues an auto_dismiss_overlays command as a side effect.
// Used when auto_dismiss=true is passed as a composable param on navigate.
func (h *InteractActionHandler) QueueComposableAutoDismiss(req JSONRPCRequest) {
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
func (h *InteractActionHandler) QueueComposableActionDiff(req JSONRPCRequest) {
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
func (h *InteractActionHandler) QueueComposableWaitForStable(req JSONRPCRequest, stabilityMs int) {
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
func (h *InteractActionHandler) QueueComposableSubtitle(req JSONRPCRequest, text string) {
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
