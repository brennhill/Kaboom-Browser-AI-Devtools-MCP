// Purpose: Implements fill_form and fill_form_and_submit workflow handlers.
// Why: Keeps form-focused workflow orchestration isolated and DRY.
// Docs: docs/features/feature/form-filling/index.md

package toolinteract

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// handleFillFormAndSubmit fills multiple form fields and clicks a submit button.
// Gates (requirePilot, requireExtension, requireTabTracking) are applied by the delegated handlers.
func (h *InteractActionHandler) HandleFillFormAndSubmit(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Fields         []FormField `json:"fields"`
		SubmitSelector string      `json:"submit_selector"`
		SubmitIndex    *int        `json:"submit_index,omitempty"`
		TabID          int         `json:"tab_id,omitempty"`
		TimeoutMs      int         `json:"timeout_ms,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}
	if len(params.Fields) == 0 {
		return fail(req, ErrMissingParam, "Required parameter 'fields' is empty", "Provide at least one {selector, value} field entry", withParam("fields"))
	}
	if params.SubmitSelector == "" && params.SubmitIndex == nil {
		return fail(req, ErrMissingParam, "Required parameter 'submit_selector' or 'submit_index' is missing", "Add the selector or index of the submit button", withParam("submit_selector"))
	}
	if params.TimeoutMs <= 0 {
		params.TimeoutMs = 15_000
	}

	trace := make([]WorkflowStep, 0, len(params.Fields)+1)
	workflowStart := time.Now()

	trace, errResp := h.fillWorkflowFields(req, "fill_form_and_submit", params.Fields, params.TabID, params.TimeoutMs, trace, workflowStart)
	if errResp != nil {
		return *errResp
	}

	// The submit click is part of the same budget: never submit a form the
	// workflow no longer has time to finish.
	if _, ok := remainingNavigateAndDocumentTimeoutMs(workflowStart, params.TimeoutMs); !ok {
		trace = append(trace, fillWorkflowTimeoutStep("click_submit"))
		return workflowResult(req, "fill_form_and_submit", trace,
			fillWorkflowTimeoutExceeded(req, "click_submit"), workflowStart)
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
	trace = append(trace, WorkflowStep{
		Action:   "click_submit",
		Status:   responseStatus(clickResp),
		TimingMs: time.Since(stepStart).Milliseconds(),
		Detail:   params.SubmitSelector,
	})

	return workflowResult(req, "fill_form_and_submit", trace, clickResp, workflowStart)
}

// handleFillForm fills multiple form fields without submitting.
// Gates (requirePilot, requireExtension, requireTabTracking) are applied by the delegated handlers.
func (h *InteractActionHandler) HandleFillForm(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Fields    []FormField `json:"fields"`
		TabID     int         `json:"tab_id,omitempty"`
		TimeoutMs int         `json:"timeout_ms,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}
	if len(params.Fields) == 0 {
		return fail(req, ErrMissingParam, "Required parameter 'fields' is empty", "Provide at least one {selector, value} field entry", withParam("fields"))
	}
	if params.TimeoutMs <= 0 {
		params.TimeoutMs = 15_000
	}

	trace := make([]WorkflowStep, 0, len(params.Fields))
	workflowStart := time.Now()

	trace, errResp := h.fillWorkflowFields(req, "fill_form", params.Fields, params.TabID, params.TimeoutMs, trace, workflowStart)
	if errResp != nil {
		return *errResp
	}

	lastResp := succeed(req, "Form filled", map[string]any{
		"status":       "filled",
		"fields_count": len(params.Fields),
	})
	return workflowResult(req, "fill_form", trace, lastResp, workflowStart)
}

// fillWorkflowTimeoutExceeded reports an exhausted fill_form* timeout_ms budget,
// naming the step the workflow was about to run.
func fillWorkflowTimeoutExceeded(req JSONRPCRequest, step string) JSONRPCResponse {
	return fail(req, ErrExtTimeout,
		fmt.Sprintf("timeout_ms exhausted before %s step", step),
		"Increase timeout_ms, or split the form into smaller fill_form calls.",
		withParam("timeout_ms"),
	)
}

// fillWorkflowTimeoutStep is the trace entry recorded for the step that the
// exhausted budget prevented from running.
func fillWorkflowTimeoutStep(step string) WorkflowStep {
	return WorkflowStep{
		Action: step,
		Status: "error",
		Detail: "timeout_ms budget exhausted",
	}
}

// fillWorkflowFields executes all field entry steps for fill_form* workflows.
// timeoutMs bounds the whole workflow: the same total-budget rule the navigate
// workflows apply (remainingNavigateAndDocumentTimeoutMs).
func (h *InteractActionHandler) fillWorkflowFields(req JSONRPCRequest, workflowName string, fields []FormField, tabID, timeoutMs int, trace []WorkflowStep, workflowStart time.Time) ([]WorkflowStep, *JSONRPCResponse) {
	for i, field := range fields {
		stepName := fmt.Sprintf("type[%d]", i)
		if _, ok := remainingNavigateAndDocumentTimeoutMs(workflowStart, timeoutMs); !ok {
			trace = append(trace, fillWorkflowTimeoutStep(stepName))
			resp := workflowResult(req, workflowName, trace, fillWorkflowTimeoutExceeded(req, stepName), workflowStart)
			return trace, &resp
		}

		if field.Selector == "" && field.Index == nil {
			trace = append(trace, WorkflowStep{
				Action: stepName,
				Status: "error",
				Detail: "Missing selector and index",
			})
			resp := workflowResult(req, workflowName, trace, fail(req, ErrMissingParam,
				fmt.Sprintf("Field %d missing 'selector' or 'index'", i),
				"Each field needs a 'selector' or 'index'",
				withParam("fields")), workflowStart)
			return trace, &resp
		}

		stepStart := time.Now()
		actionUsed, typeResp := h.executeFillFieldStep(req, field, tabID)
		trace = append(trace, WorkflowStep{
			Action:   fmt.Sprintf("%s[%d]", actionUsed, i),
			Status:   responseStatus(typeResp),
			TimingMs: time.Since(stepStart).Milliseconds(),
			Detail:   workflowFieldLabel(field),
		})
		if isErrorResponse(typeResp) {
			resp := workflowResult(req, workflowName, trace, typeResp, workflowStart)
			return trace, &resp
		}
	}
	return trace, nil
}

// executeFillFieldStep sends a type action and falls back to select for non-typeable elements.
func (h *InteractActionHandler) executeFillFieldStep(req JSONRPCRequest, field FormField, tabID int) (string, JSONRPCResponse) {
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

func workflowFieldLabel(field FormField) string {
	if field.Index != nil {
		return fmt.Sprintf("index:%d", *field.Index)
	}
	return field.Selector
}

// IsNotTypeableError checks whether response payload indicates extension-side not_typeable.
func IsNotTypeableError(resp JSONRPCResponse) bool {
	if resp.Error != nil || resp.Result == nil {
		return false
	}
	return strings.Contains(string(resp.Result), "not_typeable")
}
