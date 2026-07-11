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

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/interacthandler"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

// domPrimitiveActions delegates to the interact package.
var domPrimitiveActions = act.DOMPrimitiveActions

// randIntn returns a random int in [0, n). Uses math/rand/v2 which auto-seeds.
func randIntn(n int) int {
	if n <= 0 {
		return 0
	}
	// #nosec G404 -- non-cryptographic helper for jitter/selection in browser automation; not security-sensitive.
	return rand.IntN(n)
}

// interactAliasParams defines the deprecated alias parameters for the interact tool.
var interactAliasParams = []modeAlias{
	{JSONField: "action", DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
}

// interactRegistry is the tool registry for interact dispatch.
// PreDispatch handles evidence mode validation and async→background alias rewriting.
// PostDispatch handles composable side effects (subtitle, auto_dismiss, wait_for_stable,
// action_diff, include_screenshot, include_interactive).
var interactRegistry = toolRegistry{
	Handlers:  nil, // populated lazily per-call in toolInteract
	AliasDefs: interactAliasParams,
	Resolution: modeResolution{
		ToolName:   "interact",
		ValidModes: "", // populated lazily per-call in toolInteract
	},
	PreDispatch: func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage, what string) (json.RawMessage, *JSONRPCResponse) {
		// Apply jitter before dispatch (moved here from handler wrapping to avoid concurrent map writes).
		h.interactAction().ApplyJitter(what)

		// Validate evidence mode.
		if _, err := interacthandler.ParseEvidenceMode(args); err != nil {
			resp := fail(req, ErrInvalidParam,
				"Invalid 'evidence' value",
				"Use evidence='off' (default), 'on_mutation', or 'always'",
				withParam("evidence"))
			return args, &resp
		}
		// Quiet alias: async → background.
		args = mergeAsyncAlias(args)
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
var cachedInteractHandlers map[string]ModeHandler

// getInteractHandlers returns the cached unified handler map for interact actions.
// Merges both named handlers and DOM primitive actions into a single map[string]ModeHandler.
// The map is built once and cached for the process lifetime.
func getInteractHandlers() map[string]ModeHandler {
	interactHandlersOnce.Do(func() {
		cachedInteractHandlers = buildInteractHandlers()
	})
	return cachedInteractHandlers
}

// buildInteractHandlers constructs the full interact handler map.
func buildInteractHandlers() map[string]ModeHandler {
	handlers := map[string]ModeHandler{
		"highlight": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleHighlightImpl(req, args)
		},
		"save_state": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.stateInteract().HandleStateSave(req, args)
		},
		"state_save": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.stateInteract().HandleStateSave(req, args)
		},
		"load_state": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.stateInteract().HandleStateLoad(req, args)
		},
		"state_load": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.stateInteract().HandleStateLoad(req, args)
		},
		"list_states": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.stateInteract().HandleStateList(req, args)
		},
		"state_list": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.stateInteract().HandleStateList(req, args)
		},
		"delete_state": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.stateInteract().HandleStateDelete(req, args)
		},
		"state_delete": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.stateInteract().HandleStateDelete(req, args)
		},
		"set_storage": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleSetStorage(req, args)
		},
		"delete_storage": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleDeleteStorage(req, args)
		},
		"clear_storage": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleClearStorage(req, args)
		},
		"set_cookie": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleSetCookie(req, args)
		},
		"delete_cookie": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleDeleteCookie(req, args)
		},
		"execute_js": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleExecuteJSImpl(req, args)
		},
		"navigate": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleBrowserActionNavigateImpl(req, args)
		},
		"refresh": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleBrowserActionRefreshImpl(req, args)
		},
		"back": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleBrowserActionBackImpl(req, args)
		},
		"forward": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleBrowserActionForwardImpl(req, args)
		},
		"new_tab": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleBrowserActionNewTabImpl(req, args)
		},
		"switch_tab": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleBrowserActionSwitchTabImpl(req, args)
		},
		"close_tab": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleBrowserActionCloseTabImpl(req, args)
		},
		"screenshot": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleScreenshotAliasImpl(req, args)
		},
		"subtitle": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleSubtitleImpl(req, args)
		},
		"list_interactive": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleListInteractive(req, args)
		},
		"screen_recording_start": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.recordingInteractHandler.handleRecordStart(req, args)
		},
		"record_start": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.recordingInteractHandler.handleRecordStart(req, args)
		},
		"screen_recording_stop": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.recordingInteractHandler.handleRecordStop(req, args)
		},
		"record_stop": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.recordingInteractHandler.handleRecordStop(req, args)
		},
		"upload": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.uploadInteractHandler.HandleUpload(req, args)
		},
		"draw_mode_start": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleDrawModeStart(req, args)
		},
		"hardware_click": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleHardwareClick(req, args)
		},
		"activate_tab": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleActivateTabImpl(req, args)
		},
		"get_readable": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleGetReadable(req, args)
		},
		"get_markdown": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleGetMarkdown(req, args)
		},
		"navigate_and_wait_for": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleNavigateAndWaitFor(req, args)
		},
		"navigate_and_document": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleNavigateAndDocument(req, args)
		},
		"fill_form_and_submit": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleFillFormAndSubmit(req, args)
		},
		"fill_form": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleFillForm(req, args)
		},
		"run_a11y_and_export_sarif": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleRunA11yAndExportSARIF(req, args)
		},
		"explore_page": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleExplorePage(req, args)
		},
		"wait_for_stable": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleWaitForStable(req, args)
		},
		"auto_dismiss_overlays": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleAutoDismissOverlays(req, args)
		},
		"batch": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleBatch(req, args)
		},
		"clipboard_read": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleClipboardRead(req, args)
		},
		"clipboard_write": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleClipboardWrite(req, args)
		},
	}

	// Merge DOM primitive actions into the handler map.
	for action := range domPrimitiveActions {
		if _, exists := handlers[action]; exists {
			continue // named handler takes precedence (e.g. wait_for_stable, auto_dismiss_overlays)
		}
		action := action // capture for closure
		handlers[action] = func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
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
