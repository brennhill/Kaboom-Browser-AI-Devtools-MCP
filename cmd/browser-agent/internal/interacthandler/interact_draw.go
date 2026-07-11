// Purpose: Queues draw_mode_start queries for the extension to activate the user annotation overlay.
// Why: Separates draw mode activation from annotation retrieval to support async user interaction.
// Docs: docs/features/feature/annotated-screenshots/index.md
package interacthandler

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// handleDrawModeStart queues a draw_mode query for the extension to activate draw mode.
func (h *InteractActionHandler) HandleDrawModeStart(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID        int    `json:"tab_id,omitempty"`
		AnnotSession string `json:"annot_session,omitempty"`
	}
	if len(args) > 0 {
		lenientUnmarshal(args, &params)
	}

	if resp, blocked := checkGuards(req, h.deps.RequirePilot, h.deps.RequireExtension); blocked {
		return resp
	}
	if resp, blocked := h.deps.RequireTabTracking(req); blocked {
		return resp
	}

	correlationID := newCorrelationID("draw")

	queryParams := map[string]string{"action": "start"}
	if params.AnnotSession != "" {
		queryParams["annot_session"] = params.AnnotSession
	}
	// Error impossible: map contains only string values
	queryParamsJSON, _ := json.Marshal(queryParams)

	query := queries.PendingQuery{
		Type:          "draw_mode",
		Params:        queryParamsJSON,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := h.deps.EnqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	// Mark draw started AFTER the query is queued, so WaitForSession's timestamp
	// baseline is never set before the command that triggers the session exists.
	h.deps.MarkDrawStarted()

	// Record AI action
	h.deps.RecordAIAction("draw_mode_start", "", nil)

	return succeed(req, "Draw mode activated", map[string]any{
		"status":         "queued",
		"correlation_id": correlationID,
		"message":        "Draw mode activation queued. The user can now draw annotations on the page. Use analyze({what: 'annotations', wait: true}) to block until the user finishes drawing.",
	})
}
