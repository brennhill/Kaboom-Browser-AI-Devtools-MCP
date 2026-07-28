// Purpose: Provides top-level interact dispatch, action routing, and jitter behavior.
// Why: Keeps orchestration logic centralized while action implementations live in focused files.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
	"math/rand/v2"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolrouting"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
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

// interactRegistry is the tool registry for interact dispatch.
// PreDispatch handles evidence mode validation.
// PostDispatch handles composable side effects (subtitle, auto_dismiss, wait_for_stable,
// action_diff, include_screenshot, include_interactive).
var interactRegistry = toolrouting.Registry[*ToolHandler]{
	Handlers: nil, // populated lazily per-call in toolInteract
	Resolution: toolrouting.Resolution{
		ToolName:   "interact",
		ValidModes: "", // populated lazily per-call in toolInteract
	},
	PreDispatch: func(h *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage, what string) (json.RawMessage, *mcp.JSONRPCResponse) {
		// Apply jitter before dispatch (moved here from handler wrapping to avoid concurrent map writes).
		h.interactActionHandler.ApplyJitter(what)

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
			return th.interactActionHandler.HandleHighlightImpl(req, args)
		},
		"save_state": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.stateInteractHandler.HandleStateSave(req, args)
		},
		"load_state": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.stateInteractHandler.HandleStateLoad(req, args)
		},
		"list_states": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.stateInteractHandler.HandleStateList(req, args)
		},
		"delete_state": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.stateInteractHandler.HandleStateDelete(req, args)
		},
		"set_storage": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleSetStorage(req, args)
		},
		"delete_storage": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleDeleteStorage(req, args)
		},
		"clear_storage": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleClearStorage(req, args)
		},
		"set_cookie": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleSetCookie(req, args)
		},
		"delete_cookie": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleDeleteCookie(req, args)
		},
		"execute_js": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleExecuteJSImpl(req, args)
		},
		"navigate": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleBrowserActionNavigateImpl(req, args)
		},
		"refresh": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleBrowserActionRefreshImpl(req, args)
		},
		"back": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleBrowserActionBackImpl(req, args)
		},
		"forward": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleBrowserActionForwardImpl(req, args)
		},
		"new_tab": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleBrowserActionNewTabImpl(req, args)
		},
		"switch_tab": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleBrowserActionSwitchTabImpl(req, args)
		},
		"close_tab": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleBrowserActionCloseTabImpl(req, args)
		},
		"subtitle": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleSubtitleImpl(req, args)
		},
		"list_interactive": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleListInteractive(req, args)
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
			return th.interactActionHandler.HandleDrawModeStart(req, args)
		},
		"hardware_click": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleHardwareClick(req, args)
		},
		"activate_tab": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleActivateTabImpl(req, args)
		},
		"get_readable": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleGetReadable(req, args)
		},
		"get_markdown": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleGetMarkdown(req, args)
		},
		"navigate_and_wait_for": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleNavigateAndWaitFor(req, args)
		},
		"navigate_and_document": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleNavigateAndDocument(req, args)
		},
		"fill_form_and_submit": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleFillFormAndSubmit(req, args)
		},
		"fill_form": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleFillForm(req, args)
		},
		"run_a11y_and_export_sarif": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleRunA11yAndExportSARIF(req, args)
		},
		"explore_page": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleExplorePage(req, args)
		},
		"wait_for_stable": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleWaitForStable(req, args)
		},
		"auto_dismiss_overlays": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleAutoDismissOverlays(req, args)
		},
		"batch": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleBatch(req, args)
		},
		"clipboard_read": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleClipboardRead(req, args)
		},
		"clipboard_write": func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleClipboardWrite(req, args)
		},
	}

	// Merge DOM primitive actions into the handler map.
	for action := range act.DOMPrimitiveActions {
		if _, exists := handlers[action]; exists {
			continue // named handler takes precedence (e.g. wait_for_stable, auto_dismiss_overlays)
		}
		action := action // capture for closure
		handlers[action] = func(th *ToolHandler, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return th.interactActionHandler.HandleDOMPrimitive(req, args, action)
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
	what := resolveWhatForComposable(args)
	response := toolrouting.Dispatch(h, req, args, registry)

	if composable.Subtitle != nil && what != "subtitle" && response.Error == nil {
		h.interactActionHandler.QueueComposableSubtitle(req, *composable.Subtitle)
	}
	hasSideEffects := false
	if composable.AutoDismiss && what == "navigate" && !act.IsErrorResponse(response) {
		h.interactActionHandler.QueueComposableAutoDismiss(req)
		hasSideEffects = true
	}
	if composable.WaitForStable && (what == "navigate" || what == "click") && !act.IsErrorResponse(response) {
		h.interactActionHandler.QueueComposableWaitForStable(req, composable.StabilityMs)
		hasSideEffects = true
	}
	if composable.ActionDiff && !act.IsErrorResponse(response) {
		h.interactActionHandler.QueueComposableActionDiff(req)
		hasSideEffects = true
	}
	if hasSideEffects && composable.IncludeScreenshot {
		time.Sleep(composableSideEffectDelay)
	}
	if composable.IncludeScreenshot && !act.IsErrorResponse(response) {
		response = h.interactActionHandler.AppendScreenshotToResponse(response, req)
	}
	if composable.IncludeInteractive && !act.IsErrorResponse(response) {
		response = h.interactActionHandler.AppendInteractiveToResponse(response, req)
	}
	return response
}

func resolveWhatForComposable(args json.RawMessage) string {
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
	return ""
}
