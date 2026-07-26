// interact_state.go — StateInteractHandler: save/load/list/delete of page-state
// snapshots, plus the browser-side capture and restore commands they queue.
// Docs: docs/features/feature/state-time-travel/index.md

package toolinteract

import (
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

// StateInteractHandler handles state save/load/list/delete operations.
type StateInteractHandler struct {
	deps *Deps

	// Concrete session store injected at construction.
	sessionStoreImpl *persistence.SessionStore
}

// NewStateInteractHandler creates a new StateInteractHandler with the given dependencies.
func NewStateInteractHandler(deps *Deps, store *persistence.SessionStore) *StateInteractHandler {
	return &StateInteractHandler{
		deps:             deps,
		sessionStoreImpl: store,
	}
}

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

func (h *StateInteractHandler) HandleStateSave(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		SnapshotName string `json:"snapshot_name"`
		Name         string `json:"name"` // backward-compatible alias
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	snapshotName := resolveStateSnapshotName(params.SnapshotName, params.Name)
	if resp, blocked := requireString(req, snapshotName, "snapshot_name", "Add the 'snapshot_name' parameter (legacy alias: 'name')"); blocked {
		return resp
	}

	if resp, blocked := h.deps.RequireSessionStore(req); blocked {
		return resp
	}

	_, tabID, tabURL := h.deps.Capture().GetTrackingStatus()
	tabTitle := h.deps.Capture().GetTrackedTabTitle()

	stateData := map[string]any{
		"url":      tabURL,
		"title":    tabTitle,
		"tab_id":   tabID,
		"saved_at": time.Now().Format(time.RFC3339),
	}

	// State capture — always produces a status for the response
	capture := h.CaptureState(req)
	if capture.Status == act.StateCaptureStatusCaptured && capture.Data != nil {
		for _, field := range act.StateDataFields {
			if v, ok := capture.Data[field]; ok {
				stateData[field] = v
			}
		}
	}

	// Server-side redaction: scrub sensitive values before persisting to disk (#132)
	if re := h.deps.GetRedactionEngine(); re != nil {
		stateData = re.RedactMapValues(stateData)
	}

	data, err := json.Marshal(stateData)
	if err != nil {
		return fail(req, ErrInternal, "Failed to serialize state: "+err.Error(), "Internal error — do not retry")
	}

	if err := h.sessionStoreImpl.Save(act.StateNamespace, snapshotName, data); err != nil {
		return fail(req, ErrInternal, "Failed to save state: "+err.Error(), "Internal error — check storage")
	}

	h.deps.RecordAIAction("save_state", tabURL, map[string]any{"snapshot_name": snapshotName})

	return succeed(req, "State saved", map[string]any{
		"status":        "saved",
		"snapshot_name": snapshotName,
		"state_capture": capture.Status,
		"state": map[string]any{
			"url":   tabURL,
			"title": tabTitle,
		},
	})
}

func (h *StateInteractHandler) HandleStateLoad(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		SnapshotName string `json:"snapshot_name"`
		Name         string `json:"name"` // backward-compatible alias
		IncludeURL   bool   `json:"include_url,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	snapshotName := resolveStateSnapshotName(params.SnapshotName, params.Name)
	if resp, blocked := requireString(req, snapshotName, "snapshot_name", "Add the 'snapshot_name' parameter (legacy alias: 'name')"); blocked {
		return resp
	}

	if resp, blocked := h.deps.RequireSessionStore(req); blocked {
		return resp
	}

	data, err := h.sessionStoreImpl.Load(act.StateNamespace, snapshotName)
	if err != nil {
		return fail(req, ErrNoData, "State not found: "+snapshotName, "Use interact with action='list_states' to see available snapshots", h.deps.DiagnosticHint())
	}

	var stateData map[string]any
	if err := json.Unmarshal(data, &stateData); err != nil {
		return fail(req, ErrInternal, "Failed to parse state data", "Internal error — state may be corrupted")
	}

	if params.IncludeURL {
		h.QueueStateNavigation(req, stateData)
	}

	responseData := map[string]any{
		"status":        "loaded",
		"snapshot_name": snapshotName,
		"state":         stateData,
	}

	formValues, _ := stateData["form_values"].(map[string]any)
	scrollPos, _ := stateData["scroll_position"].(map[string]any)
	localStorage, _ := stateData["local_storage"].(map[string]any)
	sessionStorage, _ := stateData["session_storage"].(map[string]any)
	cookies, _ := stateData["cookies"].(map[string]any)

	hasData := len(formValues) > 0 || len(localStorage) > 0 || len(sessionStorage) > 0 || len(cookies) > 0

	if !hasData {
		responseData["state_restore"] = act.StateRestoreStatusNoData
	} else if !h.deps.Capture().IsPilotActionAllowed() {
		responseData["state_restore"] = act.StateRestoreStatusPilotDisabled
	} else if !h.deps.Capture().IsExtensionConnected() {
		responseData["state_restore"] = act.StateRestoreStatusExtensionDown
	} else {
		restoreCorrelationID := h.queueStateRestore(req, formValues, scrollPos, localStorage, sessionStorage, cookies)
		responseData["state_restore"] = act.StateRestoreStatusQueued
		responseData["restore_correlation_id"] = restoreCorrelationID
	}

	h.deps.RecordAIAction("load_state", "", map[string]any{"snapshot_name": snapshotName})

	return succeed(req, "State loaded", responseData)
}

func resolveStateSnapshotName(snapshotName, legacyName string) string {
	if snapshotName != "" {
		return snapshotName
	}
	return legacyName
}

func (h *StateInteractHandler) HandleStateList(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if resp, blocked := h.deps.RequireSessionStore(req); blocked {
		return resp
	}

	keys, err := h.sessionStoreImpl.List(act.StateNamespace)
	if err != nil {
		return fail(req, ErrInternal, "Failed to list states: "+err.Error(), "Internal error — do not retry")
	}

	states := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		states = append(states, h.buildStateEntry(key))
	}

	return succeed(req, "States listed", map[string]any{
		"states": states,
		"count":  len(states),
	})
}

// buildStateEntry loads metadata for a single saved state key and returns an entry map.
func (h *StateInteractHandler) buildStateEntry(key string) map[string]any {
	entry := map[string]any{"name": key}
	data, err := h.sessionStoreImpl.Load(act.StateNamespace, key)
	if err != nil {
		return entry
	}
	var stateData map[string]any
	if json.Unmarshal(data, &stateData) != nil {
		return entry
	}
	for _, field := range []string{"url", "title", "saved_at"} {
		if v, ok := stateData[field].(string); ok {
			entry[field] = v
		}
	}
	return entry
}

func (h *StateInteractHandler) HandleStateDelete(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		SnapshotName string `json:"snapshot_name"`
		Name         string `json:"name"` // backward-compatible alias
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	snapshotName := resolveStateSnapshotName(params.SnapshotName, params.Name)
	if resp, blocked := requireString(req, snapshotName, "snapshot_name", "Add the 'snapshot_name' parameter (legacy alias: 'name')"); blocked {
		return resp
	}

	if resp, blocked := h.deps.RequireSessionStore(req); blocked {
		return resp
	}

	if err := h.sessionStoreImpl.Delete(act.StateNamespace, snapshotName); err != nil {
		return fail(req, ErrNoData, "State not found: "+snapshotName, "Use interact with action='list_states' to see available snapshots", h.deps.DiagnosticHint())
	}

	h.deps.RecordAIAction("delete_state", "", map[string]any{"snapshot_name": snapshotName})

	return succeed(req, "State deleted", map[string]any{
		"status":        "deleted",
		"snapshot_name": snapshotName,
	})
}
