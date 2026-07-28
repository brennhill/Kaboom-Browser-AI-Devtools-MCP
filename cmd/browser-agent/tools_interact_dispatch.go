// Purpose: Provides top-level interact dispatch, action routing, and jitter behavior.
// Why: Keeps orchestration logic centralized while action implementations live in focused files.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"math/rand/v2"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolrouting"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

// randIntn returns a random int in [0, n). Uses math/rand/v2 which auto-seeds.
func randIntn(n int) int {
	if n <= 0 {
		return 0
	}
	// #nosec G404 -- non-cryptographic helper for jitter/selection in browser automation; not security-sensitive.
	return rand.IntN(n)
}

// interactAliasParams defines the deprecated alias parameters for the interact tool.
var interactAliasParams = []toolrouting.Alias{
	{JSONField: "action", DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
}

// interactRegistry is the tool registry for interact dispatch.
// PreDispatch handles evidence mode validation.
// PostDispatch handles composable side effects (subtitle, auto_dismiss, wait_for_stable,
// action_diff, include_screenshot, include_interactive).
var interactRegistry = toolrouting.Registry[*ToolHandler]{
	Handlers:  nil, // populated lazily per-call in toolInteract
	AliasDefs: interactAliasParams,
	Resolution: toolrouting.Resolution{
		ToolName:   "interact",
		ValidModes: "", // populated lazily per-call in toolInteract
	},
	PreDispatch: func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage, what string) (json.RawMessage, *mcp.JSONRPCResponse) {
		// Apply jitter before dispatch (moved here from handler wrapping to avoid concurrent map writes).
		h.interactAction().ApplyJitter(what)

		// Validate evidence mode.
		if _, err := toolinteract.ParseEvidenceMode(args); err != nil {
			resp := mcp.Fail(req, mcp.ErrInvalidParam,
				"Invalid 'evidence' value",
				"Use evidence='off' (default), 'on_mutation', or 'always'",
				mcp.WithParam("evidence"))
			return args, &resp
		}
		return args, nil
	},
	// PostDispatch is nil: composable side effects (subtitle, auto_dismiss, wait_for_stable,
	// action_diff, include_screenshot, include_interactive) are handled in toolInteract
	// after dispatchTool returns, since PostDispatch doesn't receive args.
}

// interactHandlersOnce ensures the handler map is built exactly once, even under concurrency.
var interactHandlersOnce sync.Once

// cachedInteractHandlers is the lazily-initialized handler map for interact actions.
// Populated once via sync.Once on first call to getInteractHandlers() and reused thereafter.
var cachedInteractHandlers map[string]toolrouting.Handler[*ToolHandler]

// getInteractHandlers returns the cached unified handler map for interact actions.
// Merges both named handlers and DOM primitive actions into a single map[string]toolrouting.Handler[*ToolHandler].
// The map is built once and cached for the process lifetime.
func getInteractHandlers() map[string]toolrouting.Handler[*ToolHandler] {
	interactHandlersOnce.Do(func() {
		cachedInteractHandlers = buildInteractHandlers()
	})
	return cachedInteractHandlers
}

// buildInteractHandlers constructs the full interact handler map.
func buildInteractHandlers() map[string]toolrouting.Handler[*ToolHandler] {
	handlers := map[string]toolrouting.Handler[*ToolHandler]{
		"highlight": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleHighlightImpl(req, args)
		},
		"save_state": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.stateInteract().HandleStateSave(req, args)
		},
		"load_state": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.stateInteract().HandleStateLoad(req, args)
		},
		"list_states": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.stateInteract().HandleStateList(req, args)
		},
		"delete_state": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.stateInteract().HandleStateDelete(req, args)
		},
		"set_storage": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleSetStorage(req, args)
		},
		"delete_storage": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleDeleteStorage(req, args)
		},
		"clear_storage": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleClearStorage(req, args)
		},
		"set_cookie": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleSetCookie(req, args)
		},
		"delete_cookie": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleDeleteCookie(req, args)
		},
		"execute_js": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleExecuteJSImpl(req, args)
		},
		"navigate": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleBrowserActionNavigateImpl(req, args)
		},
		"refresh": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleBrowserActionRefreshImpl(req, args)
		},
		"back": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleBrowserActionBackImpl(req, args)
		},
		"forward": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleBrowserActionForwardImpl(req, args)
		},
		"new_tab": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleBrowserActionNewTabImpl(req, args)
		},
		"switch_tab": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleBrowserActionSwitchTabImpl(req, args)
		},
		"close_tab": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleBrowserActionCloseTabImpl(req, args)
		},
		"subtitle": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleSubtitleImpl(req, args)
		},
		"list_interactive": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleListInteractive(req, args)
		},
		"screen_recording_start": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.recordingInteractHandler.HandleRecordStart(req, args)
		},
		"screen_recording_stop": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.recordingInteractHandler.HandleRecordStop(req, args)
		},
		"upload": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.uploadInteractHandler.HandleUpload(req, args)
		},
		"draw_mode_start": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleDrawModeStart(req, args)
		},
		"hardware_click": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleHardwareClick(req, args)
		},
		"activate_tab": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleActivateTabImpl(req, args)
		},
		"get_readable": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleGetReadable(req, args)
		},
		"get_markdown": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleGetMarkdown(req, args)
		},
		"navigate_and_wait_for": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleNavigateAndWaitFor(req, args)
		},
		"navigate_and_document": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleNavigateAndDocument(req, args)
		},
		"fill_form_and_submit": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleFillFormAndSubmit(req, args)
		},
		"fill_form": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleFillForm(req, args)
		},
		"run_a11y_and_export_sarif": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleRunA11yAndExportSARIF(req, args)
		},
		"explore_page": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleExplorePage(req, args)
		},
		"wait_for_stable": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleWaitForStable(req, args)
		},
		"auto_dismiss_overlays": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleAutoDismissOverlays(req, args)
		},
		"batch": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleBatch(req, args)
		},
		"clipboard_read": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleClipboardRead(req, args)
		},
		"clipboard_write": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleClipboardWrite(req, args)
		},
	}

	// Merge DOM primitive actions into the handler map.
	for action := range act.DOMPrimitiveActions {
		if _, exists := handlers[action]; exists {
			continue // named handler takes precedence (e.g. wait_for_stable, auto_dismiss_overlays)
		}
		action := action // capture for closure
		handlers[action] = func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactAction().HandleDOMPrimitive(req, args, action)
		}
	}

	return handlers
}

// getValidInteractActions returns a sorted, comma-separated list of valid interact actions.
func getValidInteractActions() string {
	handlers := getInteractHandlers()
	sorted := make([]string, 0, len(handlers))
	for action := range handlers {
		sorted = append(sorted, action)
	}
	sort.Strings(sorted)
	return strings.Join(sorted, ", ")
}

const composableSideEffectDelay = 300 * time.Millisecond

func (h *ToolHandler) toolInteract(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var composable struct {
		Subtitle           *string `json:"subtitle"`
		IncludeScreenshot  bool    `json:"include_screenshot"`
		IncludeInteractive bool    `json:"include_interactive"`
		AutoDismiss        bool    `json:"auto_dismiss"`
		WaitForStable      bool    `json:"wait_for_stable"`
		StabilityMs        int     `json:"stability_ms,omitempty"`
		ActionDiff         bool    `json:"action_diff"`
	}
	mcp.LenientUnmarshal(args, &composable)

	registry := interactRegistry
	registry.Handlers = getInteractHandlers()
	registry.Resolution.ValidModes = getValidInteractActions()
	what := resolveWhatForComposable(args, interactAliasParams)
	response := toolrouting.Dispatch(h, req, args, registry)

	if composable.Subtitle != nil && what != "subtitle" && response.Error == nil {
		h.interactAction().QueueComposableSubtitle(req, *composable.Subtitle)
	}
	hasSideEffects := false
	if composable.AutoDismiss && what == "navigate" && !act.IsErrorResponse(response) {
		h.interactAction().QueueComposableAutoDismiss(req)
		hasSideEffects = true
	}
	if composable.WaitForStable && (what == "navigate" || what == "click") && !act.IsErrorResponse(response) {
		h.interactAction().QueueComposableWaitForStable(req, composable.StabilityMs)
		hasSideEffects = true
	}
	if composable.ActionDiff && !act.IsErrorResponse(response) {
		h.interactAction().QueueComposableActionDiff(req)
		hasSideEffects = true
	}
	if hasSideEffects && composable.IncludeScreenshot {
		time.Sleep(composableSideEffectDelay)
	}
	if composable.IncludeScreenshot && !act.IsErrorResponse(response) {
		response = h.interactAction().AppendScreenshotToResponse(response, req)
	}
	if composable.IncludeInteractive && !act.IsErrorResponse(response) {
		response = h.interactAction().AppendInteractiveToResponse(response, req)
	}
	return response
}

func resolveWhatForComposable(args json.RawMessage, aliases []toolrouting.Alias) string {
	if len(args) == 0 {
		return ""
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(args, &raw) != nil {
		return ""
	}
	if value, ok := raw["what"]; ok {
		var what string
		if json.Unmarshal(value, &what) == nil && what != "" {
			return what
		}
	}
	for _, alias := range aliases {
		if value, ok := raw[alias.JSONField]; ok {
			var what string
			if json.Unmarshal(value, &what) == nil && what != "" {
				return what
			}
		}
	}
	return ""
}

func (h *ToolHandler) recordAIAction(actionType, url string, details map[string]any) {
	action := types.EnhancedAction{
		Type: actionType, Timestamp: time.Now().UnixMilli(), URL: url, Source: "ai",
	}
	if len(details) > 0 {
		action.Selectors = details
	}
	h.capture.AddEnhancedActions([]types.EnhancedAction{action})
}

func (h *ToolHandler) recordAIEnhancedAction(action types.EnhancedAction) {
	action.Timestamp = time.Now().UnixMilli()
	action.Source = "ai"
	h.capture.AddEnhancedActions([]types.EnhancedAction{action})
}

func (h *ToolHandler) recordDOMPrimitiveAction(action, selector, text, value string) {
	reproType, ok := act.DOMActionToReproType[action]
	if !ok {
		h.recordAIAction("dom_"+action, "", map[string]any{"selector": selector})
		return
	}
	enhanced := types.EnhancedAction{
		Type: reproType, Selectors: act.ParseSelectorForReproduction(selector),
	}
	switch action {
	case "type":
		enhanced.Value = text
	case "key_press":
		enhanced.Key = text
	case "select":
		enhanced.SelectedValue = value
	}
	h.recordAIEnhancedAction(enhanced)
}

func (h *ToolHandler) enrichNavigateResponse(resp mcp.JSONRPCResponse, req mcp.JSONRPCRequest, tabID int) mcp.JSONRPCResponse {
	var result mcp.MCPToolResult
	if json.Unmarshal(resp.Result, &result) != nil || result.IsError {
		return resp
	}
	_, _, tabURL := h.capture.GetTrackingStatus()
	tabTitle := h.capture.GetTrackedTabTitle()
	vitals := h.capture.GetPerformanceSnapshots()
	correlationID := toolresp.NewCorrelationID("nav_content")
	params := mcp.SafeMarshal(map[string]any{"timeout_ms": 4000}, "{}")
	query := queries.PendingQuery{
		Type: "page_summary", Params: params, TabID: tabID, CorrelationID: correlationID,
	}
	if enqueueResponse, blocked := h.EnqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResponse
	}
	var textContent string
	command, found := h.capture.WaitForCommand(correlationID, toolinteract.NavigatePageSummaryWait)
	if found && command.Status != "pending" && command.Result != nil {
		var summary map[string]any
		if json.Unmarshal(command.Result, &summary) == nil {
			textContent, _ = summary["main_content_preview"].(string)
		}
	}
	if len(result.Content) > 0 {
		enrichment := map[string]any{"url": tabURL, "title": tabTitle, "text_content": textContent}
		if len(vitals) > 0 {
			enrichment["vitals"] = vitals[len(vitals)-1]
		}
		enrichmentJSON, _ := json.Marshal(enrichment)
		result.Content = append(result.Content, mcp.MCPContentBlock{
			Type: "text", Text: "Page content:\n" + string(enrichmentJSON),
		})
	}
	resultJSON, _ := json.Marshal(result)
	resp.Result = resultJSON
	return resp
}
