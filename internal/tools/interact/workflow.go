// Purpose: Tracks multi-step workflow execution traces with per-step timing, status, and error recording.
// Docs: docs/features/feature/interact-explore/index.md

package interact

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// WorkflowStep records a single step's outcome within a workflow trace.
type WorkflowStep struct {
	Action        string `json:"action"`
	CorrelationID string `json:"correlation_id,omitempty"`
	Status        string `json:"status"` // "success", "error", "skipped"
	TimingMs      int64  `json:"timing_ms"`
	Detail        string `json:"detail,omitempty"`
}

// WorkflowStageTrace is a normalized stage-level trace entry for workflow execution.
type WorkflowStageTrace struct {
	Stage         string `json:"stage"`
	StartedAt     string `json:"started_at"`
	CompletedAt   string `json:"completed_at"`
	DurationMs    int64  `json:"duration_ms"`
	Status        string `json:"status"`
	CorrelationID string `json:"correlation_id,omitempty"`
	Error         string `json:"error,omitempty"`
}

// WorkflowTraceEnvelope is the machine-readable workflow trace contract.
type WorkflowTraceEnvelope struct {
	TraceID     string               `json:"trace_id"`
	Workflow    string               `json:"workflow"`
	StartedAt   string               `json:"started_at"`
	CompletedAt string               `json:"completed_at"`
	DurationMs  int64                `json:"duration_ms"`
	Status      string               `json:"status"`
	Stages      []WorkflowStageTrace `json:"stages"`
}

// FormField represents a single field to fill in a form workflow.
type FormField struct {
	Selector string `json:"selector"`
	Value    string `json:"value"`
	Index    *int   `json:"index,omitempty"`
}

// IsErrorResponse checks if a JSONRPCResponse represents an error.
func IsErrorResponse(resp mcp.JSONRPCResponse) bool {
	if resp.Error != nil {
		return true
	}
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err == nil {
		return result.IsError
	}
	return false
}

// IsNonFinalResponse checks if a JSONRPCResponse is a non-final async response
// (queued or still_processing) that carries a correlation_id for later polling.
func IsNonFinalResponse(resp mcp.JSONRPCResponse) bool {
	if resp.Error != nil {
		return false
	}
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || result.IsError {
		return false
	}
	data, ok := mcp.ReadResultPayload(resp)
	if !ok {
		return false
	}
	final, ok := data["final"].(bool)
	return ok && !final
}

// ResponseStatus returns "success" or "error" based on the response.
func ResponseStatus(resp mcp.JSONRPCResponse) string {
	if IsErrorResponse(resp) {
		return "error"
	}
	return "success"
}

// BuildWorkflowTraceEnvelope generates a normalized stage trace envelope.
func BuildWorkflowTraceEnvelope(workflow string, trace []WorkflowStep, start, end time.Time, status string) WorkflowTraceEnvelope {
	if end.Before(start) {
		end = start
	}
	stages := make([]WorkflowStageTrace, 0, len(trace))
	stageCursor := start
	for _, step := range trace {
		duration := step.TimingMs
		if duration < 0 {
			duration = 0
		}
		stageStart := stageCursor
		stageEnd := stageStart.Add(time.Duration(duration) * time.Millisecond)
		stageCursor = stageEnd

		stage := WorkflowStageTrace{
			Stage:         step.Action,
			StartedAt:     stageStart.UTC().Format(time.RFC3339Nano),
			CompletedAt:   stageEnd.UTC().Format(time.RFC3339Nano),
			DurationMs:    duration,
			Status:        step.Status,
			CorrelationID: step.CorrelationID,
		}
		if step.Status == "error" && step.Detail != "" {
			stage.Error = step.Detail
		}
		stages = append(stages, stage)
	}

	return WorkflowTraceEnvelope{
		TraceID:     fmt.Sprintf("workflow_%s_%d", workflow, start.UTC().UnixNano()),
		Workflow:    workflow,
		StartedAt:   start.UTC().Format(time.RFC3339Nano),
		CompletedAt: end.UTC().Format(time.RFC3339Nano),
		DurationMs:  end.Sub(start).Milliseconds(),
		Status:      status,
		Stages:      stages,
	}
}

// WorkflowResult wraps the final step's response with workflow metadata (trace + timing).
// On failure, the response uses isError=true so the MCP envelope correctly signals an error.
func WorkflowResult(req mcp.JSONRPCRequest, workflow string, trace []WorkflowStep, lastResp mcp.JSONRPCResponse, start time.Time) mcp.JSONRPCResponse {
	end := time.Now()
	totalMs := end.Sub(start).Milliseconds()

	// Count steps by status
	successCount := 0
	for _, s := range trace {
		if s.Status == "success" {
			successCount++
		}
	}
	allSuccess := successCount == len(trace)
	failed := IsErrorResponse(lastResp)

	status := "success"
	if failed {
		status = "failed"
	} else if !allSuccess {
		status = "partial_failure"
	}

	traceEnvelope := BuildWorkflowTraceEnvelope(workflow, trace, start, end, status)

	var summary string
	if failed {
		summary = fmt.Sprintf("%s failed at step %d/%d (%dms)", workflow, len(trace), len(trace), totalMs)
	} else {
		summary = fmt.Sprintf("%s completed (%d/%d steps succeeded, %dms)", workflow, successCount, len(trace), totalMs)
	}

	data := map[string]any{
		"workflow":       workflow,
		"status":         status,
		"trace":          trace,
		"trace_id":       traceEnvelope.TraceID,
		"stages":         traceEnvelope.Stages,
		"workflow_trace": traceEnvelope,
		"total_ms":       totalMs,
		"steps":          len(trace),
		"successful":     successCount,
	}

	// Extract the failing step's error detail for context
	if failed {
		var lastResult mcp.MCPToolResult
		if json.Unmarshal(lastResp.Result, &lastResult) == nil && len(lastResult.Content) > 0 {
			data["error_detail"] = lastResult.Content[0].Text
		} else if lastResp.Error != nil {
			data["error_detail"] = lastResp.Error.Message
		}
	}

	dataJSON, _ := json.Marshal(data)
	resultText := summary + "\n" + string(dataJSON)

	result := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: resultText}},
		IsError: failed,
		Metadata: map[string]any{
			"trace_id":       traceEnvelope.TraceID,
			"workflow_trace": traceEnvelope,
		},
	}
	resultJSON, _ := json.Marshal(result)
	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(resultJSON)}
}
