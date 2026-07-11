// Purpose: Implements browser-side state capture and restore command queueing.
// Why: Isolates extension command orchestration from MCP request/response handlers.
// Docs: docs/features/feature/state-time-travel/index.md

package interacthandler

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

const (
	// stateCaptureTimeout is the time to wait for the extension to execute
	// the state capture script and return form/scroll/storage data.
	stateCaptureTimeout = 5 * time.Second
)

// stateCaptureResult — type alias delegated to internal/tools/interact package.
type stateCaptureResult = act.StateCaptureResult

// captureState attempts to capture form values, scroll position, and web storage from the browser.
// Always returns a stateCaptureResult with an explicit Status the caller can surface to the LLM.
func (h *StateInteractHandler) CaptureState(req JSONRPCRequest) stateCaptureResult {
	if !h.deps.Capture().IsPilotActionAllowed() {
		return stateCaptureResult{Status: act.StateCaptureStatusPilotDisabled}
	}
	if !h.deps.Capture().IsExtensionConnected() {
		return stateCaptureResult{Status: act.StateCaptureStatusExtensionDisconnected}
	}

	correlationID := newCorrelationID("state_capture")

	scriptArgs := buildQueryParams(map[string]any{
		"action": "execute_js",
		"script": act.StateCaptureScript,
		"world":  "main",
	})

	query := queries.PendingQuery{
		Type:          "execute",
		Params:        scriptArgs,
		CorrelationID: correlationID,
	}
	if _, blocked := h.deps.EnqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return stateCaptureResult{Status: act.StateCaptureStatusError}
	}

	cmd, found := h.deps.Capture().WaitForCommand(correlationID, stateCaptureTimeout)
	if !found || cmd.Status == "pending" {
		return stateCaptureResult{Status: act.StateCaptureStatusTimeout}
	}
	if cmd.Error != "" {
		return stateCaptureResult{Status: act.StateCaptureStatusError}
	}
	if cmd.Status != "complete" || len(cmd.Result) == 0 {
		return stateCaptureResult{Status: act.StateCaptureStatusError}
	}

	captureData, err := act.ParseCapturedStatePayload(cmd.Result)
	if err != nil {
		return stateCaptureResult{Status: act.StateCaptureStatusError}
	}

	return stateCaptureResult{Status: act.StateCaptureStatusCaptured, Data: captureData}
}

// queueStateRestore queues a JS execute command to restore form values, scroll position,
// localStorage, sessionStorage, and cookies. This is fire-and-forget.
func (h *StateInteractHandler) queueStateRestore(req JSONRPCRequest, formValues, scrollPos, localStorage, sessionStorage, cookies map[string]any) string {
	correlationID := newCorrelationID("state_restore")

	script := act.BuildStateRestoreScript(formValues, scrollPos, localStorage, sessionStorage, cookies)
	scriptArgs := buildQueryParams(map[string]any{
		"action": "execute_js",
		"script": script,
		"world":  "main",
	})

	query := queries.PendingQuery{
		Type:          "execute",
		Params:        scriptArgs,
		CorrelationID: correlationID,
	}
	if _, blocked := h.deps.EnqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return ""
	}

	return correlationID
}

// queueStateNavigation queues a navigation to the saved URL if pilot is enabled
// and the state contains a non-empty URL. Mutates stateData to add tracking fields.
func (h *StateInteractHandler) QueueStateNavigation(req JSONRPCRequest, stateData map[string]any) {
	savedURL, ok := stateData["url"].(string)
	if !ok || savedURL == "" || !h.deps.Capture().IsPilotActionAllowed() || !h.deps.Capture().IsExtensionConnected() {
		return
	}
	correlationID := newCorrelationID("nav")
	// Error impossible: map contains only string values
	navArgs := buildQueryParams(map[string]any{"action": "navigate", "url": savedURL})
	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        navArgs,
		CorrelationID: correlationID,
	}
	if _, blocked := h.deps.EnqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return
	}
	stateData["navigation_queued"] = true
	stateData["correlation_id"] = correlationID
}
