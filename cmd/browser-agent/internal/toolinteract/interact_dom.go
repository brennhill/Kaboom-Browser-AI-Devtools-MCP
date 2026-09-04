// interact_dom.go — DOM action execution and canonical command construction.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"encoding/json"
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract/elemindex"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
	"strings"
	"time"
)

// normalizeDOMActionArgs rewrites interact args so extension-facing dom_action
// payloads always carry canonical "action", while preserving user-facing "what".
func normalizeDOMActionArgs(args json.RawMessage, action string) json.RawMessage {
	var payload map[string]any
	if err := json.Unmarshal(args, &payload); err != nil || payload == nil {
		payload = map[string]any{}
	}
	payload["action"] = action
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

func (h *DOMActions) HandleDOMPrimitive(req mcp.JSONRPCRequest, args json.RawMessage, action string) mcp.JSONRPCResponse {
	params, err := ParseDOMPrimitiveParams(args)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}

	// If x/y coordinates provided on a click action, escalate to CDP for hardware-level click
	if action == "click" && params.X != nil && params.Y != nil {
		return h.HandleCDPClick(req, args, action, cdpClickTarget{X: *params.X, Y: *params.Y, Modifiers: params.Modifiers, TabID: params.TabID})
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

	if problem := act.ValidateGesture(action, params.gestureTarget()); problem != nil {
		return act.GestureParamFailure(req, action, problem)
	}

	if errResp, failed := ValidateDOMActionParams(req, action, params.Text, params.Value, params.Name); failed {
		return errResp
	}

	args = normalizeDOMActionArgs(args, action)

	return h.runtime.newCommand("dom_"+action).
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
	// Pointer gesture inputs (kaboom-05ue.5). The route is drag_path, not path: interact
	// already spends `path` on the cookie path string.
	DragPath  []gesturePoint `json:"drag_path,omitempty"`
	Modifiers []string       `json:"modifiers,omitempty"`
	DeltaX    *float64       `json:"delta_x,omitempty"`
	DeltaY    *float64       `json:"delta_y,omitempty"`
}

// gesturePoint is one viewport coordinate on a drag route.
type gesturePoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// gestureTarget reduces the parsed arguments to what the shared gesture rules need.
func (p DOMPrimitiveParams) gestureTarget() act.GestureTarget {
	return act.GestureTarget{
		Selector:   p.Selector,
		ElementID:  p.ElementID,
		HasIndex:   p.Index != nil,
		HasX:       p.X != nil,
		HasY:       p.Y != nil,
		PathPoints: len(p.DragPath),
		HasDeltaX:  p.DeltaX != nil,
		HasDeltaY:  p.DeltaY != nil,
	}
}

type hardwareClickParams struct {
	X         *float64 `json:"x"`
	Y         *float64 `json:"y"`
	Modifiers []string `json:"modifiers,omitempty"`
	TabID     int      `json:"tab_id,omitempty"`
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
	// The pointer gestures address a coordinate as readily as an element, so the blanket
	// selector rule would reject a correct call. act.ValidateGesture enforces their own rule.
	"drag":         {},
	"right_click":  {},
	"double_click": {},
	"triple_click": {},
	"hover_at":     {},
	"scroll_at":    {},
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
func (h *DOMActions) resolveDOMSelectorFromIndex(req mcp.JSONRPCRequest, args json.RawMessage, params *DOMPrimitiveParams) (json.RawMessage, mcp.JSONRPCResponse, bool) {
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
	rule, ok := act.DOMActionRequiredParams[action]
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
func (h *DOMActions) HandleHardwareClick(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
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

	return h.HandleCDPClick(req, args, "hardware_click", cdpClickTarget{X: *params.X, Y: *params.Y, Modifiers: params.Modifiers, TabID: params.TabID})
}

// cdpClickTarget is where a coordinate click lands and what is held while it lands.
type cdpClickTarget struct {
	X, Y      float64
	Modifiers []string
	TabID     int
}

// handleCDPClick creates a cdp_action query for a hardware-level click at coordinates.
// Modifiers travel with it: dropping them would turn a ctrl+click into an ordinary click that
// navigates in place instead of opening a background tab, and still report success.
func (h *DOMActions) HandleCDPClick(req mcp.JSONRPCRequest, args json.RawMessage, action string, target cdpClickTarget) mcp.JSONRPCResponse {
	params := map[string]any{"action": "click", "x": target.X, "y": target.Y}
	if len(target.Modifiers) > 0 {
		params["modifiers"] = target.Modifiers
	}
	return h.runtime.newCommand("cdp_click").
		correlationPrefix("cdp_click").
		reason(action).
		queryType("cdp_action").
		buildParams(params).
		tabID(target.TabID).
		guardsWithOpts(
			[]func(*mcp.StructuredError){mcp.WithAction(action)},
			h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking,
		).
		recordAction(action, "", map[string]any{"x": target.X, "y": target.Y, "method": "cdp"}).
		queuedMessage(action+" queued").
		execute(req, args)
}

func (h *DOMActions) HandleListInteractive(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		TabID       int  `json:"tab_id,omitempty"`
		VisibleOnly bool `json:"visible_only,omitempty"`
		Limit       int  `json:"limit,omitempty"`
		Verbose     bool `json:"verbose,omitempty"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}

	args = normalizeDOMActionArgs(args, "list_interactive")
	resp, correlationID := h.runtime.newCommand("list_interactive").
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
	// Last, after the element index and truncation have read the full payload.
	return toolresp.ProjectElementsInResponse(resp, params.Verbose)
}

func (h *DOMActions) buildElementIndexFromResponse(clientID string, tabID int, generation string, resp mcp.JSONRPCResponse) string {
	block, ok := decodeFirstToolResultJSONBlock(resp)
	if !ok {
		return ""
	}
	elements := act.ExtractElementList(block.data)
	if elements == nil {
		return ""
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

type toolResultJSONBlock struct {
	result       mcp.MCPToolResult
	contentIndex int
	prefix       string
	data         map[string]any
}

func decodeFirstToolResultJSONBlock(resp mcp.JSONRPCResponse) (toolResultJSONBlock, bool) {
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || result.IsError {
		return toolResultJSONBlock{}, false
	}
	for i, content := range result.Content {
		jsonStart := strings.Index(content.Text, "{")
		if jsonStart < 0 {
			continue
		}
		var data map[string]any
		if json.Unmarshal([]byte(content.Text[jsonStart:]), &data) == nil {
			return toolResultJSONBlock{
				result:       result,
				contentIndex: i,
				prefix:       content.Text[:jsonStart],
				data:         data,
			}, true
		}
	}
	return toolResultJSONBlock{}, false
}

func (b toolResultJSONBlock) replace(resp mcp.JSONRPCResponse) mcp.JSONRPCResponse {
	data, err := json.Marshal(b.data)
	if err != nil {
		return resp
	}
	b.result.Content[b.contentIndex].Text = b.prefix + string(data)
	resp.Result = mcp.SafeMarshal(b.result, string(resp.Result))
	return resp
}

func annotateListInteractiveIndexMetadata(resp mcp.JSONRPCResponse, tabID int, generation string) mcp.JSONRPCResponse {
	if generation == "" {
		return resp
	}
	block, ok := decodeFirstToolResultJSONBlock(resp)
	if !ok {
		return resp
	}
	block.data["index_generation"] = generation
	block.data["index_scope_tab_id"] = tabID
	return block.replace(resp)
}

func truncateListInteractiveResponse(resp mcp.JSONRPCResponse, limit int) mcp.JSONRPCResponse {
	block, ok := decodeFirstToolResultJSONBlock(resp)
	if !ok {
		return resp
	}
	elements := act.ExtractElementList(block.data)
	if elements == nil || len(elements) <= limit {
		return resp
	}

	setNestedElements(block.data, elements[:limit])
	block.data["total"] = len(elements)
	block.data["truncated"] = true
	return block.replace(resp)
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

func (h *DOMActions) resolveIndexToSelector(clientID string, tabID int, index int, generation string) (string, bool, bool, string) {
	if h.elementIndexRegistry == nil {
		return "", false, false, ""
	}
	return h.elementIndexRegistry.Resolve(clientID, tabID, index, generation)
}

// commandBuilder provides a fluent API for the common interact handler sequence:
//  1. Run guard checks (requirePilot, requireExtension, requireTabTracking, etc.)
//  2. Generate a correlation ID with a prefix
//  3. Arm evidence for the command
//  4. Build or set query params
//  5. Enqueue a pending query
//  6. Optionally record an AI action
//  7. Wait for the command result via MaybeWaitForCommand
//
// Usage:
//
//	return h.newCommand("highlight").
//	    correlationPrefix("highlight").
//	    reason("highlight").
//	    queryType("highlight").
//	    queryParams(args).
//	    tabID(params.TabID).
//	    guards(h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking).
//	    recordAction("highlight", "", map[string]any{"selector": params.Selector}).
//	    queuedMessage("Highlight queued").
//	    execute(req, args)
type commandBuilder struct {
	runtime *ActionRuntime

	// Identity
	name string // descriptive name (for debugging; not used in output)

	// Correlation
	corrPrefix string // prefix for newCorrelationID
	reasonStr  string // reason for armEvidenceForCommand

	// Query
	qType    string          // pending query type (e.g. "execute", "browser_action", "dom_action")
	qParams  json.RawMessage // serialized query params; nil = use waitArgs from execute()
	qTabID   int             // tab ID for the pending query
	qTimeout time.Duration   // enqueue timeout; zero = queries.AsyncCommandTimeout

	// Guards
	guardFns  []toolguard.Check
	guardOpts []func(*mcp.StructuredError) // optional opts passed to checkGuardsWithOpts

	// AI recording (optional)
	doRecord  bool
	recAction string
	recURL    string
	recExtra  map[string]any

	// CSP guard (optional)
	cspWorld string // world value for requireCSPClear; empty = skip

	// Pre-enqueue callback (optional). Called after correlation ID is generated
	// but before the query is enqueued. Used for side effects that need the
	// correlation ID (e.g. stashPerfSnapshot).
	preEnqueueFn func(correlationID string)

	// Post-enqueue callback (optional). Called after the query is successfully
	// enqueued but before MaybeWaitForCommand. Used for recording actions with
	// non-standard signatures (for example DOM primitive recording).
	postEnqueueFn func()

	// Response
	queuedMsg string // message for MaybeWaitForCommand when command is async
}

// newCommand creates a command builder bound to shared execution policy.
// The name is descriptive only (for debugging/logging).
func (h *ActionRuntime) newCommand(name string) *commandBuilder {
	return &commandBuilder{
		runtime: h,
		name:    name,
	}
}

// correlationPrefix sets the prefix for the generated correlation ID.
func (b *commandBuilder) correlationPrefix(prefix string) *commandBuilder {
	b.corrPrefix = prefix
	return b
}

// reason sets the reason string passed to armEvidenceForCommand.
func (b *commandBuilder) reason(r string) *commandBuilder {
	b.reasonStr = r
	return b
}

// queryType sets the PendingQuery.Type field.
func (b *commandBuilder) queryType(t string) *commandBuilder {
	b.qType = t
	return b
}

// queryParams sets pre-serialized query parameters.
func (b *commandBuilder) queryParams(p json.RawMessage) *commandBuilder {
	b.qParams = p
	return b
}

// buildParams constructs query parameters with the package's empty-object fallback policy.
func (b *commandBuilder) buildParams(m map[string]any) *commandBuilder {
	b.qParams = marshalQueryParams(m)
	return b
}

// tabID sets the tab ID for the pending query.
func (b *commandBuilder) tabID(id int) *commandBuilder {
	b.qTabID = id
	return b
}

// guards adds guard checks that run before the command is enqueued.
// Guards are run in order; the first blocking guard short-circuits.
func (b *commandBuilder) guards(fns ...toolguard.Check) *commandBuilder {
	b.guardFns = append(b.guardFns, fns...)
	return b
}

// guardsWithOpts adds guard checks with mcp.StructuredError options.
// This is used by handlers like handleDOMPrimitive that need to pass
// contextOpts (action, selector) through to guard error responses.
// Note: opts are accumulated, not replaced — multiple calls are safe.
func (b *commandBuilder) guardsWithOpts(opts []func(*mcp.StructuredError), fns ...toolguard.Check) *commandBuilder {
	b.guardOpts = append(b.guardOpts, opts...)
	b.guardFns = append(b.guardFns, fns...)
	return b
}

// preEnqueue registers a callback invoked after correlation ID generation but before enqueue.
// Useful for side effects like stashPerfSnapshot that need the correlation ID.
func (b *commandBuilder) preEnqueue(fn func(correlationID string)) *commandBuilder {
	b.preEnqueueFn = fn
	return b
}

// postEnqueue registers a callback invoked after successful enqueue but before MaybeWaitForCommand.
// Used for recording actions with non-standard signatures (for example DOM primitives).
func (b *commandBuilder) postEnqueue(fn func()) *commandBuilder {
	b.postEnqueueFn = fn
	return b
}

// cspGuard adds a CSP check for the given world after other guards.
// Only world="main" is blocked — "auto" and "isolated" bypass page CSP.
func (b *commandBuilder) cspGuard(world string) *commandBuilder {
	b.cspWorld = world
	return b
}

// recordAction configures AI action recording after the command is enqueued.
func (b *commandBuilder) recordAction(action, url string, extra map[string]any) *commandBuilder {
	b.doRecord = true
	b.recAction = action
	b.recURL = url
	b.recExtra = extra
	return b
}

// queuedMessage sets the message shown when the command is async (queued).
func (b *commandBuilder) queuedMessage(msg string) *commandBuilder {
	b.queuedMsg = msg
	return b
}

// execute runs the full command sequence: guards → correlate → arm → enqueue → record → wait.
// waitArgs is the original args passed to MaybeWaitForCommand for sync/background resolution.
func (b *commandBuilder) execute(req mcp.JSONRPCRequest, waitArgs json.RawMessage) mcp.JSONRPCResponse {
	resp, _ := b.executeWithCorrelation(req, waitArgs)
	return resp
}

// executeWithCorrelation is like execute but also returns the generated correlation ID.
// Useful for handlers that need the correlation ID for post-processing (e.g. element index).
// Returns empty string if guards blocked before correlation ID generation.
func (b *commandBuilder) executeWithCorrelation(req mcp.JSONRPCRequest, waitArgs json.RawMessage) (mcp.JSONRPCResponse, string) {
	// 0. Validate required fields
	if b.corrPrefix == "" {
		b.corrPrefix = b.name // fall back to builder name
	}
	if b.qType == "" {
		return mcp.Fail(req, mcp.ErrMissingParam, "commandBuilder: queryType is required", "Set queryType before calling execute"), ""
	}

	// 1. Run guards
	if len(b.guardOpts) > 0 {
		if resp, blocked := checkGuardsWithOpts(req, b.guardOpts, b.guardFns...); blocked {
			return resp, ""
		}
	} else if len(b.guardFns) > 0 {
		if resp, blocked := checkGuards(req, b.guardFns...); blocked {
			return resp, ""
		}
	}

	// 1b. Run CSP guard if configured
	if b.cspWorld != "" {
		if resp, blocked := b.runtime.deps.RequireCSPClear(req, b.cspWorld); blocked {
			return resp, ""
		}
	}

	// 2. Generate correlation ID and arm evidence
	correlationID := toolresp.NewCorrelationID(b.corrPrefix)
	b.runtime.ArmEvidenceForCommand(correlationID, b.reasonStr, waitArgs, req.ClientID)

	// 2b. Pre-enqueue callback (e.g. stash perf snapshot)
	if b.preEnqueueFn != nil {
		b.preEnqueueFn(correlationID)
	}

	// 3. Resolve query params
	params := b.qParams
	if params == nil {
		params = waitArgs
	}

	// 4. Resolve timeout
	timeout := b.qTimeout
	if timeout == 0 {
		timeout = queries.AsyncCommandTimeout
	}

	// 5. Enqueue pending query
	query := queries.PendingQuery{
		Type:          b.qType,
		Params:        params,
		TabID:         b.qTabID,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := b.runtime.deps.EnqueuePendingQuery(req, query, timeout); blocked {
		return enqueueResp, correlationID
	}

	// 6. Record AI action (optional)
	if b.doRecord {
		b.runtime.deps.RecordAIAction(b.recAction, b.recURL, b.recExtra)
	}

	// 6b. Post-enqueue callback (for example DOM primitive recording)
	if b.postEnqueueFn != nil {
		b.postEnqueueFn()
	}

	// 7. Wait for command
	return b.runtime.deps.MaybeWaitForCommand(req, correlationID, waitArgs, b.queuedMsg), correlationID
}
