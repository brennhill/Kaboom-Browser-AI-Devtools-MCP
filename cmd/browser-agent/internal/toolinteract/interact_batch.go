// Purpose: Executes a sequence of interact actions inline in a single tool call, without requiring a saved sequence.
// Why: Reduces round-trip overhead when agents need to execute predictable multi-step interactions.
// Docs: docs/features/feature/batch-sequences/index.md

package toolinteract

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/replay"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

const (
	maxSequenceSteps   = replay.MaxSteps
	defaultStepTimeout = replay.DefaultStepTimeout
)

// ReplayMu prevents concurrent batch/replay execution.
var ReplayMu sync.Mutex

// handleBatch executes a sequence of interact steps provided inline.
func (h *InteractActionHandler) HandleBatch(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
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
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}

	// Validate steps
	if len(params.Steps) == 0 {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Steps must be a non-empty array", "Add at least one step", mcp.WithParam("steps"))
	}
	if len(params.Steps) > maxSequenceSteps {
		return mcp.Fail(req, mcp.ErrInvalidParam, fmt.Sprintf("Steps exceeds maximum of %d", maxSequenceSteps), "Split into smaller batches", mcp.WithParam("steps"))
	}

	// Validate each step has a what (or action) field
	for i, step := range params.Steps {
		var s struct {
			What   string `json:"what"`
			Action string `json:"action"`
		}
		if err := json.Unmarshal(step, &s); err != nil || (s.What == "" && s.Action == "") {
			return mcp.Fail(req, mcp.ErrInvalidParam, fmt.Sprintf("Step[%d] missing required 'what' field", i), "Add a 'what' field to each step", mcp.WithParam("steps"))
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
		return mcp.Fail(req, mcp.ErrInvalidParam, "Another batch or sequence is currently executing", "Wait for it to complete")
	}
	defer mu.Unlock()

	// Record audit trail
	h.deps.RecordAIAction("batch", "", map[string]any{"steps": len(params.Steps)})

	start := time.Now()
	results := make([]replay.StepResult, 0, len(params.Steps))
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

		replayStepArgs := replay.ForceAsync(stepArgs)
		stepStart := time.Now()
		stepResp := h.deps.ToolInteract(req, replayStepArgs)
		stepDuration := time.Since(stepStart).Milliseconds()

		stepResult := replay.StepResult{
			StepIndex:  i,
			Action:     actionName,
			DurationMs: stepDuration,
		}

		if corrID := replay.CorrelationID(stepResp); corrID != "" {
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

		if act.IsErrorResponse(stepResp) {
			// Only count as failed if not already counted by the correlation path above (#9.R1).
			// In the contradictory case where correlation resolved to "ok" but isErrorResponse
			// is true, we trust the correlation result since it reflects the actual extension-side
			// outcome. This cannot happen in practice because replay.ForceAsync generates
			// a fresh correlation ID per step.
			if stepResult.Status == "" {
				stepResult.Status = "error"
				stepResult.Error = replay.ErrorMessage(stepResp)
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

	return mcp.Succeed(req, "Batch execution", responseData)
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
