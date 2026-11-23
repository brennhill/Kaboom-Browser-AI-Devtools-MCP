// Purpose: Tests for interact workflow orchestration.
// Docs: docs/features/feature/interact-explore/index.md

// workflow_test.go — Tests for workflow helper functions.
package interact

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/mcp"
)

func TestIsErrorResponse_NilError(t *testing.T) {
	t.Parallel()
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0"}
	if IsErrorResponse(resp) {
		t.Error("expected non-error for empty response with no error field")
	}
}

func TestIsErrorResponse_WithJSONRPCError(t *testing.T) {
	t.Parallel()
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", Error: &mcp.JSONRPCError{Code: -32600, Message: "bad"}}
	if !IsErrorResponse(resp) {
		t.Error("expected error response when Error field is set")
	}
}

func TestIsErrorResponse_WithMCPToolError(t *testing.T) {
	t.Parallel()
	result := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: "fail"}},
		IsError: true,
	}
	raw, _ := json.Marshal(result)
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", Result: raw}
	if !IsErrorResponse(resp) {
		t.Error("expected error when isError=true in MCPToolResult")
	}
}

func TestResponseStatus(t *testing.T) {
	t.Parallel()
	ok := mcp.JSONRPCResponse{JSONRPC: "2.0"}
	if ResponseStatus(ok) != "success" {
		t.Error("expected success for non-error response")
	}

	err := mcp.JSONRPCResponse{JSONRPC: "2.0", Error: &mcp.JSONRPCError{Code: -1, Message: "x"}}
	if ResponseStatus(err) != "error" {
		t.Error("expected error for error response")
	}
}

func TestWorkflowResult_AllSuccess(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	trace := []WorkflowStep{
		{Action: "step1", Status: "success", TimingMs: 10},
		{Action: "step2", Status: "success", TimingMs: 20},
	}
	okResp := mcp.JSONRPCResponse{JSONRPC: "2.0"}
	resp := WorkflowResult(req, "test_workflow", trace, okResp, time.Now())

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal MCPToolResult: %v", err)
	}
	if result.IsError {
		t.Fatal("expected non-error result")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content blocks")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, `"status":"success"`) {
		t.Errorf("expected status=success in text, got: %s", text)
	}
	if !strings.Contains(text, `"successful":2`) {
		t.Errorf("expected successful=2 in text, got: %s", text)
	}
	if !strings.Contains(text, "test_workflow completed") {
		t.Errorf("expected summary line, got: %s", text)
	}
	if !strings.Contains(text, `"trace_id":"workflow_test_workflow_`) {
		t.Errorf("expected trace_id in workflow payload, got: %s", text)
	}
	if !strings.Contains(text, `"stages":[`) {
		t.Errorf("expected stages in workflow payload, got: %s", text)
	}
	if result.Metadata == nil {
		t.Fatal("expected workflow metadata")
	}
	if _, ok := result.Metadata["trace_id"].(string); !ok {
		t.Fatalf("metadata.trace_id missing or wrong type: %#v", result.Metadata["trace_id"])
	}
	if _, ok := result.Metadata["workflow_trace"].(map[string]any); !ok {
		t.Fatalf("metadata.workflow_trace missing or wrong type: %#v", result.Metadata["workflow_trace"])
	}
}

func TestWorkflowResult_Failure(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	trace := []WorkflowStep{
		{Action: "step1", Status: "success", TimingMs: 10},
		{Action: "step2", Status: "error", TimingMs: 5},
	}
	errResp := mcp.JSONRPCResponse{JSONRPC: "2.0", Error: &mcp.JSONRPCError{Code: -1, Message: "fail"}}
	resp := WorkflowResult(req, "test_workflow", trace, errResp, time.Now())

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true for failed workflow")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, `"status":"failed"`) {
		t.Errorf("expected status=failed, got: %s", text)
	}
	if !strings.Contains(text, `"successful":1`) {
		t.Errorf("expected successful=1, got: %s", text)
	}
	if !strings.Contains(text, `"error_detail"`) {
		t.Errorf("expected error_detail in failed workflow, got: %s", text)
	}
	if !strings.Contains(text, `"status":"failed"`) || !strings.Contains(text, `"workflow_trace"`) {
		t.Errorf("expected normalized workflow trace fields, got: %s", text)
	}
}

func TestWorkflowResult_FailureWithMCPError(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	trace := []WorkflowStep{
		{Action: "step1", Status: "error", TimingMs: 5},
	}

	mcpErr := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: "pilot disabled"}},
		IsError: true,
	}
	raw, _ := json.Marshal(mcpErr)
	errResp := mcp.JSONRPCResponse{JSONRPC: "2.0", Result: raw}
	resp := WorkflowResult(req, "test_workflow", trace, errResp, time.Now())

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "pilot disabled") {
		t.Errorf("expected error detail from MCP error, got: %s", text)
	}
}

func TestBuildWorkflowTraceEnvelope_StagesContainTimingAndStatus(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.March, 3, 12, 0, 0, 0, time.UTC)
	end := start.Add(35 * time.Millisecond)
	trace := []WorkflowStep{
		{Action: "click", Status: "success", TimingMs: 10, CorrelationID: "dom_click_1"},
		{Action: "wait_for_stable", Status: "error", TimingMs: 25, Detail: "timed out"},
	}

	envelope := BuildWorkflowTraceEnvelope("navigate_and_document", trace, start, end, "failed")
	if envelope.TraceID == "" {
		t.Fatal("expected non-empty trace_id")
	}
	if len(envelope.Stages) != 2 {
		t.Fatalf("stages len = %d, want 2", len(envelope.Stages))
	}
	if envelope.Stages[0].Stage != "click" || envelope.Stages[0].DurationMs != 10 {
		t.Fatalf("unexpected stage[0]: %+v", envelope.Stages[0])
	}
	if envelope.Stages[1].Stage != "wait_for_stable" || envelope.Stages[1].Error != "timed out" {
		t.Fatalf("unexpected stage[1]: %+v", envelope.Stages[1])
	}
	if envelope.Status != "failed" {
		t.Fatalf("envelope status = %q, want failed", envelope.Status)
	}
}
