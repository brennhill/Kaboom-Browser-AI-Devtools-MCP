// tools_interact_dispatch.go — Wires canonical interact action owners into the dispatcher.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/interactdispatch"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

func (h *ToolHandler) toolInteract(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return h.interactDispatcher.Handle(req, args)
}

func buildInteractDispatcher(h *ToolHandler) *interactdispatch.Handler {
	return interactdispatch.New(interactdispatch.Deps{
		Actions:            buildInteractActions(h),
		ApplyJitter:        h.interactRuntime.ApplyJitter,
		QueueSubtitle:      h.pageActions.QueueComposableSubtitle,
		QueueAutoDismiss:   h.pageActions.QueueComposableAutoDismiss,
		QueueWaitForStable: h.pageActions.QueueComposableWaitForStable,
		QueueActionDiff:    h.pageActions.QueueComposableActionDiff,
		AppendScreenshot:   h.pageActions.AppendScreenshotToResponse,
		AppendInteractive:  h.pageActions.AppendInteractiveToResponse,
	})
}

func buildInteractActions(h *ToolHandler) map[string]interactdispatch.Action {
	actions := map[string]interactdispatch.Action{
		"highlight":                 browserAction(h, "highlight"),
		"execute_js":                browserAction(h, "execute_js"),
		"navigate":                  browserAction(h, "navigate"),
		"refresh":                   browserAction(h, "refresh"),
		"back":                      browserAction(h, "back"),
		"forward":                   browserAction(h, "forward"),
		"new_tab":                   browserAction(h, "new_tab"),
		"switch_tab":                browserAction(h, "switch_tab"),
		"close_tab":                 browserAction(h, "close_tab"),
		"subtitle":                  browserAction(h, "subtitle"),
		"activate_tab":              browserAction(h, "activate_tab"),
		"zoom_region":               browserAction(h, "zoom_region"),
		"pin_environment":           browserAction(h, "pin_environment"),
		"unpin_environment":         browserAction(h, "unpin_environment"),
		"save_state":                h.stateInteractHandler.HandleStateSave,
		"load_state":                h.stateInteractHandler.HandleStateLoad,
		"list_states":               h.stateInteractHandler.HandleStateList,
		"delete_state":              h.stateInteractHandler.HandleStateDelete,
		"set_storage":               h.storageActions.HandleSetStorage,
		"delete_storage":            h.storageActions.HandleDeleteStorage,
		"clear_storage":             h.storageActions.HandleClearStorage,
		"set_cookie":                h.storageActions.HandleSetCookie,
		"delete_cookie":             h.storageActions.HandleDeleteCookie,
		"list_interactive":          h.domActions.HandleListInteractive,
		"hardware_click":            h.domActions.HandleHardwareClick,
		"draw_mode_start":           h.pageActions.HandleDrawModeStart,
		"find":                      h.pageActions.HandleFind,
		"get_readable":              h.pageActions.HandleGetReadable,
		"get_markdown":              h.pageActions.HandleGetMarkdown,
		"explore_page":              h.pageActions.HandleExplorePage,
		"wait_for_stable":           h.pageActions.HandleWaitForStable,
		"auto_dismiss_overlays":     h.pageActions.HandleAutoDismissOverlays,
		"clipboard_read":            h.pageActions.HandleClipboardRead,
		"clipboard_write":           h.pageActions.HandleClipboardWrite,
		"navigate_and_wait_for":     h.workflowActions.HandleNavigateAndWaitFor,
		"navigate_and_document":     h.workflowActions.HandleNavigateAndDocument,
		"fill_form_and_submit":      h.workflowActions.HandleFillFormAndSubmit,
		"fill_form":                 h.workflowActions.HandleFillForm,
		"run_a11y_and_export_sarif": h.workflowActions.HandleRunA11yAndExportSARIF,
		"screen_recording_start":    h.recordingInteractHandler.HandleRecordStart,
		"screen_recording_stop":     h.recordingInteractHandler.HandleRecordStop,
		"upload":                    h.uploadInteractHandler.HandleUpload,
		"batch":                     h.batchActions.Handle,
	}
	for action := range act.DOMPrimitiveActions {
		if _, exists := actions[action]; exists {
			continue
		}
		action := action
		actions[action] = func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return h.domActions.HandleDOMPrimitive(req, args, action)
		}
	}
	return actions
}

func browserAction(h *ToolHandler, action string) interactdispatch.Action {
	return func(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
		return h.browserActions.Handle(action, req, args)
	}
}
