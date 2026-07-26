// interact_dom.go — The DOM primitive action: parse its parameters, validate them,
// resolve an index to a selector, then dispatch either the extension DOM path or
// the CDP hardware-click path.
// Why one file: parsing, validation and dispatch were three files, but they share
// DOMPrimitiveParams and are only ever entered through HandleDOMPrimitive — no
// caller outside this file uses the validators except HandleDOMPrimitive itself.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"encoding/json"
	"fmt"

	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

// domPrimitiveActions delegates to the interact package.
var domPrimitiveActions = act.DOMPrimitiveActions

// domActionRequiredParams delegates to the interact package.
var domActionRequiredParams = act.DOMActionRequiredParams

// domActionToReproType delegates to the interact package.
var domActionToReproType = act.DOMActionToReproType

// parseSelectorForReproduction delegates to the interact package.
var parseSelectorForReproduction = act.ParseSelectorForReproduction

// normalizeDOMActionArgs rewrites interact args so extension-facing dom_action
// payloads always carry canonical "action", while preserving user-facing "what".
func normalizeDOMActionArgs(args json.RawMessage, action string) json.RawMessage {
	var payload map[string]any
	if err := json.Unmarshal(args, &payload); err != nil || payload == nil {
		payload = map[string]any{}
	}
	payload["action"] = action
	if _, hasScopeRect := payload["scope_rect"]; !hasScopeRect {
		if annotationRect, hasAnnotationRect := payload["annotation_rect"]; hasAnnotationRect {
			payload["scope_rect"] = annotationRect
		}
	}
	// #448: Convert near_x/near_y/near_radius to scope_rect for region-scoped discovery
	if _, hasScopeRect := payload["scope_rect"]; !hasScopeRect {
		nearX, hasX := toFloat64(payload["near_x"])
		nearY, hasY := toFloat64(payload["near_y"])
		nearR, hasR := toFloat64(payload["near_radius"])
		if hasX && hasY && hasR && nearR > 0 {
			payload["scope_rect"] = map[string]any{
				"x":      nearX - nearR,
				"y":      nearY - nearR,
				"width":  nearR * 2,
				"height": nearR * 2,
			}
		}
	}
	normalized, err := json.Marshal(payload)
	if err != nil {
		return args
	}
	return normalized
}

// toFloat64 extracts a float64 from an any value (handles int, float64, json.Number).
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func (h *InteractActionHandler) HandleDOMPrimitive(req JSONRPCRequest, args json.RawMessage, action string) JSONRPCResponse {
	params, err := ParseDOMPrimitiveParams(args)
	if err != nil {
		return fail(req, ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}

	// If x/y coordinates provided on a click action, escalate to CDP for hardware-level click
	if action == "click" && params.X != nil && params.Y != nil {
		return h.HandleCDPClick(req, args, action, *params.X, *params.Y, params.TabID)
	}

	var failed bool
	var errResp JSONRPCResponse
	args, errResp, failed = h.resolveDOMSelectorFromIndex(req, args, &params)
	if failed {
		return errResp
	}

	if errResp, failed := validateDOMSelectorRequirement(req, action, params); failed {
		return errResp
	}

	if errResp, failed := validateWaitForConditions(req, action, params); failed {
		return errResp
	}

	if errResp, failed := ValidateDOMActionParams(req, action, params.Text, params.Value, params.Name); failed {
		return errResp
	}

	args = normalizeDOMActionArgs(args, action)

	return h.newCommand("dom_" + action).
		correlationPrefix("dom_" + action).
		reason(action).
		queryType("dom_action").
		queryParams(args).
		tabID(params.TabID).
		guardsWithOpts(
			domActionContextOptions(action, params.Selector),
			h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking,
		).
		postEnqueue(func() {
			h.deps.RecordDOMPrimitiveAction(action, params.Selector, params.Text, params.Value)
		}).
		queuedMessage(action + " queued").
		execute(req, args)
}

type DOMPrimitiveParams struct {
	Selector      string   `json:"selector"`
	ScopeSelector string   `json:"scope_selector,omitempty"`
	ElementID     string   `json:"element_id,omitempty"`
	Index         *int     `json:"index,omitempty"`
	Nth           *int     `json:"nth,omitempty"`
	IndexGen      string   `json:"index_generation,omitempty"`
	Text          string   `json:"text,omitempty"`
	Value         string   `json:"value,omitempty"`
	Direction     string   `json:"direction,omitempty"`
	Clear         bool     `json:"clear,omitempty"`
	Checked       *bool    `json:"checked,omitempty"`
	Name          string   `json:"name,omitempty"`
	TimeoutMs     int      `json:"timeout_ms,omitempty"`
	TabID         int      `json:"tab_id,omitempty"`
	Analyze       bool     `json:"analyze,omitempty"`
	X             *float64 `json:"x,omitempty"`
	Y             *float64 `json:"y,omitempty"`
	URLContains   string   `json:"url_contains,omitempty"`
	Absent        bool     `json:"absent,omitempty"`
	Structured    bool     `json:"structured,omitempty"`
}

type hardwareClickParams struct {
	X     *float64 `json:"x"`
	Y     *float64 `json:"y"`
	TabID int      `json:"tab_id,omitempty"`
}

var domSelectorOptionalActions = map[string]struct{}{
	"open_composer":          {},
	"submit_active_composer": {},
	"confirm_top_dialog":     {},
	"dismiss_top_overlay":    {},
	"auto_dismiss_overlays":  {},
	"wait_for_stable":        {},
	"key_press":              {},
	"wait_for":               {},
}

func ParseDOMPrimitiveParams(args json.RawMessage) (DOMPrimitiveParams, error) {
	var params DOMPrimitiveParams
	if err := json.Unmarshal(args, &params); err != nil {
		return DOMPrimitiveParams{}, err
	}
	return params, nil
}

func parseHardwareClickParams(args json.RawMessage) (hardwareClickParams, error) {
	var params hardwareClickParams
	if err := json.Unmarshal(args, &params); err != nil {
		return hardwareClickParams{}, err
	}
	return params, nil
}

func updateArgsSelector(args json.RawMessage, selector string) json.RawMessage {
	var rawArgs map[string]json.RawMessage
	if json.Unmarshal(args, &rawArgs) != nil {
		return args
	}
	selectorJSON, err := json.Marshal(selector)
	if err != nil {
		return args
	}
	rawArgs["selector"] = selectorJSON
	updated, err := json.Marshal(rawArgs)
	if err != nil {
		return args
	}
	return updated
}

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

// handleHardwareClick dispatches a coordinate-based click via CDP Input.dispatchMouseEvent.
// This gives LLMs an explicit "I see coordinates in a screenshot, click there" path.
func (h *InteractActionHandler) HandleHardwareClick(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	params, err := parseHardwareClickParams(args)
	if err != nil {
		return fail(req, ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}

	if params.X == nil {
		return fail(req, ErrMissingParam, "Required parameter 'x' is missing", "Add the 'x' coordinate (pixels from left)", withParam("x"))
	}
	if params.Y == nil {
		return fail(req, ErrMissingParam, "Required parameter 'y' is missing", "Add the 'y' coordinate (pixels from top)", withParam("y"))
	}

	return h.HandleCDPClick(req, args, "hardware_click", *params.X, *params.Y, params.TabID)
}

// handleCDPClick creates a cdp_action query for a hardware-level click at coordinates.
func (h *InteractActionHandler) HandleCDPClick(req JSONRPCRequest, args json.RawMessage, action string, x, y float64, tabID int) JSONRPCResponse {
	return h.newCommand("cdp_click").
		correlationPrefix("cdp_click").
		reason(action).
		queryType("cdp_action").
		buildParams(map[string]any{
			"action": "click",
			"x":      x,
			"y":      y,
		}).
		tabID(tabID).
		guardsWithOpts(
			[]func(*StructuredError){withAction(action)},
			h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking,
		).
		recordAction(action, "", map[string]any{"x": x, "y": y, "method": "cdp"}).
		queuedMessage(action + " queued").
		execute(req, args)
}
