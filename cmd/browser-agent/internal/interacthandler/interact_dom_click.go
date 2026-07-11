// Purpose: Handles coordinate-driven click flows that route through CDP actions.
// Why: Separates hardware-click behavior from generic DOM selector-based primitives.
// Docs: docs/features/feature/interact-explore/index.md

package interacthandler

import (
	"encoding/json"
)

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
		queuedMessage(action+" queued").
		execute(req, args)
}
