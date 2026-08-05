// replay.go — Saved-sequence replay planning, execution, and result shaping.
// Docs: docs/features/feature/batch-sequences/index.md

package sequencehandler

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/replay"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

type replayParams struct {
	Name          string            `json:"name"`
	OverrideSteps []json.RawMessage `json:"override_steps"`
	StepTimeoutMs int               `json:"step_timeout_ms"`
	ContinueOnErr *bool             `json:"continue_on_error"`
	StopAfterStep int               `json:"stop_after_step"`
}

type replayPlan struct {
	continueOnError bool
	stepTimeout     time.Duration
	maxSteps        int
}

type replayMetrics struct {
	executed int
	failed   int
	queued   int
}

func (h *Handler) Replay(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params replayParams
	mcp.LenientUnmarshal(args, &params)
	if resp, stop := toolresp.RequireString(req, params.Name, "name", "Add the 'name' parameter"); stop {
		return resp
	}
	sequence, failure := h.load(req, params.Name)
	if failure != nil {
		return *failure
	}
	plan, failure := buildPlan(req, sequence, params)
	if failure != nil {
		return *failure
	}
	if h.deps.ReplayMu == nil || h.deps.Interact == nil {
		return mcp.Fail(req, mcp.ErrNotInitialized, "Sequence replay not initialized", "Internal error — do not retry")
	}
	if !h.deps.ReplayMu.TryLock() {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Another sequence is currently replaying", "Wait for it to complete")
	}
	defer h.deps.ReplayMu.Unlock()
	if h.deps.RecordAction != nil {
		h.deps.RecordAction("replay_sequence", "", map[string]any{"name": params.Name, "steps": sequence.StepCount})
	}
	start := time.Now()
	results, metrics := h.executeSteps(req, sequence, params, plan)
	duration := time.Since(start).Milliseconds()
	status, message := summarize(sequence.StepCount, metrics, duration)
	return mcp.Succeed(req, "Sequence replay", map[string]any{
		"status": status, "name": params.Name, "steps_executed": metrics.executed,
		"steps_failed": metrics.failed, "steps_queued": metrics.queued,
		"steps_total": sequence.StepCount, "duration_ms": duration, "results": results, "message": message,
	})
}

func buildPlan(req mcp.JSONRPCRequest, sequence *Sequence, params replayParams) (replayPlan, *mcp.JSONRPCResponse) {
	if params.OverrideSteps != nil && len(params.OverrideSteps) != sequence.StepCount {
		resp := mcp.Fail(req, mcp.ErrInvalidParam,
			fmt.Sprintf("override_steps length (%d) does not match sequence step count (%d)", len(params.OverrideSteps), sequence.StepCount),
			"Fix array length to match step count", mcp.WithParam("override_steps"))
		return replayPlan{}, &resp
	}
	continueOnError := true
	if params.ContinueOnErr != nil {
		continueOnError = *params.ContinueOnErr
	}
	timeout := params.StepTimeoutMs
	if timeout <= 0 {
		timeout = replay.DefaultStepTimeout
	}
	maxSteps := sequence.StepCount
	if params.StopAfterStep > 0 && params.StopAfterStep < maxSteps {
		maxSteps = params.StopAfterStep
	}
	return replayPlan{continueOnError: continueOnError, stepTimeout: time.Duration(timeout) * time.Millisecond, maxSteps: maxSteps}, nil
}

func (h *Handler) executeSteps(req mcp.JSONRPCRequest, sequence *Sequence, params replayParams, plan replayPlan) ([]replay.StepResult, replayMetrics) {
	results := make([]replay.StepResult, 0, plan.maxSteps)
	var metrics replayMetrics
	for index := 0; index < plan.maxSteps; index++ {
		stepArgs := sequence.Steps[index]
		if params.OverrideSteps != nil && string(params.OverrideSteps[index]) != "null" {
			stepArgs = params.OverrideSteps[index]
		}
		result, failed := h.executeStep(req, stepArgs, index, plan.stepTimeout)
		results = append(results, result)
		metrics.executed++
		switch result.Status {
		case "queued":
			metrics.queued++
		case "error":
			metrics.failed++
		}
		if failed && !plan.continueOnError {
			break
		}
	}
	return results, metrics
}

func (h *Handler) executeStep(req mcp.JSONRPCRequest, args json.RawMessage, index int, timeout time.Duration) (replay.StepResult, bool) {
	start := time.Now()
	response := h.deps.Interact(req, replay.ForceAsync(args))
	result := replay.StepResult{StepIndex: index, Action: actionName(args), DurationMs: time.Since(start).Milliseconds()}
	if correlationID := replay.CorrelationID(response); correlationID != "" {
		result.CorrelationID = correlationID
		if h.deps.WaitForCommand != nil && timeout > 0 {
			command, found := h.deps.WaitForCommand(correlationID, timeout)
			switch {
			case !found:
				result.Status = "queued"
			case command.Status == "pending":
				result.Status = "queued"
			case command.Status == "complete":
				result.Status = "ok"
			default:
				result.Status = "error"
				result.Error = command.Error
				if result.Error == "" {
					result.Error = "command failed with status " + command.Status
				}
			}
		}
	}
	failed := act.IsErrorResponse(response)
	if failed && result.Status == "" {
		result.Status = "error"
		result.Error = replay.ErrorMessage(response)
		if result.Error == "" {
			result.Error = "unknown error"
		}
	}
	if result.Status == "" {
		result.Status = "ok"
	}
	return result, failed
}

func actionName(args json.RawMessage) string {
	var action struct {
		What   string `json:"what"`
		Action string `json:"action"`
	}
	_ = json.Unmarshal(args, &action)
	if action.What != "" {
		return action.What
	}
	return action.Action
}

func summarize(total int, metrics replayMetrics, duration int64) (string, string) {
	switch {
	case metrics.failed > 0 && metrics.executed < total:
		return "error", fmt.Sprintf("Sequence failed at step %d/%d", metrics.executed, total)
	case metrics.queued > 0 && metrics.failed > 0:
		return "partial", fmt.Sprintf("Sequence replay queued with failures: %d queued, %d failed", metrics.queued, metrics.failed)
	case metrics.queued > 0:
		return "queued", fmt.Sprintf("Sequence replay queued: %d/%d steps still running", metrics.queued, total)
	case metrics.failed > 0:
		return "partial", fmt.Sprintf("Sequence partially replayed: %d/%d steps executed, %d failed", metrics.executed-metrics.failed, total, metrics.failed)
	default:
		return "ok", fmt.Sprintf("Sequence replayed: %d/%d steps executed in %dms", metrics.executed, total, duration)
	}
}

func responseIsError(resp mcp.JSONRPCResponse) bool {
	return act.IsErrorResponse(resp)
}
