// interact_workflow.go — The composite interact actions: fill_form(_and_submit),
// navigate_and_wait_for, navigate_and_document, and run_a11y_and_export_sarif.
// Why one file: all four build the same []act.WorkflowStep trace and end in the same
// workflowResult envelope (formerly interact_workflow_types.go), so the shared
// contract lived in a fifth file that none of them could be read without.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

// handleFillFormAndSubmit fills multiple form fields and clicks a submit button.
// Gates (requirePilot, requireExtension, requireTabTracking) are applied by the delegated handlers.
func (h *InteractActionHandler) HandleFillFormAndSubmit(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Fields         []act.FormField `json:"fields"`
		SubmitSelector string          `json:"submit_selector"`
		SubmitIndex    *int            `json:"submit_index,omitempty"`
		TabID          int             `json:"tab_id,omitempty"`
		TimeoutMs      int             `json:"timeout_ms,omitempty"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}
	if len(params.Fields) == 0 {
		return mcp.Fail(req, mcp.ErrMissingParam, "Required parameter 'fields' is empty", "Provide at least one {selector, value} field entry", mcp.WithParam("fields"))
	}
	if params.SubmitSelector == "" && params.SubmitIndex == nil {
		return mcp.Fail(req, mcp.ErrMissingParam, "Required parameter 'submit_selector' or 'submit_index' is missing", "Add the selector or index of the submit button", mcp.WithParam("submit_selector"))
	}
	if params.TimeoutMs <= 0 {
		params.TimeoutMs = 15_000
	}

	trace := make([]act.WorkflowStep, 0, len(params.Fields)+1)
	workflowStart := time.Now()

	trace, errResp := h.fillWorkflowFields(req, "fill_form_and_submit", params.Fields, params.TabID, trace, workflowStart)
	if errResp != nil {
		return *errResp
	}

	clickArgs := map[string]any{
		"action": "click",
		"tab_id": params.TabID,
	}
	if params.SubmitIndex != nil {
		clickArgs["index"] = *params.SubmitIndex
	} else {
		clickArgs["selector"] = params.SubmitSelector
	}
	clickJSON, _ := json.Marshal(clickArgs)

	stepStart := time.Now()
	clickResp := h.HandleDOMPrimitive(req, clickJSON, "click")
	trace = append(trace, act.WorkflowStep{
		Action:   "click_submit",
		Status:   act.ResponseStatus(clickResp),
		TimingMs: time.Since(stepStart).Milliseconds(),
		Detail:   params.SubmitSelector,
	})

	return act.WorkflowResult(req, "fill_form_and_submit", trace, clickResp, workflowStart)
}

// handleFillForm fills multiple form fields without submitting.
// Gates (requirePilot, requireExtension, requireTabTracking) are applied by the delegated handlers.
func (h *InteractActionHandler) HandleFillForm(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Fields    []act.FormField `json:"fields"`
		TabID     int             `json:"tab_id,omitempty"`
		TimeoutMs int             `json:"timeout_ms,omitempty"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}
	if len(params.Fields) == 0 {
		return mcp.Fail(req, mcp.ErrMissingParam, "Required parameter 'fields' is empty", "Provide at least one {selector, value} field entry", mcp.WithParam("fields"))
	}
	if params.TimeoutMs <= 0 {
		params.TimeoutMs = 15_000
	}

	trace := make([]act.WorkflowStep, 0, len(params.Fields))
	workflowStart := time.Now()

	trace, errResp := h.fillWorkflowFields(req, "fill_form", params.Fields, params.TabID, trace, workflowStart)
	if errResp != nil {
		return *errResp
	}

	lastResp := mcp.Succeed(req, "Form filled", map[string]any{
		"status":       "filled",
		"fields_count": len(params.Fields),
	})
	return act.WorkflowResult(req, "fill_form", trace, lastResp, workflowStart)
}

// fillWorkflowFields executes all field entry steps for fill_form* workflows.
func (h *InteractActionHandler) fillWorkflowFields(req mcp.JSONRPCRequest, workflowName string, fields []act.FormField, tabID int, trace []act.WorkflowStep, workflowStart time.Time) ([]act.WorkflowStep, *mcp.JSONRPCResponse) {
	for i, field := range fields {
		if field.Selector == "" && field.Index == nil {
			trace = append(trace, act.WorkflowStep{
				Action: fmt.Sprintf("type[%d]", i),
				Status: "error",
				Detail: "Missing selector and index",
			})
			resp := act.WorkflowResult(req, workflowName, trace, mcp.Fail(req, mcp.ErrMissingParam,
				fmt.Sprintf("Field %d missing 'selector' or 'index'", i),
				"Each field needs a 'selector' or 'index'",
				mcp.WithParam("fields")), workflowStart)
			return trace, &resp
		}

		stepStart := time.Now()
		actionUsed, typeResp := h.executeFillFieldStep(req, field, tabID)
		trace = append(trace, act.WorkflowStep{
			Action:   fmt.Sprintf("%s[%d]", actionUsed, i),
			Status:   act.ResponseStatus(typeResp),
			TimingMs: time.Since(stepStart).Milliseconds(),
			Detail:   workflowFieldLabel(field),
		})
		if act.IsErrorResponse(typeResp) {
			resp := act.WorkflowResult(req, workflowName, trace, typeResp, workflowStart)
			return trace, &resp
		}
	}
	return trace, nil
}

// executeFillFieldStep sends a type action and falls back to select for non-typeable elements.
func (h *InteractActionHandler) executeFillFieldStep(req mcp.JSONRPCRequest, field act.FormField, tabID int) (string, mcp.JSONRPCResponse) {
	typeArgs := map[string]any{
		"action": "type",
		"text":   field.Value,
		"clear":  true,
		"tab_id": tabID,
	}
	if field.Index != nil {
		typeArgs["index"] = *field.Index
	} else {
		typeArgs["selector"] = field.Selector
	}
	argsJSON, _ := json.Marshal(typeArgs)
	typeResp := h.HandleDOMPrimitive(req, argsJSON, "type")
	actionUsed := "type"

	// Fallback: if the element is a <select>, retry with "select" action.
	if IsNotTypeableError(typeResp) {
		selectArgs := map[string]any{
			"action": "select",
			"value":  field.Value,
			"tab_id": tabID,
		}
		if field.Index != nil {
			selectArgs["index"] = *field.Index
		} else {
			selectArgs["selector"] = field.Selector
		}
		selectJSON, _ := json.Marshal(selectArgs)
		typeResp = h.HandleDOMPrimitive(req, selectJSON, "select")
		actionUsed = "select"
	}

	return actionUsed, typeResp
}

func workflowFieldLabel(field act.FormField) string {
	if field.Index != nil {
		return fmt.Sprintf("index:%d", *field.Index)
	}
	return field.Selector
}

// IsNotTypeableError checks whether response payload indicates extension-side not_typeable.
func IsNotTypeableError(resp mcp.JSONRPCResponse) bool {
	if resp.Error != nil || resp.Result == nil {
		return false
	}
	return strings.Contains(string(resp.Result), "not_typeable")
}

// handleNavigateAndWaitFor navigates to a URL, waits for a CSS selector to appear,
// and optionally returns page content — all in one call.
// Gates (requirePilot, requireExtension, requireTabTracking) are applied by the delegated handlers.
func (h *InteractActionHandler) HandleNavigateAndWaitFor(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		URL            string `json:"url"`
		WaitFor        string `json:"wait_for"`
		TabID          int    `json:"tab_id,omitempty"`
		TimeoutMs      int    `json:"timeout_ms,omitempty"`
		IncludeContent bool   `json:"include_content,omitempty"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}
	if resp, blocked := toolresp.RequireString(req, params.URL, "url", "Add 'url' to navigate to"); blocked {
		return resp
	}
	if resp, blocked := toolresp.RequireString(req, params.WaitFor, "wait_for", "Add a CSS selector to wait for after navigation"); blocked {
		return resp
	}
	if params.TimeoutMs <= 0 {
		params.TimeoutMs = 15_000
	}

	trace := make([]act.WorkflowStep, 0, 3)
	workflowStart := time.Now()

	// Step 1: Navigate.
	navArgs := marshalQueryParams(map[string]any{
		"action": "navigate",
		"url":    params.URL,
		"tab_id": params.TabID,
	})
	stepStart := time.Now()
	navResp := h.HandleBrowserActionNavigateImpl(req, navArgs)
	trace = append(trace, act.WorkflowStep{
		Action:   "navigate",
		Status:   act.ResponseStatus(navResp),
		TimingMs: time.Since(stepStart).Milliseconds(),
		Detail:   params.URL,
	})
	if act.IsErrorResponse(navResp) {
		return act.WorkflowResult(req, "navigate_and_wait_for", trace, navResp, workflowStart)
	}

	// Step 2: Wait for selector.
	elapsed := time.Since(workflowStart).Milliseconds()
	waitTimeout := params.TimeoutMs - int(elapsed)
	if waitTimeout < 1000 {
		waitTimeout = 1000
	}
	waitArgs := marshalQueryParams(map[string]any{
		"action":     "wait_for",
		"selector":   params.WaitFor,
		"timeout_ms": waitTimeout,
		"tab_id":     params.TabID,
	})
	stepStart = time.Now()
	waitResp := h.HandleDOMPrimitive(req, waitArgs, "wait_for")
	trace = append(trace, act.WorkflowStep{
		Action:   "wait_for",
		Status:   act.ResponseStatus(waitResp),
		TimingMs: time.Since(stepStart).Milliseconds(),
		Detail:   params.WaitFor,
	})
	if act.IsErrorResponse(waitResp) {
		return act.WorkflowResult(req, "navigate_and_wait_for", trace, waitResp, workflowStart)
	}

	// Step 3: Optional content enrichment.
	if params.IncludeContent {
		stepStart = time.Now()
		navResp = h.deps.EnrichNavigateResponse(navResp, req, params.TabID)
		trace = append(trace, act.WorkflowStep{
			Action:   "get_content",
			Status:   "success",
			TimingMs: time.Since(stepStart).Milliseconds(),
		})
	}

	return act.WorkflowResult(req, "navigate_and_wait_for", trace, navResp, workflowStart)
}

// handleNavigateAndDocument performs click-based navigation, waits for URL/stability,
// then enriches the response with compact page context (url/title/tab_id).
func (h *InteractActionHandler) HandleNavigateAndDocument(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	workflowStart := time.Now()
	trace := make([]act.WorkflowStep, 0, 4)

	var params struct {
		TimeoutMs        int   `json:"timeout_ms,omitempty"`
		StabilityMs      int   `json:"stability_ms,omitempty"`
		TabID            int   `json:"tab_id,omitempty"`
		WaitForURLChange *bool `json:"wait_for_url_change,omitempty"`
		WaitForStable    *bool `json:"wait_for_stable,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
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
		trace = append(trace, act.WorkflowStep{
			Action:   "validate_tab",
			Status:   "error",
			TimingMs: time.Since(validateStart).Milliseconds(),
			Detail:   "tab_id mismatch with tracked tab",
		})
		return h.AppendWorkflowTraceToResponse(resp, "navigate_and_document", trace, workflowStart, "failed")
	}
	trace = append(trace, act.WorkflowStep{
		Action:   "validate_tab",
		Status:   "success",
		TimingMs: time.Since(validateStart).Milliseconds(),
	})

	beforeURL := h.currentTrackedURL(req)

	clickArgs := filterNavigateAndDocumentClickArgs(args)
	clickStart := time.Now()
	clickResp := h.HandleDOMPrimitive(req, clickArgs, "click")
	trace = append(trace, act.WorkflowStep{
		Action:   "click",
		Status:   act.ResponseStatus(clickResp),
		TimingMs: time.Since(clickStart).Milliseconds(),
	})
	if act.IsErrorResponse(clickResp) {
		return h.AppendWorkflowTraceToResponse(clickResp, "navigate_and_document", trace, workflowStart, "failed")
	}

	// Non-final click response (async correlation pending): return early with
	// correlation metadata so the caller can poll instead of continuing the workflow.
	if act.IsNonFinalResponse(clickResp) {
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
				trace = append(trace, act.WorkflowStep{
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
			failResp := mcp.Fail(req, mcp.ErrExtTimeout,
				"URL did not change after click within timeout",
				"Increase timeout_ms, disable wait_for_url_change, or verify the click target triggers navigation.",
				mcp.WithParam("wait_for_url_change"),
			)
			trace = append(trace, act.WorkflowStep{
				Action:   "wait_for_url_change",
				Status:   "error",
				TimingMs: time.Since(waitURLStart).Milliseconds(),
				Detail:   "tracked URL did not change from baseline",
			})
			return h.AppendWorkflowTraceToResponse(failResp, "navigate_and_document", trace, workflowStart, "failed")
		}
		trace = append(trace, act.WorkflowStep{
			Action:   "wait_for_url_change",
			Status:   "success",
			TimingMs: time.Since(waitURLStart).Milliseconds(),
			Detail:   lastURL,
		})
	} else if waitForURLChange {
		trace = append(trace, act.WorkflowStep{
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
				trace = append(trace, act.WorkflowStep{
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
		trace = append(trace, act.WorkflowStep{
			Action:   "wait_for_stable",
			Status:   act.ResponseStatus(waitResp),
			TimingMs: time.Since(waitStableStart).Milliseconds(),
		})
		if act.IsErrorResponse(waitResp) {
			return h.AppendWorkflowTraceToResponse(waitResp, "navigate_and_document", trace, workflowStart, "failed")
		}
	} else {
		trace = append(trace, act.WorkflowStep{
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
		"selector", "scope_selector", "scope_rect",
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

func (h *InteractActionHandler) currentTrackedURL(req mcp.JSONRPCRequest) string {
	_, _, trackedURL := h.deps.Capture().Extension().GetTrackingStatus()
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

func (h *InteractActionHandler) waitForTrackedURLChange(req mcp.JSONRPCRequest, beforeURL string, timeoutMs int) (string, bool) {
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
func (h *InteractActionHandler) validateNavigateAndDocumentTab(req mcp.JSONRPCRequest, tabID int) (mcp.JSONRPCResponse, bool) {
	if tabID <= 0 {
		return mcp.JSONRPCResponse{}, false
	}

	enabled, trackedTabID, _ := h.deps.Capture().Extension().GetTrackingStatus()
	if !enabled || trackedTabID <= 0 {
		return mcp.Fail(req, mcp.ErrInvalidParam,
			fmt.Sprintf("navigate_and_document with tab_id=%d requires an actively tracked tab", tabID),
			"Switch tracking to the target tab first (interact what=switch_tab), then retry navigate_and_document.",
			mcp.WithParam("tab_id"),
		), true
	}
	if trackedTabID == tabID {
		return mcp.JSONRPCResponse{}, false
	}

	return mcp.Fail(req, mcp.ErrInvalidParam,
		fmt.Sprintf("navigate_and_document requires tracked tab_id=%d; got tab_id=%d", trackedTabID, tabID),
		"Switch tracking to the target tab first (interact what=switch_tab) or omit tab_id.",
		mcp.WithParam("tab_id"),
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

func navigateAndDocumentTimeoutBudgetExceeded(req mcp.JSONRPCRequest, stage string) mcp.JSONRPCResponse {
	return mcp.Fail(req, mcp.ErrExtTimeout,
		fmt.Sprintf("timeout_ms exhausted before %s stage", stage),
		"Increase timeout_ms or disable one of the workflow wait stages.",
		mcp.WithParam("timeout_ms"),
	)
}

// handleRunA11yAndExportSARIF runs accessibility audit then exports SARIF in one call.
// Gates (requirePilot, requireExtension, requireTabTracking) are applied by the delegated handlers.
func (h *InteractActionHandler) HandleRunA11yAndExportSARIF(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Scope  string `json:"scope,omitempty"`
		SaveTo string `json:"save_to,omitempty"`
		TabID  int    `json:"tab_id,omitempty"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}

	trace := make([]act.WorkflowStep, 0, 2)
	workflowStart := time.Now()

	// Step 1: Run accessibility audit.
	a11yArgs := marshalQueryParams(map[string]any{
		"what":   "accessibility",
		"scope":  params.Scope,
		"tab_id": params.TabID,
	})
	stepStart := time.Now()
	a11yResp := h.deps.ToolAnalyze(req, a11yArgs)
	trace = append(trace, act.WorkflowStep{
		Action:   "analyze_accessibility",
		Status:   act.ResponseStatus(a11yResp),
		TimingMs: time.Since(stepStart).Milliseconds(),
	})
	if act.IsErrorResponse(a11yResp) {
		return act.WorkflowResult(req, "run_a11y_and_export_sarif", trace, a11yResp, workflowStart)
	}

	// Step 2: Export as SARIF, reusing successful a11y payload to avoid a second blocking query.
	sarifParams := map[string]any{
		"scope":   params.Scope,
		"save_to": params.SaveTo,
	}
	if a11yResult := extractMCPResponseJSONPayload(a11yResp); len(a11yResult) > 0 {
		sarifParams["a11y_result"] = a11yResult
	}
	sarifArgs, _ := json.Marshal(sarifParams)
	stepStart = time.Now()
	sarifResp := h.deps.ToolExportSARIF(req, sarifArgs)
	trace = append(trace, act.WorkflowStep{
		Action:   "generate_sarif",
		Status:   act.ResponseStatus(sarifResp),
		TimingMs: time.Since(stepStart).Milliseconds(),
	})

	return act.WorkflowResult(req, "run_a11y_and_export_sarif", trace, sarifResp, workflowStart)
}

// extractMCPResponseJSONPayload extracts JSON payload from first text block in MCP response.
func extractMCPResponseJSONPayload(resp mcp.JSONRPCResponse) json.RawMessage {
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || len(result.Content) == 0 {
		return nil
	}

	text := strings.TrimSpace(result.Content[0].Text)
	jsonStart := strings.IndexAny(text, "{[")
	if jsonStart < 0 {
		return nil
	}
	payload := strings.TrimSpace(text[jsonStart:])
	if !json.Valid([]byte(payload)) {
		return nil
	}
	return json.RawMessage(payload)
}
