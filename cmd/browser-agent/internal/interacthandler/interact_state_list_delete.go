// Purpose: Implements list_states and delete_state snapshot management handlers.
// Why: Keeps snapshot CRUD routes separate from capture/restore execution details.
// Docs: docs/features/feature/state-time-travel/index.md

package interacthandler

import (
	"encoding/json"

	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

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
