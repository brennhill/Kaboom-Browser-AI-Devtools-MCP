// tools_interact_rich_cmdresult_test.go — Tests interact failures and recovery diagnostics.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// ============================================
// Failed command visibility: IsError signaling
// ============================================

func TestCommandResult_ExpiredSetsIsError(t *testing.T) {
	env := newInteractTestEnv(t)
	capturefixture.SetPilot(env.capture, true)

	// Queue a command
	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed")
	}

	pq := lastPendingQuerySnapshot(env.capture.Queries())
	corrID := pq.CorrelationID

	// Simulate expiry — extension never responded
	env.capture.Queries().ExpireCommand(corrID)

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
	capturefixture.SetPilot(env.capture, true)

	// Queue a command
	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed")
	}

	pq := lastPendingQuerySnapshot(env.capture.Queries())
	corrID := pq.CorrelationID

	// Simulate extension completing with an error
	env.capture.Queries().ApplyCommandResult(corrID, "complete", json.RawMessage(`null`), "Element not found: #btn")

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
	capturefixture.SetPilot(env.capture, true)

	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed")
	}

	pq := lastPendingQuerySnapshot(env.capture.Queries())
	corrID := pq.CorrelationID

	// Extension reported failure inside the result payload without setting command error.
	env.capture.Queries().ApplyCommandResult(corrID, "complete", json.RawMessage(`{"success":false,"error":"selector_not_found","message":"#btn not found"}`), "")

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
	capturefixture.SetPilot(env.capture, true)

	result, ok := env.callInteract(t, `{"what":"execute_js","script":"(() => 1)()","background":true}`)
	if !ok || result.IsError {
		t.Fatal("execute_js should queue successfully")
	}

	pq := lastPendingQuerySnapshot(env.capture.Queries())
	corrID := pq.CorrelationID

	env.capture.Queries().ApplyCommandResult(corrID, "complete", json.RawMessage(`{"success":false,"error":"csp_blocked_all_worlds","message":"Page CSP blocks dynamic script execution"}`), "")

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
	capturefixture.SetPilot(env.capture, true)

	result, ok := env.callInteract(t, `{"what":"navigate","url":"https://example.com","background":true}`)
	if !ok || result.IsError {
		t.Fatal("navigate should queue successfully")
	}

	pq := lastPendingQuerySnapshot(env.capture.Queries())
	corrID := pq.CorrelationID
	env.capture.Queries().ApplyCommandResult(corrID, "error", json.RawMessage(`{"success":false,"error":"csp_blocked_page","message":"This page blocks extension script execution.","csp_blocked":true,"failure_cause":"csp"}`), "csp_blocked_page")

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
			capturefixture.SetPilot(env.capture, true)

			result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
			if !ok || result.IsError {
				t.Fatal("click should queue successfully")
			}

			pq := lastPendingQuerySnapshot(env.capture.Queries())
			corrID := pq.CorrelationID
			env.capture.Queries().ApplyCommandResult(corrID, "error", json.RawMessage(tc.resultJSON), tc.errorCode)

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
	capturefixture.SetPilot(env.capture, true)

	// Queue a command
	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed")
	}

	pq := lastPendingQuerySnapshot(env.capture.Queries())
	corrID := pq.CorrelationID

	// Simulate successful completion
	env.capture.Queries().ApplyCommandResult(corrID, "complete", json.RawMessage(`{"success":true}`), "")

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

func TestCommandResult_ExpiredHasFinalTrue(t *testing.T) {
	env := newInteractTestEnv(t)
	capturefixture.SetPilot(env.capture, true)

	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	var resultData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	corrID := resultData["correlation_id"].(string)

	env.capture.Queries().ExpireCommand(corrID)

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
	resp := env.handler.asyncCommands.FormatCommandResult(req, cmd, cmd.CorrelationID)

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
	capturefixture.SetPilot(env.capture, true)

	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed")
	}

	pq := lastPendingQuerySnapshot(env.capture.Queries())
	corrID := pq.CorrelationID
	env.capture.Queries().ExpireCommand(corrID)

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
