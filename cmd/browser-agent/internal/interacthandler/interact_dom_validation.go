// Purpose: Implements selector/index resolution and parameter validation for DOM primitive actions.
// Why: Keeps action dispatch focused on orchestration while centralizing validation policy and errors.
// Docs: docs/features/feature/interact-explore/index.md

package interacthandler

import (
	"encoding/json"
	"fmt"
)

// resolveDOMSelectorFromIndex resolves index -> selector for primitive actions that omitted selector/element_id.
func (h *InteractActionHandler) resolveDOMSelectorFromIndex(req JSONRPCRequest, args json.RawMessage, params *DOMPrimitiveParams) (json.RawMessage, JSONRPCResponse, bool) {
	if params.Index == nil || params.Selector != "" || params.ElementID != "" {
		return args, JSONRPCResponse{}, false
	}

	sel, ok, stale, latestGeneration := h.resolveIndexToSelector(req.ClientID, params.TabID, *params.Index, params.IndexGen)
	if stale {
		return args, fail(req, ErrInvalidParam,
			formatIndexGenerationConflict(params.IndexGen, latestGeneration),
			"Re-run interact with what='list_interactive' for the current page context, then retry with the returned index_generation.",
			withParam("index_generation"), withParam("index"),
		), true
	}
	if !ok {
		return args, fail(req, ErrInvalidParam,
			fmt.Sprintf("Element index %d not found for tab_id=%d. Call list_interactive first to refresh the element index for this tab/client scope.", *params.Index, params.TabID),
			"Call interact with what='list_interactive' first (same tab/client scope), then use the returned index.",
			withParam("index"), withParam("tab_id"),
		), true
	}

	params.Selector = sel
	return updateArgsSelector(args, sel), JSONRPCResponse{}, false
}

func validateDOMSelectorRequirement(req JSONRPCRequest, action string, params DOMPrimitiveParams) (JSONRPCResponse, bool) {
	_, selectorOptional := domSelectorOptionalActions[action]
	if params.Selector != "" || params.ElementID != "" || selectorOptional {
		return JSONRPCResponse{}, false
	}

	return fail(req, ErrMissingParam,
		"Required parameter 'selector', 'element_id', or 'index' is missing",
		"Add 'selector' (CSS or semantic selector), or use 'element_id'/'index' from list_interactive results.",
		withParam("selector"),
	), true
}

func validateWaitForConditions(req JSONRPCRequest, action string, params DOMPrimitiveParams) (JSONRPCResponse, bool) {
	if action != "wait_for" {
		return JSONRPCResponse{}, false
	}

	hasSelector := params.Selector != "" || params.ElementID != ""
	hasText := params.Text != ""
	hasURL := params.URLContains != ""

	conditionCount := 0
	if hasSelector || params.Absent {
		conditionCount++
	}
	if hasText {
		conditionCount++
	}
	if hasURL {
		conditionCount++
	}

	if conditionCount == 0 {
		return fail(req, ErrMissingParam,
			"wait_for requires at least one condition: selector, text, or url_contains",
			"Provide 'selector' (wait for element), 'text' (wait for text), or 'url_contains' (wait for URL change).",
			withParam("selector"),
		), true
	}
	if conditionCount > 1 {
		return fail(req, ErrInvalidParam,
			"wait_for conditions are mutually exclusive: use only one of selector, text, or url_contains",
			"Choose a single wait condition per call.",
		), true
	}
	if params.Absent && !hasSelector {
		return fail(req, ErrMissingParam,
			"wait_for with absent requires a selector",
			"Provide 'selector' to specify which element to wait to disappear.",
			withParam("selector"),
		), true
	}
	return JSONRPCResponse{}, false
}

func domActionContextOptions(action, selector string) []func(*StructuredError) {
	opts := []func(*StructuredError){withAction(action)}
	if selector != "" {
		opts = append(opts, withSelector(selector))
	}
	return opts
}

// ValidateDOMActionParams checks action-specific required parameters.
// Returns (response, true) if validation failed, or (zero, false) if valid.
func ValidateDOMActionParams(req JSONRPCRequest, action, text, value, name string) (JSONRPCResponse, bool) {
	rule, ok := domActionRequiredParams[action]
	if !ok {
		return JSONRPCResponse{}, false
	}

	var paramValue string
	switch rule.Field {
	case "text":
		paramValue = text
	case "value":
		paramValue = value
	case "name":
		paramValue = name
	}
	if paramValue == "" {
		return fail(req, ErrMissingParam, rule.Message, rule.Retry, withParam(rule.Field)), true
	}
	return JSONRPCResponse{}, false
}
