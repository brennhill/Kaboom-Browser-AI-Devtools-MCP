// Purpose: Implements navigate_and_document workflow for click-driven page transitions.
// Why: Reduces multi-call navigation documentation loops into one action with URL-change + stability guards.
// Docs: docs/features/feature/interact-explore/index.md

package interacthandler

import (
	"encoding/json"
	"fmt"
	"time"
)

// handleNavigateAndDocument performs click-based navigation, waits for URL/stability,
// then enriches the response with compact page context (url/title/tab_id).
func (h *InteractActionHandler) HandleNavigateAndDocument(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	workflowStart := time.Now()
	trace := make([]WorkflowStep, 0, 4)

	var params struct {
		TimeoutMs        int   `json:"timeout_ms,omitempty"`
		StabilityMs      int   `json:"stability_ms,omitempty"`
		TabID            int   `json:"tab_id,omitempty"`
		WaitForURLChange *bool `json:"wait_for_url_change,omitempty"`
		WaitForStable    *bool `json:"wait_for_stable,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return fail(req, ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}

	waitForURLChange := true
	if params.WaitForURLChange != nil {
		waitForURLChange = *params.WaitForURLChange
	}
	waitForStable := true
	if params.WaitForStable != nil {
		waitForStable = *params.WaitForStable
	}

	validateStart := time.Now()
	if resp, blocked := h.validateNavigateAndDocumentTab(req, params.TabID); blocked {
		trace = append(trace, WorkflowStep{
			Action:   "validate_tab",
			Status:   "error",
			TimingMs: time.Since(validateStart).Milliseconds(),
			Detail:   "tab_id mismatch with tracked tab",
		})
		return h.AppendWorkflowTraceToResponse(resp, "navigate_and_document", trace, workflowStart, "failed")
	}
	trace = append(trace, WorkflowStep{
		Action:   "validate_tab",
		Status:   "success",
		TimingMs: time.Since(validateStart).Milliseconds(),
	})

	beforeURL := h.currentTrackedURL(req)

	clickArgs := filterNavigateAndDocumentClickArgs(args)
	clickStart := time.Now()
	clickResp := h.HandleDOMPrimitive(req, clickArgs, "click")
	trace = append(trace, WorkflowStep{
		Action:   "click",
		Status:   responseStatus(clickResp),
		TimingMs: time.Since(clickStart).Milliseconds(),
	})
	if isErrorResponse(clickResp) {
		return h.AppendWorkflowTraceToResponse(clickResp, "navigate_and_document", trace, workflowStart, "failed")
	}

	// Non-final click response (async correlation pending): return early with
	// correlation metadata so the caller can poll instead of continuing the workflow.
	if isNonFinalResponse(clickResp) {
		return h.AppendWorkflowTraceToResponse(clickResp, "navigate_and_document", trace, workflowStart, "pending")
	}

	if waitForURLChange && beforeURL != "" {
		waitURLStart := time.Now()
		timeoutMs := params.TimeoutMs
		if params.TimeoutMs > 0 {
			var ok bool
			timeoutMs, ok = remainingNavigateAndDocumentTimeoutMs(workflowStart, params.TimeoutMs)
			if !ok {
				timeoutResp := navigateAndDocumentTimeoutBudgetExceeded(req, "wait_for_url_change")
				trace = append(trace, WorkflowStep{
					Action:   "wait_for_url_change",
					Status:   "error",
					TimingMs: time.Since(waitURLStart).Milliseconds(),
					Detail:   "timeout budget exhausted before URL wait stage",
				})
				return h.AppendWorkflowTraceToResponse(timeoutResp, "navigate_and_document", trace, workflowStart, "failed")
			}
		} else if timeoutMs <= 0 {
			timeoutMs = 5000
		}
		lastURL, changed := h.waitForTrackedURLChange(req, beforeURL, timeoutMs)
		if !changed {
			failResp := fail(req, ErrExtTimeout,
				"URL did not change after click within timeout",
				"Increase timeout_ms, disable wait_for_url_change, or verify the click target triggers navigation.",
				withParam("wait_for_url_change"),
			)
			trace = append(trace, WorkflowStep{
				Action:   "wait_for_url_change",
				Status:   "error",
				TimingMs: time.Since(waitURLStart).Milliseconds(),
				Detail:   "tracked URL did not change from baseline",
			})
			return h.AppendWorkflowTraceToResponse(failResp, "navigate_and_document", trace, workflowStart, "failed")
		}
		trace = append(trace, WorkflowStep{
			Action:   "wait_for_url_change",
			Status:   "success",
			TimingMs: time.Since(waitURLStart).Milliseconds(),
			Detail:   lastURL,
		})
	} else if waitForURLChange {
		trace = append(trace, WorkflowStep{
			Action: "wait_for_url_change",
			Status: "skipped",
			Detail: "no pre-click tracked URL available",
		})
	}

	if waitForStable {
		waitStableStart := time.Now()
		waitArgsMap := map[string]any{
			"action": "wait_for_stable",
		}
		if params.StabilityMs > 0 {
			waitArgsMap["stability_ms"] = params.StabilityMs
		}
		if params.TimeoutMs > 0 {
			timeoutMs, ok := remainingNavigateAndDocumentTimeoutMs(workflowStart, params.TimeoutMs)
			if !ok {
				timeoutResp := navigateAndDocumentTimeoutBudgetExceeded(req, "wait_for_stable")
				trace = append(trace, WorkflowStep{
					Action:   "wait_for_stable",
					Status:   "error",
					TimingMs: time.Since(waitStableStart).Milliseconds(),
					Detail:   "timeout budget exhausted before stability stage",
				})
				return h.AppendWorkflowTraceToResponse(timeoutResp, "navigate_and_document", trace, workflowStart, "failed")
			}
			waitArgsMap["timeout_ms"] = timeoutMs
		}
		waitArgs, _ := json.Marshal(waitArgsMap)
		waitResp := h.HandleWaitForStable(req, waitArgs)
		trace = append(trace, WorkflowStep{
			Action:   "wait_for_stable",
			Status:   responseStatus(waitResp),
			TimingMs: time.Since(waitStableStart).Milliseconds(),
		})
		if isErrorResponse(waitResp) {
			return h.AppendWorkflowTraceToResponse(waitResp, "navigate_and_document", trace, workflowStart, "failed")
		}
	} else {
		trace = append(trace, WorkflowStep{
			Action: "wait_for_stable",
			Status: "skipped",
			Detail: "wait_for_stable disabled",
		})
	}

	resp := h.AppendPageContextToResponse(clickResp, req)
	return h.AppendWorkflowTraceToResponse(resp, "navigate_and_document", trace, workflowStart, "success")
}

// filterNavigateAndDocumentClickArgs keeps only click-relevant fields.
func filterNavigateAndDocumentClickArgs(args json.RawMessage) json.RawMessage {
	var raw map[string]any
	if err := json.Unmarshal(args, &raw); err != nil || raw == nil {
		return args
	}

	click := make(map[string]any, 12)
	for _, key := range []string{
		"selector", "scope_selector", "scope_rect", "annotation_rect",
		"element_id", "index", "index_generation", "nth",
		"x", "y",
		"tab_id", "frame", "timeout_ms", "reason",
	} {
		if v, ok := raw[key]; ok {
			click[key] = v
		}
	}
	encoded, err := json.Marshal(click)
	if err != nil {
		return args
	}
	return encoded
}

func (h *InteractActionHandler) currentTrackedURL(req JSONRPCRequest) string {
	_, _, trackedURL := h.deps.Capture().GetTrackingStatus()
	if trackedURL != "" {
		return trackedURL
	}
	if pageCtx, ok := h.readPageContext(req); ok {
		if url, ok := pageCtx["url"].(string); ok {
			return url
		}
	}
	return ""
}

func (h *InteractActionHandler) waitForTrackedURLChange(req JSONRPCRequest, beforeURL string, timeoutMs int) (string, bool) {
	if timeoutMs <= 0 {
		timeoutMs = 5000
	}
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	lastURL := beforeURL
	for time.Now().Before(deadline) {
		lastURL = h.currentTrackedURL(req)
		if lastURL != "" && lastURL != beforeURL {
			return lastURL, true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return lastURL, false
}

// validateNavigateAndDocumentTab ensures workflow-level waits and page context are
// scoped to the currently tracked tab. Unlike plain click, this workflow derives
// post-action state from tracked page metadata.
func (h *InteractActionHandler) validateNavigateAndDocumentTab(req JSONRPCRequest, tabID int) (JSONRPCResponse, bool) {
	if tabID <= 0 {
		return JSONRPCResponse{}, false
	}

	enabled, trackedTabID, _ := h.deps.Capture().GetTrackingStatus()
	if !enabled || trackedTabID <= 0 {
		return fail(req, ErrInvalidParam,
			fmt.Sprintf("navigate_and_document with tab_id=%d requires an actively tracked tab", tabID),
			"Switch tracking to the target tab first (interact what=switch_tab), then retry navigate_and_document.",
			withParam("tab_id"),
		), true
	}
	if trackedTabID == tabID {
		return JSONRPCResponse{}, false
	}

	return fail(req, ErrInvalidParam,
		fmt.Sprintf("navigate_and_document requires tracked tab_id=%d; got tab_id=%d", trackedTabID, tabID),
		"Switch tracking to the target tab first (interact what=switch_tab) or omit tab_id.",
		withParam("tab_id"),
	), true
}

// remainingNavigateAndDocumentTimeoutMs converts total workflow timeout into
// remaining stage timeout. Returns false when budget is exhausted.
func remainingNavigateAndDocumentTimeoutMs(workflowStart time.Time, totalTimeoutMs int) (int, bool) {
	if totalTimeoutMs <= 0 {
		return 0, false
	}
	remaining := totalTimeoutMs - int(time.Since(workflowStart).Milliseconds())
	if remaining <= 0 {
		return 0, false
	}
	return remaining, true
}

func navigateAndDocumentTimeoutBudgetExceeded(req JSONRPCRequest, stage string) JSONRPCResponse {
	return fail(req, ErrExtTimeout,
		fmt.Sprintf("timeout_ms exhausted before %s stage", stage),
		"Increase timeout_ms or disable one of the workflow wait stages.",
		withParam("timeout_ms"),
	)
}
