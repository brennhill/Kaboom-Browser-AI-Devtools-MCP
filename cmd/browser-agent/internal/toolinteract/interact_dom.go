// interact_dom.go — DOM primitive and list_interactive actions: parse, validate,
// resolve indexed selectors, and dispatch through extension or CDP paths.
// Why one file: parsing, validation and dispatch were three files, but they share
// DOMPrimitiveParams and are only ever entered through HandleDOMPrimitive — no
// caller outside this file uses the validators except HandleDOMPrimitive itself.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract/elemindex"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
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

func (h *InteractActionHandler) HandleDOMPrimitive(req mcp.JSONRPCRequest, args json.RawMessage, action string) mcp.JSONRPCResponse {
	params, err := ParseDOMPrimitiveParams(args)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}

	// If x/y coordinates provided on a click action, escalate to CDP for hardware-level click
	if action == "click" && params.X != nil && params.Y != nil {
		return h.HandleCDPClick(req, args, action, *params.X, *params.Y, params.TabID)
	}

	var failed bool
	var errResp mcp.JSONRPCResponse
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
func (h *InteractActionHandler) resolveDOMSelectorFromIndex(req mcp.JSONRPCRequest, args json.RawMessage, params *DOMPrimitiveParams) (json.RawMessage, mcp.JSONRPCResponse, bool) {
	if params.Index == nil || params.Selector != "" || params.ElementID != "" {
		return args, mcp.JSONRPCResponse{}, false
	}

	sel, ok, stale, latestGeneration := h.resolveIndexToSelector(req.ClientID, params.TabID, *params.Index, params.IndexGen)
	if stale {
		return args, mcp.Fail(req, mcp.ErrInvalidParam,
			elemindex.FormatGenerationConflict(params.IndexGen, latestGeneration),
			"Re-run interact with what='list_interactive' for the current page context, then retry with the returned index_generation.",
			mcp.WithParam("index_generation"), mcp.WithParam("index"),
		), true
	}
	if !ok {
		return args, mcp.Fail(req, mcp.ErrInvalidParam,
			fmt.Sprintf("Element index %d not found for tab_id=%d. Call list_interactive first to refresh the element index for this tab/client scope.", *params.Index, params.TabID),
			"Call interact with what='list_interactive' first (same tab/client scope), then use the returned index.",
			mcp.WithParam("index"), mcp.WithParam("tab_id"),
		), true
	}

	params.Selector = sel
	return updateArgsSelector(args, sel), mcp.JSONRPCResponse{}, false
}

func validateDOMSelectorRequirement(req mcp.JSONRPCRequest, action string, params DOMPrimitiveParams) (mcp.JSONRPCResponse, bool) {
	_, selectorOptional := domSelectorOptionalActions[action]
	if params.Selector != "" || params.ElementID != "" || selectorOptional {
		return mcp.JSONRPCResponse{}, false
	}

	return mcp.Fail(req, mcp.ErrMissingParam,
		"Required parameter 'selector', 'element_id', or 'index' is missing",
		"Add 'selector' (CSS or semantic selector), or use 'element_id'/'index' from list_interactive results.",
		mcp.WithParam("selector"),
	), true
}

func validateWaitForConditions(req mcp.JSONRPCRequest, action string, params DOMPrimitiveParams) (mcp.JSONRPCResponse, bool) {
	if action != "wait_for" {
		return mcp.JSONRPCResponse{}, false
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
		return mcp.Fail(req, mcp.ErrMissingParam,
			"wait_for requires at least one condition: selector, text, or url_contains",
			"Provide 'selector' (wait for element), 'text' (wait for text), or 'url_contains' (wait for URL change).",
			mcp.WithParam("selector"),
		), true
	}
	if conditionCount > 1 {
		return mcp.Fail(req, mcp.ErrInvalidParam,
			"wait_for conditions are mutually exclusive: use only one of selector, text, or url_contains",
			"Choose a single wait condition per call.",
		), true
	}
	if params.Absent && !hasSelector {
		return mcp.Fail(req, mcp.ErrMissingParam,
			"wait_for with absent requires a selector",
			"Provide 'selector' to specify which element to wait to disappear.",
			mcp.WithParam("selector"),
		), true
	}
	return mcp.JSONRPCResponse{}, false
}

func domActionContextOptions(action, selector string) []func(*mcp.StructuredError) {
	opts := []func(*mcp.StructuredError){mcp.WithAction(action)}
	if selector != "" {
		opts = append(opts, mcp.WithSelector(selector))
	}
	return opts
}

// ValidateDOMActionParams checks action-specific required parameters.
// Returns (response, true) if validation failed, or (zero, false) if valid.
func ValidateDOMActionParams(req mcp.JSONRPCRequest, action, text, value, name string) (mcp.JSONRPCResponse, bool) {
	rule, ok := domActionRequiredParams[action]
	if !ok {
		return mcp.JSONRPCResponse{}, false
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
		return mcp.Fail(req, mcp.ErrMissingParam, rule.Message, rule.Retry, mcp.WithParam(rule.Field)), true
	}
	return mcp.JSONRPCResponse{}, false
}

// handleHardwareClick dispatches a coordinate-based click via CDP Input.dispatchMouseEvent.
// This gives LLMs an explicit "I see coordinates in a screenshot, click there" path.
func (h *InteractActionHandler) HandleHardwareClick(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	params, err := parseHardwareClickParams(args)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}

	if params.X == nil {
		return mcp.Fail(req, mcp.ErrMissingParam, "Required parameter 'x' is missing", "Add the 'x' coordinate (pixels from left)", mcp.WithParam("x"))
	}
	if params.Y == nil {
		return mcp.Fail(req, mcp.ErrMissingParam, "Required parameter 'y' is missing", "Add the 'y' coordinate (pixels from top)", mcp.WithParam("y"))
	}

	return h.HandleCDPClick(req, args, "hardware_click", *params.X, *params.Y, params.TabID)
}

// handleCDPClick creates a cdp_action query for a hardware-level click at coordinates.
func (h *InteractActionHandler) HandleCDPClick(req mcp.JSONRPCRequest, args json.RawMessage, action string, x, y float64, tabID int) mcp.JSONRPCResponse {
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
			[]func(*mcp.StructuredError){mcp.WithAction(action)},
			h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking,
		).
		recordAction(action, "", map[string]any{"x": x, "y": y, "method": "cdp"}).
		queuedMessage(action+" queued").
		execute(req, args)
}

func (h *InteractActionHandler) HandleListInteractive(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		TabID       int  `json:"tab_id,omitempty"`
		VisibleOnly bool `json:"visible_only,omitempty"`
		Limit       int  `json:"limit,omitempty"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}

	args = normalizeDOMActionArgs(args, "list_interactive")
	resp, correlationID := h.newCommand("list_interactive").
		correlationPrefix("dom_list").
		reason("list_interactive").
		queryType("dom_action").
		queryParams(args).
		tabID(params.TabID).
		guards(h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking).
		recordAction("dom_list_interactive", "", nil).
		queuedMessage("list_interactive queued").
		executeWithCorrelation(req, args)

	indexGeneration := h.buildElementIndexFromResponse(req.ClientID, params.TabID, correlationID, resp)
	if indexGeneration != "" {
		resp = annotateListInteractiveIndexMetadata(resp, params.TabID, indexGeneration)
	}
	if params.Limit > 0 {
		resp = truncateListInteractiveResponse(resp, params.Limit)
	}
	return resp
}

func (h *InteractActionHandler) buildElementIndexFromResponse(clientID string, tabID int, generation string, resp mcp.JSONRPCResponse) string {
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || result.IsError {
		return ""
	}

	for _, block := range result.Content {
		idx := strings.Index(block.Text, "{")
		if idx < 0 {
			continue
		}
		var data map[string]any
		if json.Unmarshal([]byte(block.Text[idx:]), &data) != nil {
			continue
		}
		elements := extractElementList(data)
		if elements == nil {
			continue
		}

		indexMap := make(map[int]string, len(elements))
		for _, elem := range elements {
			elemMap, ok := elem.(map[string]any)
			if !ok {
				continue
			}
			indexVal, _ := elemMap["index"].(float64)
			selector, _ := elemMap["selector"].(string)
			if selector != "" {
				indexMap[int(indexVal)] = selector
			}
		}
		if h.elementIndexRegistry == nil {
			h.elementIndexRegistry = elemindex.New()
		}
		return h.elementIndexRegistry.Store(clientID, tabID, generation, indexMap)
	}
	return ""
}

func annotateListInteractiveIndexMetadata(resp mcp.JSONRPCResponse, tabID int, generation string) mcp.JSONRPCResponse {
	if generation == "" {
		return resp
	}
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || result.IsError {
		return resp
	}
	for i, block := range result.Content {
		idx := strings.Index(block.Text, "{")
		if idx < 0 {
			continue
		}
		var data map[string]any
		if json.Unmarshal([]byte(block.Text[idx:]), &data) != nil {
			continue
		}
		data["index_generation"] = generation
		data["index_scope_tab_id"] = tabID
		newJSON, err := json.Marshal(data)
		if err != nil {
			continue
		}
		result.Content[i].Text = block.Text[:idx] + string(newJSON)
		newResult, _ := json.Marshal(result)
		resp.Result = newResult
		return resp
	}
	return resp
}

func truncateListInteractiveResponse(resp mcp.JSONRPCResponse, limit int) mcp.JSONRPCResponse {
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || result.IsError {
		return resp
	}

	for i, block := range result.Content {
		idx := strings.Index(block.Text, "{")
		if idx < 0 {
			continue
		}
		var data map[string]any
		if json.Unmarshal([]byte(block.Text[idx:]), &data) != nil {
			continue
		}
		elements := extractElementList(data)
		if elements == nil || len(elements) <= limit {
			continue
		}

		total := len(elements)
		setNestedElements(data, elements[:limit])
		data["total"] = total
		data["truncated"] = true
		newJSON, err := json.Marshal(data)
		if err != nil {
			continue
		}
		result.Content[i].Text = block.Text[:idx] + string(newJSON)
		newResult, _ := json.Marshal(result)
		resp.Result = newResult
		return resp
	}
	return resp
}

func setNestedElements(data map[string]any, elements []any) {
	if _, ok := data["elements"]; ok {
		data["elements"] = elements
		return
	}
	if result, ok := data["result"].(map[string]any); ok {
		if _, ok := result["elements"]; ok {
			result["elements"] = elements
			return
		}
		if nested, ok := result["result"].(map[string]any); ok {
			if _, ok := nested["elements"]; ok {
				nested["elements"] = elements
			}
		}
	}
}

var extractElementList = act.ExtractElementList

func (h *InteractActionHandler) resolveIndexToSelector(clientID string, tabID int, index int, generation string) (string, bool, bool, string) {
	if h.elementIndexRegistry == nil {
		return "", false, false, ""
	}
	return h.elementIndexRegistry.Resolve(clientID, tabID, index, generation)
}
