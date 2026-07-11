// Purpose: Implements navigation/history browser actions.
// Why: Keep navigation flow logic separate from wrapper entrypoints and utility helpers.
// Docs: docs/features/feature/interact-explore/index.md

package interacthandler

import (
	"encoding/json"
)

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
