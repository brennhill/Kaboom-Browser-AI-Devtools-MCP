// batch.go — Executes bounded inline interaction sequences under one runtime lock.
// Docs: docs/features/feature/interact-explore/index.md

package interactbatch

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/replay"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

const (
	maxSteps           = replay.MaxSteps
	defaultStepTimeout = replay.DefaultStepTimeout
)

// Deps declares the runtime capabilities needed by inline batch execution.
type Deps struct {
	RequirePilot, RequireExtension toolguard.Check
	Capture                        func() *capture.Capture
	RecordAIAction                 func(string, string, map[string]any)
	Interact                       func(mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse
	ReplayMu                       *sync.Mutex
	Now                            func() time.Time
}

// Handler owns inline batch validation, serialization, and result aggregation.
type Handler struct {
	deps Deps
}

// New constructs a batch handler with runtime-scoped coordination.
func New(deps Deps) *Handler {
	return &Handler{deps: deps}
}

type batchParams struct {
	Steps         []json.RawMessage `json:"steps"`
	StepTimeoutMs int               `json:"step_timeout_ms"`
	ContinueOnErr *bool             `json:"continue_on_error"`
	StopAfterStep int               `json:"stop_after_step"`
}

// Handle executes a sequence of interact steps provided inline.
func (h *Handler) Handle(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	if response, blocked := h.checkBatchRuntime(req); blocked {
		return response
	}

	var params batchParams
	if response, stop := mcp.ParseArgs(req, args, &params); stop {
		return response
	}
	if len(params.Steps) == 0 {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Steps must be a non-empty array", "Add at least one step", mcp.WithParam("steps"))
	}
	if len(params.Steps) > maxSteps {
		return mcp.Fail(req, mcp.ErrInvalidParam, fmt.Sprintf("Steps exceeds maximum of %d", maxSteps), "Split into smaller batches", mcp.WithParam("steps"))
	}

	actions, response, invalid := parseBatchActions(req, params.Steps)
	if invalid {
		return response
	}

	if h.deps.ReplayMu == nil || h.deps.Interact == nil || h.deps.RecordAIAction == nil {
		return mcp.Fail(req, mcp.ErrNotInitialized, "Batch runtime is incomplete", "Restart Kaboom")
	}
	if !h.deps.ReplayMu.TryLock() {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Another batch or sequence is currently executing", "Wait for it to complete")
	}
	defer h.deps.ReplayMu.Unlock()

	if params.StepTimeoutMs <= 0 {
		params.StepTimeoutMs = defaultStepTimeout
	}
	h.deps.RecordAIAction("batch", "", map[string]any{"steps": len(params.Steps)})
	now := h.deps.Now
	if now == nil {
		now = time.Now
	}
	started := now()
	results, executed, failed, queued := h.executeBatchSteps(req, params, actions, now)

	duration := now().Sub(started).Milliseconds()
	status, message := summarize(executed, failed, queued, len(params.Steps), duration)
	return mcp.Succeed(req, "Batch execution", map[string]any{
		"status": status, "steps_executed": executed, "steps_failed": failed,
		"steps_queued": queued, "steps_total": len(params.Steps), "duration_ms": duration,
		"results": results, "message": message,
	})
}

func (h *Handler) checkBatchRuntime(req mcp.JSONRPCRequest) (mcp.JSONRPCResponse, bool) {
	if h == nil {
		return mcp.Fail(req, mcp.ErrNotInitialized, "Batch handler is not initialized", "Restart Kaboom"), true
	}
	for _, guard := range []toolguard.Check{h.deps.RequirePilot, h.deps.RequireExtension} {
		if guard == nil {
			return mcp.Fail(req, mcp.ErrNotInitialized, "Batch runtime is incomplete", "Restart Kaboom"), true
		}
		if response, blocked := guard(req); blocked {
			return response, true
		}
	}
	return mcp.JSONRPCResponse{}, false
}

func parseBatchActions(req mcp.JSONRPCRequest, steps []json.RawMessage) ([]string, mcp.JSONRPCResponse, bool) {
	actions := make([]string, len(steps))
	for index, step := range steps {
		var selector struct {
			What string `json:"what"`
		}
		if err := json.Unmarshal(step, &selector); err != nil {
			return nil, mcp.Fail(req, mcp.ErrInvalidParam, fmt.Sprintf("Step[%d] must be valid JSON", index), "Fix the step JSON", mcp.WithParam("steps")), true
		}
		actions[index] = strings.TrimSpace(selector.What)
		if actions[index] == "" {
			return nil, mcp.Fail(req, mcp.ErrInvalidParam, fmt.Sprintf("Step[%d] missing required 'what' field", index), "Add a 'what' field to each step", mcp.WithParam("steps")), true
		}
	}
	return actions, mcp.JSONRPCResponse{}, false
}

func classifyBatchCommand(store *capture.Capture, correlationID string, timeoutMs int) (string, string) {
	if store == nil {
		return "error", "batch capture runtime unavailable"
	}
	command, found := store.Queries().WaitForCommand(correlationID, time.Duration(timeoutMs)*time.Millisecond)
	if !found {
		return "queued", ""
	}
	switch command.Status {
	case "pending":
		return "queued", ""
	case "complete":
		return "ok", ""
	default:
		if command.Error == "" {
			return "error", "command failed with status " + command.Status
		}
		return "error", command.Error
	}
}

func (h *Handler) executeBatchSteps(req mcp.JSONRPCRequest, params batchParams, actions []string, now func() time.Time) ([]replay.StepResult, int, int, int) {
	results := make([]replay.StepResult, 0, len(params.Steps))
	executed, failed, queued := 0, 0, 0
	limit := len(params.Steps)
	if params.StopAfterStep > 0 && params.StopAfterStep < limit {
		limit = params.StopAfterStep
	}
	continueOnError := params.ContinueOnErr == nil || *params.ContinueOnErr

	for index := 0; index < limit; index++ {
		stepStarted := now()
		response := h.deps.Interact(req, replay.ForceAsync(stripComposableScreenshot(params.Steps[index])))
		step := replay.StepResult{StepIndex: index, Action: actions[index], DurationMs: now().Sub(stepStarted).Milliseconds()}
		if correlationID := replay.CorrelationID(response); correlationID != "" {
			step.CorrelationID = correlationID
			var store *capture.Capture
			if h.deps.Capture != nil {
				store = h.deps.Capture()
			}
			step.Status, step.Error = classifyBatchCommand(store, correlationID, params.StepTimeoutMs)
		}
		if act.IsErrorResponse(response) && step.Status == "" {
			step.Status, step.Error = "error", replay.ErrorMessage(response)
		}
		if step.Status == "" {
			step.Status = "ok"
		}
		if step.Status == "error" {
			failed++
		}
		if step.Status == "queued" {
			queued++
		}
		results = append(results, step)
		executed++
		if step.Status == "error" && !continueOnError {
			break
		}
	}
	return results, executed, failed, queued
}

func summarize(executed, failed, queued, total int, duration int64) (string, string) {
	switch {
	case failed > 0 && executed < total:
		return "error", fmt.Sprintf("Batch failed at step %d/%d", executed, total)
	case failed > 0 && failed == executed:
		return "error", fmt.Sprintf("Batch failed: all %d executed steps had errors", failed)
	case queued > 0 && failed > 0:
		return "partial", fmt.Sprintf("Batch executed with failures: %d queued, %d failed", queued, failed)
	case queued > 0:
		return "queued", fmt.Sprintf("Batch queued: %d/%d steps still running", queued, total)
	case failed > 0:
		return "partial", fmt.Sprintf("Batch partially executed: %d/%d steps succeeded, %d failed", executed-failed, total, failed)
	default:
		return "ok", fmt.Sprintf("Batch executed: %d/%d steps in %dms", executed, total, duration)
	}
}

func stripComposableScreenshot(step json.RawMessage) json.RawMessage {
	var raw map[string]any
	if err := json.Unmarshal(step, &raw); err != nil {
		return step
	}
	if _, exists := raw["include_screenshot"]; !exists {
		return step
	}
	delete(raw, "include_screenshot")
	patched, err := json.Marshal(raw)
	if err != nil {
		return step
	}
	return patched
}
