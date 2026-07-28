// Purpose: Tests for interact rich command-result formatting.
// Docs: docs/features/feature/interact-explore/index.md

// tools_interact_rich_cmdresult_test.go — TDD tests for Rich Action command result handling.
// Tests failed command visibility (IsError signaling), queued/final markers,
// effective context surfacing, subtitle correlation IDs, and diagnostic hints.
//
// Run: go test ./cmd/browser-agent -run "TestCommandResult|TestQueuedResponse|TestSubtitle" -v
package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// ============================================
// Failed command visibility: IsError signaling
// ============================================

func TestCommandResult_ExpiredSetsIsError(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Queue a command
	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID

	// Simulate expiry — extension never responded
	env.capture.ExpireCommand(corrID)

	// Observe the expired command
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if !observeResult.IsError {
		t.Error("Expired command MUST set IsError=true so LLMs recognize failure")
	}

	text := observeResult.Content[0].Text
	if !strings.Contains(text, "extension_timeout") {
		t.Errorf("Expired command should include error code 'extension_timeout', got: %s", text)
	}
	if !strings.Contains(text, "retry") {
		t.Errorf("Expired command should include retry instructions, got: %s", text)
	}
}

func TestCommandResult_CompleteWithErrorSetsIsError(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Queue a command
	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID

	// Simulate extension completing with an error
	env.capture.ApplyCommandResult(corrID, "complete", json.RawMessage(`null`), "Element not found: #btn")

	// Observe the failed command
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if !observeResult.IsError {
		t.Error("Command completed with error MUST set IsError=true so LLMs recognize failure")
	}

	text := observeResult.Content[0].Text
	if !strings.Contains(text, "FAILED") {
		t.Errorf("Failed command summary should include 'FAILED', got: %s", text)
	}
	if !strings.Contains(text, "Element not found") {
		t.Errorf("Failed command should include error message, got: %s", text)
	}
}

func TestCommandResult_EmbeddedFailureSetsIsError(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID

	// Extension reported failure inside the result payload without setting command error.
	env.capture.ApplyCommandResult(corrID, "complete", json.RawMessage(`{"success":false,"error":"selector_not_found","message":"#btn not found"}`), "")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if !observeResult.IsError {
		t.Fatal("Embedded success=false MUST set IsError=true")
	}

	text := observeResult.Content[0].Text
	if !strings.Contains(text, "FAILED") {
		t.Fatalf("Expected FAILED summary, got: %s", text)
	}
	if !strings.Contains(text, "selector_not_found") {
		t.Fatalf("Expected embedded error to surface, got: %s", text)
	}
}

func TestCommandResult_EmbeddedCSPFailureAddsCSPMarkers(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"execute_js","script":"(() => 1)()","background":true}`)
	if !ok || result.IsError {
		t.Fatal("execute_js should queue successfully")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID

	env.capture.ApplyCommandResult(corrID, "complete", json.RawMessage(`{"success":false,"error":"csp_blocked_all_worlds","message":"Page CSP blocks dynamic script execution"}`), "")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}
	if !observeResult.IsError {
		t.Fatal("CSP failure must set IsError=true")
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}
	if blocked, _ := responseData["csp_blocked"].(bool); !blocked {
		t.Fatalf("csp_blocked = %v, want true", responseData["csp_blocked"])
	}
	if responseData["failure_cause"] != "csp" {
		t.Fatalf("failure_cause = %v, want csp", responseData["failure_cause"])
	}
}

func TestCommandResult_ErrorStatusCSPFailureIncludesRetryHint(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"navigate","url":"https://example.com","background":true}`)
	if !ok || result.IsError {
		t.Fatal("navigate should queue successfully")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID
	env.capture.ApplyCommandResult(corrID, "error", json.RawMessage(`{"success":false,"error":"csp_blocked_page","message":"This page blocks extension script execution.","csp_blocked":true,"failure_cause":"csp"}`), "csp_blocked_page")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}
	if !observeResult.IsError {
		t.Fatal("CSP error status must set IsError=true")
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}
	if blocked, _ := responseData["csp_blocked"].(bool); !blocked {
		t.Fatalf("csp_blocked = %v, want true", responseData["csp_blocked"])
	}
	if responseData["failure_cause"] != "csp" {
		t.Fatalf("failure_cause = %v, want csp", responseData["failure_cause"])
	}
	retry, _ := responseData["retry"].(string)
	if !strings.Contains(strings.ToLower(retry), "navigate") {
		t.Fatalf("retry hint should include navigation guidance, got: %q", retry)
	}
}

func TestCommandResult_InteractFailureCodesIncludeRecoveryRetryGuidance(t *testing.T) {
	cases := []struct {
		name          string
		errorCode     string
		resultJSON    string
		retryMustHave []string
	}{
		{
			name:       "element_not_found",
			errorCode:  "element_not_found",
			resultJSON: `{"success":false,"error":"element_not_found","message":"No element matches selector: text=Submit"}`,
			retryMustHave: []string{
				"list_interactive",
				"scope",
			},
		},
		{
			name:       "ambiguous_target",
			errorCode:  "ambiguous_target",
			resultJSON: `{"success":false,"error":"ambiguous_target","message":"Selector matches multiple viable elements"}`,
			retryMustHave: []string{
				"candidates",
				"element_id",
			},
		},
		{
			name:       "stale_element_id",
			errorCode:  "stale_element_id",
			resultJSON: `{"success":false,"error":"stale_element_id","message":"Element handle is stale or unknown"}`,
			retryMustHave: []string{
				"list_interactive",
				"element_id",
			},
		},
		{
			name:       "scope_not_found",
			errorCode:  "scope_not_found",
			resultJSON: `{"success":false,"error":"scope_not_found","message":"No scope element matches selector: #missing"}`,
			retryMustHave: []string{
				"scope_selector",
				"scope_rect",
				"frame",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			env := newInteractTestEnv(t)
			env.capture.SetPilotEnabled(true)

			result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
			if !ok || result.IsError {
				t.Fatal("click should queue successfully")
			}

			pq := env.capture.GetLastPendingQuery()
			corrID := pq.CorrelationID
			env.capture.ApplyCommandResult(corrID, "error", json.RawMessage(tc.resultJSON), tc.errorCode)

			req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
			args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
			resp := env.handler.observeDispatcher.CommandResult(req, args)

			var observeResult mcp.MCPToolResult
			if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
				t.Fatalf("Failed to parse result: %v", err)
			}
			if !observeResult.IsError {
				t.Fatal("interact failure should set IsError=true")
			}

			var responseData map[string]any
			if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
				t.Fatalf("Failed to parse response JSON: %v", err)
			}
			retry, _ := responseData["retry"].(string)
			if retry == "" {
				t.Fatalf("retry guidance is missing for %s", tc.errorCode)
			}
			retryLower := strings.ToLower(retry)
			for _, required := range tc.retryMustHave {
				if !strings.Contains(retryLower, strings.ToLower(required)) {
					t.Fatalf("retry guidance %q missing token %q for %s", retry, required, tc.errorCode)
				}
			}
		})
	}
}

func TestCommandResult_SuccessDoesNotSetIsError(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Queue a command
	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID

	// Simulate successful completion
	env.capture.ApplyCommandResult(corrID, "complete", json.RawMessage(`{"success":true}`), "")

	// Observe the successful command
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if observeResult.IsError {
		t.Error("Successful command should NOT set IsError")
	}

	text := observeResult.Content[0].Text
	if strings.Contains(text, "FAILED") {
		t.Errorf("Successful command should not contain 'FAILED', got: %s", text)
	}
}

// ============================================
// Issue #92: queued/final markers on async responses
// ============================================

func TestQueuedResponse_HasQueuedAndFinalMarkers(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if responseData["status"] != "queued" {
		t.Fatalf("status = %v, want queued", responseData["status"])
	}
	if queued, _ := responseData["queued"].(bool); !queued {
		t.Fatalf("queued response should have queued=true, got %v", responseData["queued"])
	}
	if final, _ := responseData["final"].(bool); final {
		t.Fatalf("queued response should have final=false, got %v", responseData["final"])
	}
}

func TestCommandResult_CompleteHasFinalTrue(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Queue async to avoid sync-wait-for-extension
	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	var resultData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	corrID := resultData["correlation_id"].(string)

	env.capture.ApplyCommandResult(corrID, "complete", json.RawMessage(`{"success":true}`), "")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	finalVal, ok := responseData["final"].(bool)
	if !ok || !finalVal {
		t.Fatalf("complete command should have final=true, got %v", responseData["final"])
	}
}

func TestCommandResult_ErrorHasFinalTrue(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	var resultData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	corrID := resultData["correlation_id"].(string)

	env.capture.ApplyCommandResult(corrID, "complete", nil, "element_not_found")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	finalVal, ok := responseData["final"].(bool)
	if !ok || !finalVal {
		t.Fatalf("error command should have final=true, got %v", responseData["final"])
	}
}

// ============================================
// Issue #91: effective_tab_id and effective_url surfaced
// ============================================

func TestCommandResult_EffectiveContextSurfaced(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","tab_id":42,"background":true}`)
	var resultData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	corrID := resultData["correlation_id"].(string)

	extensionResult := json.RawMessage(`{
		"success": true,
		"action": "click",
		"resolved_tab_id": 42,
		"resolved_url": "https://example.com/page1",
		"effective_tab_id": 42,
		"effective_url": "https://example.com/page2"
	}`)
	env.capture.ApplyCommandResult(corrID, "complete", extensionResult, "")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	if responseData["effective_tab_id"] != float64(42) {
		t.Fatalf("effective_tab_id = %v, want 42", responseData["effective_tab_id"])
	}
	if responseData["effective_url"] != "https://example.com/page2" {
		t.Fatalf("effective_url = %v, want https://example.com/page2", responseData["effective_url"])
	}
	if responseData["resolved_url"] != "https://example.com/page1" {
		t.Fatalf("resolved_url = %v, want https://example.com/page1", responseData["resolved_url"])
	}
}

// ============================================
// Issue #92 follow-up: queued=false on non-queued responses
// ============================================

func TestCommandResult_CompleteHasQueuedFalse(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	var resultData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	corrID := resultData["correlation_id"].(string)

	env.capture.ApplyCommandResult(corrID, "complete", json.RawMessage(`{"success":true}`), "")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	if queued, ok := responseData["queued"].(bool); !ok || queued {
		t.Fatalf("complete command should have queued=false, got %v", responseData["queued"])
	}
}

// ============================================
// Issue #92 follow-up: final=true on expired/timeout
// ============================================

func TestCommandResult_ExpiredHasFinalTrue(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	var resultData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	corrID := resultData["correlation_id"].(string)

	env.capture.ExpireCommand(corrID)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	finalVal, ok := responseData["final"].(bool)
	if !ok || !finalVal {
		t.Fatalf("expired command should have final=true, got %v", responseData["final"])
	}
	if queued, ok := responseData["queued"].(bool); !ok || queued {
		t.Fatalf("expired command should have queued=false, got %v", responseData["queued"])
	}
	if responseData["error"] != mcp.ErrExtTimeout {
		t.Fatalf("expired command should have error=%s, got %v", mcp.ErrExtTimeout, responseData["error"])
	}
}

func TestCommandResult_TimeoutHasFinalTrue(t *testing.T) {
	env := newInteractTestEnv(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	cmd := queries.CommandResult{
		CorrelationID: "timeout_cmd_123",
		Status:        "timeout",
		Error:         "extension did not respond",
		CreatedAt:     time.Now().Add(-2 * time.Second),
	}
	resp := env.handler.formatCommandResult(req, cmd, cmd.CorrelationID)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	finalVal, ok := responseData["final"].(bool)
	if !ok || !finalVal {
		t.Fatalf("timeout command should have final=true, got %v", responseData["final"])
	}
	if queued, ok := responseData["queued"].(bool); !ok || queued {
		t.Fatalf("timeout command should have queued=false, got %v", responseData["queued"])
	}
	if responseData["error"] != mcp.ErrExtTimeout {
		t.Fatalf("timeout command should have error=%s, got %v", mcp.ErrExtTimeout, responseData["error"])
	}
}

func TestCommandResult_NotFoundHasFinalTrue(t *testing.T) {
	env := newInteractTestEnv(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"missing_corr_123"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}
	if !observeResult.IsError {
		t.Fatal("missing command should return isError=true")
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}
	if responseData["error_code"] != mcp.ErrNoData {
		t.Fatalf("missing command should return error_code=%s, got %v", mcp.ErrNoData, responseData["error_code"])
	}
	if finalVal, ok := responseData["final"].(bool); !ok || !finalVal {
		t.Fatalf("missing command should have final=true, got %v", responseData["final"])
	}
}

func TestCommandResult_AnnotationNotFoundHasFinalTrue(t *testing.T) {
	env := newInteractTestEnv(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"ann_missing_123"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}
	if !observeResult.IsError {
		t.Fatal("missing annotation command should return isError=true")
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}
	if responseData["error_code"] != mcp.ErrNoData {
		t.Fatalf("missing annotation command should return error_code=%s, got %v", mcp.ErrNoData, responseData["error_code"])
	}
	if finalVal, ok := responseData["final"].(bool); !ok || !finalVal {
		t.Fatalf("missing annotation command should have final=true, got %v", responseData["final"])
	}
}

func TestCommandResult_ExpiredIncludesDiagnosticHint(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID
	env.capture.ExpireCommand(corrID)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	text := observeResult.Content[0].Text

	// Diagnostic hint must include pilot and tracking state
	if !strings.Contains(text, "pilot=") {
		t.Errorf("Error response must include pilot status in diagnostic hint, got: %s", text)
	}
	if !strings.Contains(text, "tracked_tab=") {
		t.Errorf("Error response must include tracking status in diagnostic hint, got: %s", text)
	}
}

// ============================================
// Subtitle: correlation_id contract
// ============================================
// handleSubtitle() creates a PendingQuery with a correlationID but never
// returns it in the MCP response. This makes it impossible for callers to
// poll observe(command_result) for completion, causing race conditions in
// smoke tests (test 11.1: "Subtitle still visible after clear").

func TestSubtitle_SetResponse_HasCorrelationID(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"subtitle","text":"hello world"}`)
	if !ok {
		t.Fatal("subtitle set should return a result")
	}
	if result.IsError {
		t.Fatal("subtitle set should not be an error")
	}
	if len(result.Content) == 0 {
		t.Fatal("No content in result")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "correlation_id") {
		t.Errorf("Subtitle SET response must contain correlation_id so callers can poll for completion.\n"+
			"Without it, callers cannot wait for the extension to process the command.\n"+
			"Every other async handler (click, navigate, execute_js, highlight) returns correlation_id.\n"+
			"Got: %s", text)
	}
	if !strings.Contains(text, "subtitle_") {
		t.Errorf("Subtitle correlation_id should have subtitle_ prefix. Got: %s", text)
	}
}

func TestSubtitle_ClearResponse_HasCorrelationID(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"subtitle","text":""}`)
	if !ok {
		t.Fatal("subtitle clear should return a result")
	}
	if result.IsError {
		t.Fatal("subtitle clear should not be an error")
	}
	if len(result.Content) == 0 {
		t.Fatal("No content in result")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "correlation_id") {
		t.Errorf("Subtitle CLEAR response must contain correlation_id so callers can poll for completion.\n"+
			"Without it, the smoke test's interact_and_wait returns immediately and checks DOM\n"+
			"before the extension has processed the clear — causing 'still visible after clear'.\n"+
			"Got: %s", text)
	}
}

func TestSubtitle_CorrelationID_MatchesPendingQuery(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"subtitle","text":"test"}`)
	if !ok || result.IsError {
		t.Fatal("subtitle should succeed")
	}

	// The PendingQuery IS created with a correlationID
	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("No pending query created for subtitle")
	}
	if pq.CorrelationID == "" {
		t.Fatal("PendingQuery has empty correlationID")
	}

	// But the MCP response must also contain it
	text := result.Content[0].Text
	if !strings.Contains(text, pq.CorrelationID) {
		t.Errorf("MCP response must contain the same correlation_id as the PendingQuery.\n"+
			"PendingQuery has: %s\n"+
			"Response text: %s", pq.CorrelationID, text)
	}
}

// ============================================
// ambiguous_target: candidates + suggested_element_id surfaced
// ============================================

func TestCommandResult_AmbiguousTarget_CandidatesPromotedToTopLevel(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"click","selector":"text=About","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should queue successfully")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID

	// Simulate extension returning ambiguous_target with candidates
	ambiguousResult := `{
		"success": false,
		"action": "click",
		"selector": "text=About",
		"error": "ambiguous_target",
		"message": "Selector matches multiple viable elements: text=About",
		"match_count": 3,
		"match_strategy": "ambiguous_selector",
		"candidates": [
			{"tag":"a","text_preview":"About Us","selector":"a.nav-about","element_id":"el_1","visible":true,"bbox":{"x":100,"y":20,"width":60,"height":24}},
			{"tag":"a","text_preview":"About","selector":"a.footer-about","element_id":"el_2","visible":true,"bbox":{"x":100,"y":900,"width":50,"height":24}},
			{"tag":"a","text_preview":"About Our Team","selector":"a.team-about","element_id":"el_3","visible":false,"bbox":{"x":0,"y":0,"width":0,"height":0}}
		]
	}`
	env.capture.ApplyCommandResult(corrID, "error", json.RawMessage(ambiguousResult), "ambiguous_target")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}
	if !observeResult.IsError {
		t.Fatal("ambiguous_target should set IsError=true")
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// candidates must be promoted to top-level
	candidates, ok := responseData["candidates"].([]any)
	if !ok || len(candidates) == 0 {
		t.Fatalf("candidates must be promoted to top-level for LLM recovery, got: %v", responseData["candidates"])
	}
	if len(candidates) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(candidates))
	}

	// match_count must be promoted
	matchCount, ok := responseData["match_count"].(float64)
	if !ok || matchCount != 3 {
		t.Fatalf("match_count must be promoted to top-level, got: %v", responseData["match_count"])
	}
}

func TestCommandResult_AmbiguousTarget_SuggestedElementID(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"click","selector":"text=Submit","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should queue successfully")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID

	// First candidate is NOT visible, second is visible — suggested should be second
	ambiguousResult := `{
		"success": false,
		"action": "click",
		"selector": "text=Submit",
		"error": "ambiguous_target",
		"match_count": 2,
		"candidates": [
			{"tag":"button","text_preview":"Submit","element_id":"el_hidden","visible":false},
			{"tag":"button","text_preview":"Submit Form","element_id":"el_visible","visible":true}
		]
	}`
	env.capture.ApplyCommandResult(corrID, "error", json.RawMessage(ambiguousResult), "ambiguous_target")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	suggested, ok := responseData["suggested_element_id"].(string)
	if !ok || suggested == "" {
		t.Fatalf("suggested_element_id must be set to the first visible candidate's element_id, got: %v", responseData["suggested_element_id"])
	}
	if suggested != "el_visible" {
		t.Fatalf("suggested_element_id = %q, want el_visible (first VISIBLE candidate)", suggested)
	}
}

func TestCommandResult_AmbiguousTarget_RetryGuidanceMentionsCandidates(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"click","selector":"text=OK","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should queue successfully")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID

	ambiguousResult := `{
		"success": false,
		"error": "ambiguous_target",
		"candidates": [
			{"element_id":"el_1","visible":true},
			{"element_id":"el_2","visible":true}
		]
	}`
	env.capture.ApplyCommandResult(corrID, "error", json.RawMessage(ambiguousResult), "ambiguous_target")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	retry, _ := responseData["retry"].(string)
	retryLower := strings.ToLower(retry)

	// Retry guidance must tell LLM to pick from candidates directly (no extra round-trip)
	if !strings.Contains(retryLower, "candidates") {
		t.Fatalf("retry guidance must mention 'candidates' array, got: %q", retry)
	}
	if !strings.Contains(retryLower, "element_id") {
		t.Fatalf("retry guidance must mention element_id, got: %q", retry)
	}
	if !strings.Contains(retryLower, "suggested_element_id") {
		t.Fatalf("retry guidance must mention suggested_element_id, got: %q", retry)
	}
}

func TestCommandResult_AmbiguousTarget_NoCandidates_NoSuggestedElementID(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"click","selector":"text=OK","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should queue successfully")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID

	// ambiguous_target with no candidates array (edge case)
	ambiguousResult := `{"success":false,"error":"ambiguous_target","match_count":2}`
	env.capture.ApplyCommandResult(corrID, "error", json.RawMessage(ambiguousResult), "ambiguous_target")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.observeDispatcher.CommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// Should NOT have suggested_element_id when no candidates
	if _, exists := responseData["suggested_element_id"]; exists {
		t.Fatalf("suggested_element_id should not be set when no candidates present")
	}
}

func TestCommandResult_PilotDisabledIncludesDiagnosticHint(t *testing.T) {
	env := newInteractTestEnv(t)
	// Pilot is disabled by default in test env

	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	if !ok {
		t.Fatal("should return result")
	}
	if !result.IsError {
		t.Fatal("pilot disabled should return error")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "pilot=DISABLED") {
		t.Errorf("pilot_disabled error must include 'pilot=DISABLED' in hint, got: %s", text)
	}
	if !strings.Contains(text, "tracked_tab=") {
		t.Errorf("pilot_disabled error must include tracking status in hint, got: %s", text)
	}
}
