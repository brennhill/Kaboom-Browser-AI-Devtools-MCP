// Purpose: Dispatches DOM primitive actions (click, type, select, check, focus, scroll, hover, key_press) and hardware click to the extension.
// Why: Maps each low-level DOM interaction to a pending query with selector/element resolution and timeout handling.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"encoding/json"

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

	// Guarantee: `type` with clear:true and an empty text is the explicit
	// "blank this field" intent (the extension assigns "" to the element), so it
	// must not be rejected as a missing 'text'. This is what lets fill_form send
	// {"selector":"#note","value":""} to empty an input.
	if !isClearFieldRequest(action, params) {
		if errResp, failed := ValidateDOMActionParams(req, action, params.Text, params.Value, params.Name); failed {
			return errResp
		}
	}

	args = normalizeDOMActionArgs(args, action)

	return h.newCommand("dom_"+action).
		correlationPrefix("dom_"+action).
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
		queuedMessage(action+" queued").
		execute(req, args)
}
