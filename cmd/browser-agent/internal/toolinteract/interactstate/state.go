// state.go — Handler: save/load/list/delete of page-state snapshots, plus the
// browser-side capture and restore commands they queue.
//
// Package interactstate owns the state-time-travel actions of the interact tool.
// It is a separate package because it is a separate handler: it never touches
// browser, DOM, or workflow owner; its persistence dependency (a SessionStore)
// is nobody else's. Everything it asks for arrives through Deps as a
// function value, which is what lets every branch here be tested with fakes and
// no real browser, extension or disk.
//
// Docs: docs/features/feature/state-time-travel/index.md
package interactstate

import (
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

// stateCaptureTimeout is the time to wait for the extension to execute
// the state capture script and return form/scroll/storage data.
const stateCaptureTimeout = 5 * time.Second

// Deps are the host-owned seams this package needs. Each is a function value so
// the host can rebuild it per construction and late-bind its own state; nothing
// here reaches into the host's types.
type Deps struct {
	// IsPilotActionAllowed reports whether pilot mode permits driving the page.
	IsPilotActionAllowed func() bool
	// IsExtensionConnected reports whether the browser extension is attached.
	IsExtensionConnected func() bool
	// GetTrackingStatus returns (tracking, tabID, tabURL) for the tracked tab.
	GetTrackingStatus func() (bool, int, string)
	// GetTrackedTabTitle returns the tracked tab's title, or "" when unknown.
	GetTrackedTabTitle func() string
	// WaitForCommand blocks until the extension answers correlationID or timeout.
	WaitForCommand func(correlationID string, timeout time.Duration) (*queries.CommandResult, bool)

	// EnqueuePendingQuery queues a command for the extension.
	EnqueuePendingQuery func(req mcp.JSONRPCRequest, query queries.PendingQuery, timeout time.Duration) (mcp.JSONRPCResponse, bool)
	// RecordAIAction records an AI-driven action to the enhanced actions buffer.
	RecordAIAction func(action, url string, extra map[string]any)
	// RequireSessionStore checks that the session store is available.
	RequireSessionStore func(req mcp.JSONRPCRequest) (mcp.JSONRPCResponse, bool)
	// DiagnosticHint returns a StructuredError option carrying diagnostic context.
	DiagnosticHint func() func(*mcp.StructuredError)
	// Redact scrubs sensitive values before a snapshot reaches disk. It must be
	// total: when no redaction engine is configured the host returns m unchanged.
	Redact func(m map[string]any) map[string]any
}

// Handler handles state save/load/list/delete operations.
type Handler struct {
	deps *Deps

	// Concrete session store injected at construction.
	sessionStoreImpl *persistence.SessionStore
}

// New creates a new Handler with the given dependencies.
func New(deps *Deps, store *persistence.SessionStore) *Handler {
	return &Handler{
		deps:             deps,
		sessionStoreImpl: store,
	}
}

// CaptureState attempts to capture form values, scroll position, and web storage from the browser.
// It always returns the canonical result with an explicit status the caller can surface to the LLM.
func (h *Handler) CaptureState(req mcp.JSONRPCRequest) act.StateCaptureResult {
	if !h.deps.IsPilotActionAllowed() {
		return act.StateCaptureResult{Status: act.StateCaptureStatusPilotDisabled}
	}
	if !h.deps.IsExtensionConnected() {
		return act.StateCaptureResult{Status: act.StateCaptureStatusExtensionDisconnected}
	}

	correlationID := toolresp.NewCorrelationID("state_capture")

	scriptArgs := mcp.SafeMarshal(map[string]any{
		"action": "execute_js",
		"script": act.StateCaptureScript,
		"world":  "main",
	}, "{}")

	query := queries.PendingQuery{
		Type:          "execute",
		Params:        scriptArgs,
		CorrelationID: correlationID,
	}
	if _, blocked := h.deps.EnqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return act.StateCaptureResult{Status: act.StateCaptureStatusError}
	}

	cmd, found := h.deps.WaitForCommand(correlationID, stateCaptureTimeout)
	if !found || cmd.Status == "pending" {
		return act.StateCaptureResult{Status: act.StateCaptureStatusTimeout}
	}
	if cmd.Error != "" {
		return act.StateCaptureResult{Status: act.StateCaptureStatusError}
	}
	if cmd.Status != "complete" || len(cmd.Result) == 0 {
		return act.StateCaptureResult{Status: act.StateCaptureStatusError}
	}

	captureData, err := act.ParseCapturedStatePayload(cmd.Result)
	if err != nil {
		return act.StateCaptureResult{Status: act.StateCaptureStatusError}
	}

	return act.StateCaptureResult{Status: act.StateCaptureStatusCaptured, Data: captureData}
}

// queueStateRestore queues a JS execute command to restore form values, scroll position,
// localStorage, sessionStorage, and cookies. This is fire-and-forget.
func (h *Handler) queueStateRestore(req mcp.JSONRPCRequest, formValues, scrollPos, localStorage, sessionStorage, cookies map[string]any) string {
	correlationID := toolresp.NewCorrelationID("state_restore")

	script := act.BuildStateRestoreScript(formValues, scrollPos, localStorage, sessionStorage, cookies)
	scriptArgs := mcp.SafeMarshal(map[string]any{
		"action": "execute_js",
		"script": script,
		"world":  "main",
	}, "{}")

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

// QueueStateNavigation queues a navigation to the saved URL if pilot is enabled
// and the state contains a non-empty URL. Mutates stateData to add tracking fields.
func (h *Handler) QueueStateNavigation(req mcp.JSONRPCRequest, stateData map[string]any) {
	savedURL, ok := stateData["url"].(string)
	if !ok || savedURL == "" || !h.deps.IsPilotActionAllowed() || !h.deps.IsExtensionConnected() {
		return
	}
	correlationID := toolresp.NewCorrelationID("nav")
	// Error impossible: map contains only string values
	navArgs := mcp.SafeMarshal(map[string]any{"action": "navigate", "url": savedURL}, "{}")
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

type snapshotRequest struct {
	SnapshotName string `json:"snapshot_name"`
	IncludeURL   bool   `json:"include_url,omitempty"`
}

func (h *Handler) parseSnapshotRequest(req mcp.JSONRPCRequest, args json.RawMessage) (snapshotRequest, mcp.JSONRPCResponse, bool) {
	var params snapshotRequest
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return snapshotRequest{}, resp, true
	}
	if resp, blocked := requireSnapshotName(req, params.SnapshotName); blocked {
		return snapshotRequest{}, resp, true
	}
	if resp, blocked := h.deps.RequireSessionStore(req); blocked {
		return snapshotRequest{}, resp, true
	}
	return params, mcp.JSONRPCResponse{}, false
}

// HandleStateSave persists a named snapshot of the tracked tab's page state.
func (h *Handler) HandleStateSave(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	params, resp, stop := h.parseSnapshotRequest(req, args)
	if stop {
		return resp
	}
	snapshotName := params.SnapshotName

	_, tabID, tabURL := h.deps.GetTrackingStatus()
	tabTitle := h.deps.GetTrackedTabTitle()

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
	stateData = h.deps.Redact(stateData)

	data, err := json.Marshal(stateData)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInternal, "Failed to serialize state: "+err.Error(), "Internal error — do not retry")
	}

	if err := h.sessionStoreImpl.Save(act.StateNamespace, snapshotName, data); err != nil {
		return mcp.Fail(req, mcp.ErrInternal, "Failed to save state: "+err.Error(), "Internal error — check storage")
	}

	h.deps.RecordAIAction("save_state", tabURL, map[string]any{"snapshot_name": snapshotName})

	return mcp.Succeed(req, "State saved", map[string]any{
		"status":        "saved",
		"snapshot_name": snapshotName,
		"state_capture": capture.Status,
		"state": map[string]any{
			"url":   tabURL,
			"title": tabTitle,
		},
	})
}

// HandleStateLoad restores a named snapshot into the tracked tab.
func (h *Handler) HandleStateLoad(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	params, resp, stop := h.parseSnapshotRequest(req, args)
	if stop {
		return resp
	}
	snapshotName := params.SnapshotName

	data, err := h.sessionStoreImpl.Load(act.StateNamespace, snapshotName)
	if err != nil {
		return mcp.Fail(req, mcp.ErrNoData, "State not found: "+snapshotName, "Use interact with action='list_states' to see available snapshots", h.deps.DiagnosticHint())
	}

	var stateData map[string]any
	if err := json.Unmarshal(data, &stateData); err != nil {
		return mcp.Fail(req, mcp.ErrInternal, "Failed to parse state data", "Internal error — state may be corrupted")
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
	} else if !h.deps.IsPilotActionAllowed() {
		responseData["state_restore"] = act.StateRestoreStatusPilotDisabled
	} else if !h.deps.IsExtensionConnected() {
		responseData["state_restore"] = act.StateRestoreStatusExtensionDown
	} else {
		restoreCorrelationID := h.queueStateRestore(req, formValues, scrollPos, localStorage, sessionStorage, cookies)
		responseData["state_restore"] = act.StateRestoreStatusQueued
		responseData["restore_correlation_id"] = restoreCorrelationID
	}

	h.deps.RecordAIAction("load_state", "", map[string]any{"snapshot_name": snapshotName})

	return mcp.Succeed(req, "State loaded", responseData)
}

// requireSnapshotName rejects a request that named no snapshot, with the same
// message for every action so the recovery hint stays consistent.
func requireSnapshotName(req mcp.JSONRPCRequest, snapshotName string) (mcp.JSONRPCResponse, bool) {
	if snapshotName != "" {
		return mcp.JSONRPCResponse{}, false
	}
	return mcp.Fail(req, mcp.ErrMissingParam,
		"Required parameter 'snapshot_name' is missing",
		"Add the 'snapshot_name' parameter",
		mcp.WithParam("snapshot_name")), true
}

// HandleStateList lists the saved snapshots with their metadata.
func (h *Handler) HandleStateList(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	if resp, blocked := h.deps.RequireSessionStore(req); blocked {
		return resp
	}

	keys, err := h.sessionStoreImpl.List(act.StateNamespace)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInternal, "Failed to list states: "+err.Error(), "Internal error — do not retry")
	}

	states := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		states = append(states, h.buildStateEntry(key))
	}

	return mcp.Succeed(req, "States listed", map[string]any{
		"states": states,
		"count":  len(states),
	})
}

// buildStateEntry loads metadata for a single saved state key and returns an entry map.
func (h *Handler) buildStateEntry(key string) map[string]any {
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

// HandleStateDelete removes a saved snapshot.
func (h *Handler) HandleStateDelete(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	params, resp, stop := h.parseSnapshotRequest(req, args)
	if stop {
		return resp
	}
	snapshotName := params.SnapshotName

	if err := h.sessionStoreImpl.Delete(act.StateNamespace, snapshotName); err != nil {
		return mcp.Fail(req, mcp.ErrNoData, "State not found: "+snapshotName, "Use interact with action='list_states' to see available snapshots", h.deps.DiagnosticHint())
	}

	h.deps.RecordAIAction("delete_state", "", map[string]any{"snapshot_name": snapshotName})

	return mcp.Succeed(req, "State deleted", map[string]any{
		"status":        "deleted",
		"snapshot_name": snapshotName,
	})
}
