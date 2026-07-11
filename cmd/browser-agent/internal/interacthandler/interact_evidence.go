// Purpose: Implements the evidence capture state machine — before/after screenshot pairs for interact actions.
// Why: Enables visual regression proof by capturing screenshots around DOM-mutating actions when evidence mode is enabled.
// Docs: docs/features/feature/interact-explore/index.md

package interacthandler

import (
	"encoding/json"
	"strings"
)

func (h *InteractActionHandler) ArmEvidenceForCommand(correlationID, action string, args json.RawMessage, clientID string) {
	if h == nil || correlationID == "" {
		return
	}

	h.armRetryContract(correlationID, action, args)

	mode, err := ParseEvidenceMode(args)
	if err != nil {
		return
	}

	if mode == evidenceModeOff {
		h.clearEvidenceState(correlationID)
		return
	}

	if action == "" {
		action = canonicalActionFromInteractArgs(args)
	}

	maxCaptures := evidenceMaxCapturesPerCommand()
	state := &commandEvidenceState{
		mode:        mode,
		action:      strings.ToLower(strings.TrimSpace(action)),
		maxCaptures: maxCaptures,
		clientID:    clientID,
	}

	switch mode {
	case evidenceModeAlways:
		state.shouldCapture = true
	case evidenceModeOnMutation:
		state.shouldCapture = isMutationAction(state.action)
		if !state.shouldCapture {
			state.skipped = "non_mutating_action"
		}
	}

	if state.shouldCapture && state.maxCaptures <= 0 {
		state.shouldCapture = false
		state.skipped = "capture_budget_zero"
	}

	if state.shouldCapture {
		state.before = h.captureEvidenceWithRetry(clientID)
	}

	h.storeEvidenceState(correlationID, state)
}

func (h *InteractActionHandler) AttachEvidencePayload(correlationID string, responseData map[string]any) {
	if h == nil || correlationID == "" || responseData == nil {
		return
	}

	cached, needsAfter, clientID, done := h.loadEvidenceAttachContext(correlationID)
	if done {
		if cached != nil {
			responseData["evidence"] = cached
		}
		return
	}

	var after evidenceShot
	if needsAfter {
		after = h.captureEvidenceWithRetry(clientID)
	}

	payload, ok := h.finalizeEvidencePayload(correlationID, needsAfter, after)
	if !ok {
		return
	}

	responseData["evidence"] = payload
}
