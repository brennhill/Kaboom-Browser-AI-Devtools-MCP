// Purpose: Step-level execution helpers for replay_sequence.

package main

import (
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/replay"
)

func resolveReplayStepArgs(seq *Sequence, params sequenceReplayParams, idx int) json.RawMessage {
	stepArgs := seq.Steps[idx]
	if params.OverrideSteps != nil && string(params.OverrideSteps[idx]) != "null" {
		stepArgs = params.OverrideSteps[idx]
	}
	return stepArgs
}

func (h *ToolHandler) executeReplayStep(req JSONRPCRequest, stepArgs json.RawMessage, stepIndex int, stepTimeout time.Duration) (SequenceStepResult, bool) {
	actionName := extractReplayActionName(stepArgs)
	replayStepArgs := replay.ForceAsync(stepArgs)

	stepStart := time.Now()
	stepResp := h.toolInteract(req, replayStepArgs)
	stepDuration := time.Since(stepStart).Milliseconds()

	stepResult := SequenceStepResult{
		StepIndex:  stepIndex,
		Action:     actionName,
		DurationMs: stepDuration,
	}

	if corrID := replay.CorrelationID(stepResp); corrID != "" {
		stepResult.CorrelationID = corrID
		if stepTimeout > 0 {
			cmd, found := h.capture.WaitForCommand(corrID, stepTimeout)
			if found {
				switch cmd.Status {
				case "pending":
					stepResult.Status = "queued"
				case "complete":
					stepResult.Status = "ok"
				default:
					stepResult.Status = "error"
					if cmd.Error != "" {
						stepResult.Error = cmd.Error
					} else {
						stepResult.Error = "command failed with status " + cmd.Status
					}
				}
			} else {
				stepResult.Status = "queued"
			}
		}
	}

	stepRespIsError := isErrorResponse(stepResp)
	if stepRespIsError && stepResult.Status == "" {
		stepResult.Status = "error"
		stepResult.Error = extractErrorMessage(stepResp)
	}
	if stepResult.Status == "" {
		stepResult.Status = "ok"
	}

	return stepResult, stepRespIsError
}

func extractReplayActionName(stepArgs json.RawMessage) string {
	var stepAction struct {
		What   string `json:"what"`
		Action string `json:"action"`
	}
	_ = json.Unmarshal(stepArgs, &stepAction) // best-effort extraction
	if stepAction.What != "" {
		return stepAction.What
	}
	return stepAction.Action
}

// forceReplayAsyncInteractStep ensures replayed interact steps do not block on
// extension execution. This keeps replay_sequence deterministic and avoids
// transport-level timeouts for long-running actions.
func forceReplayAsyncInteractStep(stepArgs json.RawMessage) json.RawMessage {
	return replay.ForceAsync(stepArgs)
}

// extractCorrelationIDFromToolResponse returns correlation_id from JSON tool responses.
func extractCorrelationIDFromToolResponse(resp JSONRPCResponse) string {
	return replay.CorrelationID(resp)
}

// extractErrorMessage extracts the error message text from an error response.
func extractErrorMessage(resp JSONRPCResponse) string {
	if message := replay.ErrorMessage(resp); message != "" {
		return message
	}
	return "unknown error"
}
