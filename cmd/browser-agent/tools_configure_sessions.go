// Purpose: Handles configure session-diff wrappers and session manager delegation.
// Why: Separates session diff concerns from noise-rule and audit-log configure actions.
// Docs: docs/features/feature/noise-filtering/index.md

package main

import (
	"encoding/json"

	cfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/configure"
)

// handleDiffSessionsWrapper repackages verif_session_action -> action for handleDiffSessions.
func (h *configureSessionHandler) handleDiffSessionsWrapper(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	rewritten, err := cfg.RewriteDiffSessionsArgs(args)
	if err != nil {
		return fail(req, ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}
	return h.handleDiffSessions(req, rewritten)
}

func (h *configureSessionHandler) handleDiffSessions(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if h.sessionManager == nil {
		return fail(req, ErrNotInitialized, "Session manager not initialized", "Internal error — do not retry")
	}

	result, err := h.sessionManager.HandleTool(args)
	if err != nil {
		return fail(req, ErrInvalidParam, err.Error(), "Fix request parameters and retry")
	}

	responseData := map[string]any{"status": "ok"}
	if m, ok := result.(map[string]any); ok {
		for k, v := range m {
			responseData[k] = v
		}
	} else {
		responseData["result"] = result
	}

	return succeed(req, "Session diff", responseData)
}
