// Purpose: Interact screen-recording state-machine helpers.
// Why: Keeps command-result interpretation and state transitions isolated from request handlers.
// Docs: docs/features/feature/tab-recording/index.md

package screenrec

import (
	"encoding/json"
	"strings"
	"time"
)

// extractRecordingLifecycleStatus pulls the extension-reported lifecycle status
// from command result payloads ("recording", "saved", "error", etc.).
func extractRecordingLifecycleStatus(result json.RawMessage) string {
	if len(result) == 0 {
		return ""
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(payload.Status))
}

// resolveInteractRecordingState refreshes state using latest command results.
func (r *InteractHandler) resolveInteractRecordingState() State {
	r.recordInteractMu.Lock()
	defer r.recordInteractMu.Unlock()

	state := r.recordInteract
	if state.State == "" {
		state.State = recordingStateIdle
	}

	if state.StopCorrelationID != "" {
		if stopCmd, found := r.deps.GetCommandResult(state.StopCorrelationID); found {
			if stopCmd.Status == "pending" {
				state.State = recordingStateStopping
				state.UpdatedAt = time.Now()
				r.recordInteract = state
				return state
			}
			// Any terminal stop result returns the state machine to idle.
			state = State{State: recordingStateIdle, UpdatedAt: time.Now()}
			r.recordInteract = state
			return state
		}
	}

	if state.StartCorrelationID == "" {
		state.State = recordingStateIdle
		state.UpdatedAt = time.Now()
		r.recordInteract = state
		return state
	}

	startCmd, found := r.deps.GetCommandResult(state.StartCorrelationID)
	if !found {
		// Keep queued state until command result appears.
		if state.State == "" {
			state.State = recordingStateAwaitingGesture
		}
		state.UpdatedAt = time.Now()
		r.recordInteract = state
		return state
	}

	switch startCmd.Status {
	case "pending":
		state.State = recordingStateAwaitingGesture
	case "complete":
		switch extractRecordingLifecycleStatus(startCmd.Result) {
		case recordingStateRecording:
			state.State = recordingStateRecording
		case recordingStateAwaitingGesture:
			state.State = recordingStateAwaitingGesture
		default:
			state = State{State: recordingStateIdle}
		}
	default:
		// error/timeout/expired/cancelled and unknown statuses are terminal.
		state = State{State: recordingStateIdle}
	}

	state.UpdatedAt = time.Now()
	r.recordInteract = state
	return state
}

func (r *InteractHandler) setInteractRecordingStart(correlationID string) {
	r.recordInteractMu.Lock()
	defer r.recordInteractMu.Unlock()
	r.recordInteract = State{
		State:              recordingStateAwaitingGesture,
		StartCorrelationID: correlationID,
		UpdatedAt:          time.Now(),
	}
}

func (r *InteractHandler) setInteractRecordingStopping(correlationID string) {
	r.recordInteractMu.Lock()
	defer r.recordInteractMu.Unlock()
	state := r.recordInteract
	if state.State == "" {
		state.State = recordingStateIdle
	}
	state.State = recordingStateStopping
	state.StopCorrelationID = correlationID
	state.UpdatedAt = time.Now()
	r.recordInteract = state
}
