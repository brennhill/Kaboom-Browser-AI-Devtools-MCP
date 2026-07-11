// Purpose: Implements navigate_and_wait_for workflow that chains navigation + selector wait.
// Why: Isolates navigation workflow orchestration from form and audit workflows.
// Docs: docs/features/feature/form-filling/index.md

package interacthandler

import (
	"encoding/json"
	"time"
)

// handleNavigateAndWaitFor navigates to a URL, waits for a CSS selector to appear,
// and optionally returns page content — all in one call.
// Gates (requirePilot, requireExtension, requireTabTracking) are applied by the delegated handlers.
func (h *InteractActionHandler) HandleNavigateAndWaitFor(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		URL            string `json:"url"`
		WaitFor        string `json:"wait_for"`
		TabID          int    `json:"tab_id,omitempty"`
		TimeoutMs      int    `json:"timeout_ms,omitempty"`
		IncludeContent bool   `json:"include_content,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}
	if resp, blocked := requireString(req, params.URL, "url", "Add 'url' to navigate to"); blocked {
		return resp
	}
	if resp, blocked := requireString(req, params.WaitFor, "wait_for", "Add a CSS selector to wait for after navigation"); blocked {
		return resp
	}
	if params.TimeoutMs <= 0 {
		params.TimeoutMs = 15_000
	}

	trace := make([]WorkflowStep, 0, 3)
	workflowStart := time.Now()

	// Step 1: Navigate.
	navArgs := buildQueryParams(map[string]any{
		"action": "navigate",
		"url":    params.URL,
		"tab_id": params.TabID,
	})
	stepStart := time.Now()
	navResp := h.HandleBrowserActionNavigateImpl(req, navArgs)
	trace = append(trace, WorkflowStep{
		Action:   "navigate",
		Status:   responseStatus(navResp),
		TimingMs: time.Since(stepStart).Milliseconds(),
		Detail:   params.URL,
	})
	if isErrorResponse(navResp) {
		return workflowResult(req, "navigate_and_wait_for", trace, navResp, workflowStart)
	}

	// Step 2: Wait for selector.
	elapsed := time.Since(workflowStart).Milliseconds()
	waitTimeout := params.TimeoutMs - int(elapsed)
	if waitTimeout < 1000 {
		waitTimeout = 1000
	}
	waitArgs := buildQueryParams(map[string]any{
		"action":     "wait_for",
		"selector":   params.WaitFor,
		"timeout_ms": waitTimeout,
		"tab_id":     params.TabID,
	})
	stepStart = time.Now()
	waitResp := h.HandleDOMPrimitive(req, waitArgs, "wait_for")
	trace = append(trace, WorkflowStep{
		Action:   "wait_for",
		Status:   responseStatus(waitResp),
		TimingMs: time.Since(stepStart).Milliseconds(),
		Detail:   params.WaitFor,
	})
	if isErrorResponse(waitResp) {
		return workflowResult(req, "navigate_and_wait_for", trace, waitResp, workflowStart)
	}

	// Step 3: Optional content enrichment.
	if params.IncludeContent {
		stepStart = time.Now()
		navResp = h.deps.EnrichNavigateResponse(navResp, req, params.TabID)
		trace = append(trace, WorkflowStep{
			Action:   "get_content",
			Status:   "success",
			TimingMs: time.Since(stepStart).Milliseconds(),
		})
	}

	return workflowResult(req, "navigate_and_wait_for", trace, navResp, workflowStart)
}
