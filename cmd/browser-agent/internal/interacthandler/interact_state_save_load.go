// Purpose: Implements save_state and load_state request handlers.
// Why: Keeps state persistence API behavior modular and independent from capture internals.
// Docs: docs/features/feature/state-time-travel/index.md

package interacthandler

import (
	"encoding/json"
	"time"

	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

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
