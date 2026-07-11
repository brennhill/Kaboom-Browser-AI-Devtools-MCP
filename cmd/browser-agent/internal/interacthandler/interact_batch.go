// Purpose: Executes a sequence of interact actions inline in a single tool call, without requiring a saved sequence.
// Why: Reduces round-trip overhead when agents need to execute predictable multi-step interactions.
// Docs: docs/features/feature/batch-sequences/index.md

package interacthandler

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Batch/replay shared constants and state.
// NOTE: These duplicate configurehandler.MaxSequenceSteps and configurehandler.DefaultStepTimeout.
// Keep both in sync. A cross-package import is avoided to prevent a dependency cycle.
const (
	maxSequenceSteps   = 50
	defaultStepTimeout = 10_000
)

// ReplayMu prevents concurrent batch/replay execution.
var ReplayMu sync.Mutex

// SequenceStepResult holds the result of a single step in a batch/sequence.
type SequenceStepResult struct {
	StepIndex     int    `json:"step_index"`
	Action        string `json:"action"`
	CorrelationID string `json:"correlation_id,omitempty"`
	Status        string `json:"status"`
	Error         string `json:"error,omitempty"`
	DurationMs    int64  `json:"duration_ms"`
}

// forceReplayAsyncInteractStep ensures replayed interact steps do not block on
// MaybeWaitForCommand. Batch handles its own result polling via WaitForCommand.
func forceReplayAsyncInteractStep(stepArgs json.RawMessage) json.RawMessage {
	var argsMap map[string]any
	if err := json.Unmarshal(stepArgs, &argsMap); err != nil {
		return stepArgs
	}
	argsMap["sync"] = false
	argsMap["wait"] = false
	updated, err := json.Marshal(argsMap)
	if err != nil {
		return stepArgs
	}
	return updated
}

// extractCorrelationIDFromToolResponse extracts correlation_id from a tool response.
func extractCorrelationIDFromToolResponse(resp JSONRPCResponse) string {
	if resp.Result == nil {
		return ""
	}
	var result MCPToolResult
	if json.Unmarshal(resp.Result, &result) != nil || len(result.Content) == 0 {
		return ""
	}
	for _, block := range result.Content {
		if block.Type != "text" || block.Text == "" {
			continue
		}
		text := block.Text
		if idx := strings.Index(text, "\n{"); idx >= 0 {
			text = text[idx+1:]
		}
		var data map[string]any
		if json.Unmarshal([]byte(text), &data) != nil {
			continue
		}
		if cid, ok := data["correlation_id"].(string); ok && cid != "" {
			return cid
		}
	}
	return ""
}

// extractErrorMessage extracts an error message from a tool response.
func extractErrorMessage(resp JSONRPCResponse) string {
	if resp.Result == nil {
		return ""
	}
	var result MCPToolResult
	if json.Unmarshal(resp.Result, &result) != nil {
		return ""
	}
	for _, block := range result.Content {
		if block.Type != "text" || block.Text == "" {
			continue
		}
		var data map[string]any
		if json.Unmarshal([]byte(block.Text), &data) == nil {
			if msg, ok := data["message"].(string); ok {
				return msg
			}
		}
		return block.Text
	}
	return ""
}

// handleBatch executes a sequence of interact steps provided inline.
func (h *InteractActionHandler) HandleBatch(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Fail fast if pilot/extension are not available — avoids acquiring replayMu
	// and iterating steps that would all fail individually (#9.R3.9).
	if resp, blocked := checkGuards(req, h.deps.RequirePilot, h.deps.RequireExtension); blocked {
		return resp
	}

	var params struct {
		Steps         []json.RawMessage `json:"steps"`
		StepTimeoutMs int               `json:"step_timeout_ms"`
		ContinueOnErr *bool             `json:"continue_on_error"`
		StopAfterStep int               `json:"stop_after_step"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	// Validate steps
	if len(params.Steps) == 0 {
		return fail(req, ErrInvalidParam, "Steps must be a non-empty array", "Add at least one step", withParam("steps"))
	}
	if len(params.Steps) > maxSequenceSteps {
		return fail(req, ErrInvalidParam, fmt.Sprintf("Steps exceeds maximum of %d", maxSequenceSteps), "Split into smaller batches", withParam("steps"))
	}

	// Validate each step has a what (or action) field
	for i, step := range params.Steps {
		var s struct {
			What   string `json:"what"`
			Action string `json:"action"`
		}
		if err := json.Unmarshal(step, &s); err != nil || (s.What == "" && s.Action == "") {
			return fail(req, ErrInvalidParam, fmt.Sprintf("Step[%d] missing required 'what' field", i), "Add a 'what' field to each step", withParam("steps"))
		}
	}

	// Default continue_on_error to true
	continueOnError := true
	if params.ContinueOnErr != nil {
		continueOnError = *params.ContinueOnErr
	}

	if params.StepTimeoutMs <= 0 {
		params.StepTimeoutMs = defaultStepTimeout
	}

	// Acquire replay mutex (prevent concurrent batch/replay)
	mu := h.deps.ReplayMu
	if mu == nil {
		mu = &ReplayMu
	}
	if !mu.TryLock() {
		return fail(req, ErrInvalidParam, "Another batch or sequence is currently executing", "Wait for it to complete")
	}
	defer mu.Unlock()

	// Record audit trail
	h.deps.RecordAIAction("batch", "", map[string]any{"steps": len(params.Steps)})

	start := time.Now()
	results := make([]SequenceStepResult, 0, len(params.Steps))
	stepsExecuted := 0
	stepsFailed := 0
	stepsQueued := 0
	maxSteps := len(params.Steps)
	if params.StopAfterStep > 0 && params.StopAfterStep < maxSteps {
		maxSteps = params.StopAfterStep
	}

	for i := 0; i < maxSteps; i++ {
		stepArgs := params.Steps[i]

		// Extract action name for result
		var stepAction struct {
			What   string `json:"what"`
			Action string `json:"action"`
		}
		json.Unmarshal(stepArgs, &stepAction) //nolint:errcheck // best-effort extraction
		actionName := stepAction.What
		if actionName == "" {
			actionName = stepAction.Action
		}

		// Strip include_screenshot from batch steps — screenshots are captured per-step
		// but then discarded in the aggregate response, wasting CPU on base64 encoding (#9.2.2).
		stepArgs = StripComposableScreenshotFromStep(stepArgs)

		replayStepArgs := forceReplayAsyncInteractStep(stepArgs)
		stepStart := time.Now()
		stepResp := h.deps.ToolInteract(req, replayStepArgs)
		stepDuration := time.Since(stepStart).Milliseconds()

		stepResult := SequenceStepResult{
			StepIndex:  i,
			Action:     actionName,
			DurationMs: stepDuration,
		}

		if corrID := extractCorrelationIDFromToolResponse(stepResp); corrID != "" {
			stepResult.CorrelationID = corrID
			timeout := time.Duration(params.StepTimeoutMs) * time.Millisecond
			if timeout > 0 {
				cmd, found := h.deps.Capture().WaitForCommand(corrID, timeout)
				if found {
					switch cmd.Status {
					case "pending":
						stepResult.Status = "queued"
						stepsQueued++
					case "complete":
						stepResult.Status = "ok"
					default:
						stepResult.Status = "error"
						if cmd.Error != "" {
							stepResult.Error = cmd.Error
						} else {
							stepResult.Error = "command failed with status " + cmd.Status
						}
						stepsFailed++
					}
				} else {
					stepResult.Status = "queued"
					stepsQueued++
				}
			}
		}

		if isErrorResponse(stepResp) {
			// Only count as failed if not already counted by the correlation path above (#9.R1).
			// In the contradictory case where correlation resolved to "ok" but isErrorResponse
			// is true, we trust the correlation result since it reflects the actual extension-side
			// outcome. This cannot happen in practice because forceReplayAsyncInteractStep generates
			// a fresh correlation ID per step.
			if stepResult.Status == "" {
				stepResult.Status = "error"
				stepResult.Error = extractErrorMessage(stepResp)
				stepsFailed++
			}
			results = append(results, stepResult)
			stepsExecuted++
			if !continueOnError {
				break
			}
			continue
		}

		if stepResult.Status == "" {
			stepResult.Status = "ok"
		}
		stepsExecuted++
		results = append(results, stepResult)
	}

	totalDuration := time.Since(start).Milliseconds()

	status := "ok"
	var message string
	stepsTotal := len(params.Steps)
	if stepsFailed > 0 && stepsExecuted < stepsTotal {
		// Stopped early (continue_on_error=false)
		status = "error"
		message = fmt.Sprintf("Batch failed at step %d/%d", stepsExecuted, stepsTotal)
	} else if stepsFailed > 0 && stepsFailed == stepsExecuted {
		// All executed steps failed
		status = "error"
		message = fmt.Sprintf("Batch failed: all %d executed steps had errors", stepsFailed)
	} else if stepsQueued > 0 && stepsFailed > 0 {
		status = "partial"
		message = fmt.Sprintf("Batch executed with failures: %d queued, %d failed", stepsQueued, stepsFailed)
	} else if stepsQueued > 0 {
		status = "queued"
		message = fmt.Sprintf("Batch queued: %d/%d steps still running", stepsQueued, stepsTotal)
	} else if stepsFailed > 0 {
		status = "partial"
		message = fmt.Sprintf("Batch partially executed: %d/%d steps succeeded, %d failed", stepsExecuted-stepsFailed, stepsTotal, stepsFailed)
	} else {
		message = fmt.Sprintf("Batch executed: %d/%d steps in %dms", stepsExecuted, stepsTotal, totalDuration)
	}

	responseData := map[string]any{
		"status":         status,
		"steps_executed": stepsExecuted,
		"steps_failed":   stepsFailed,
		"steps_queued":   stepsQueued,
		"steps_total":    stepsTotal,
		"duration_ms":    totalDuration,
		"results":        results,
		"message":        message,
	}

	return succeed(req, "Batch execution", responseData)
}

// StripComposableScreenshotFromStep removes include_screenshot from batch step args
// to prevent wasted screenshot captures that are discarded in the aggregate response.
func StripComposableScreenshotFromStep(stepArgs json.RawMessage) json.RawMessage {
	var raw map[string]any
	if err := json.Unmarshal(stepArgs, &raw); err != nil {
		return stepArgs
	}
	if _, has := raw["include_screenshot"]; has {
		delete(raw, "include_screenshot")
		patched, err := json.Marshal(raw)
		if err != nil {
			return stepArgs
		}
		return patched
	}
	return stepArgs
}
