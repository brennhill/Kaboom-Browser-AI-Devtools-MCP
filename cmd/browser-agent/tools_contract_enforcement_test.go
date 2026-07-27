// Purpose: Tests for tool contract enforcement and validation.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_contract_enforcement_test.go — Contract enforcement tests, async bridge round-trip,
// pilot-disabled bad paths, and shared observe helpers.
//
// Run: go test ./cmd/browser-agent -run "TestContract" -v
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
// Bad Path Contracts — Pilot Disabled
// ============================================

func TestContractBadPath_Interact_Navigate_PilotDisabled(t *testing.T) {
	env := newInteractContractEnv(t)
	// Pilot is disabled by default in test env
	result, ok := env.callInteract(t, `{"what":"navigate","url":"https://example.com"}`)
	if !ok {
		t.Fatal("interact navigate pilot disabled: no result")
	}
	assertStructuredErrorCode(t, "navigate (pilot disabled)", result, "pilot_disabled")
}

func TestContractBadPath_Interact_ExecuteJS_PilotDisabled(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"what":"execute_js","script":"1+1"}`)
	if !ok {
		t.Fatal("interact execute_js pilot disabled: no result")
	}
	assertStructuredErrorCode(t, "execute_js (pilot disabled)", result, "pilot_disabled")
}

func TestContractBadPath_Interact_Refresh_PilotDisabled(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"what":"refresh"}`)
	if !ok {
		t.Fatal("interact refresh pilot disabled: no result")
	}
	assertStructuredErrorCode(t, "refresh (pilot disabled)", result, "pilot_disabled")
}

func TestContractBadPath_Interact_Back_PilotDisabled(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"what":"back"}`)
	if !ok {
		t.Fatal("interact back pilot disabled: no result")
	}
	assertStructuredErrorCode(t, "back (pilot disabled)", result, "pilot_disabled")
}

func TestContractBadPath_Interact_NewTab_PilotDisabled(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"what":"new_tab","url":"https://example.com"}`)
	if !ok {
		t.Fatal("interact new_tab pilot disabled: no result")
	}
	assertStructuredErrorCode(t, "new_tab (pilot disabled)", result, "pilot_disabled")
}

func TestContractBadPath_Interact_Forward_PilotDisabled(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"what":"forward"}`)
	if !ok {
		t.Fatal("interact forward pilot disabled: no result")
	}
	assertStructuredErrorCode(t, "forward (pilot disabled)", result, "pilot_disabled")
}

func TestContractBadPath_Interact_Highlight_PilotDisabled(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"what":"highlight","selector":"#test"}`)
	if !ok {
		t.Fatal("interact highlight pilot disabled: no result")
	}
	assertStructuredErrorCode(t, "highlight (pilot disabled)", result, "pilot_disabled")
}

// ============================================
// Bad Path Contracts — No Data
// ============================================

func TestContractBadPath_Interact_LoadState_NotFound(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"what":"load_state","snapshot_name":"nonexistent_state_xyz"}`)
	if !ok {
		t.Fatal("interact load_state not found: no result")
	}
	assertStructuredErrorCode(t, "load_state (not found)", result, "no_data")
}

func TestContractBadPath_Observe_CommandResult_NotFound(t *testing.T) {
	s := newScenario(t)
	result, ok := s.callObserveWithArgs(t, `{"what":"command_result","correlation_id":"nonexistent_corr_id"}`)
	if !ok {
		t.Fatal("observe command_result not found: no result")
	}
	assertStructuredErrorCode(t, "command_result (not found)", result, "no_data")
}

// ============================================
// Contract Enforcement: Retryable Field
// ============================================

func TestContractEnforcement_ErrorsHaveRetryableField(t *testing.T) {
	// Verify that all error responses from mcp.StructuredErrorResponse include the retryable field.
	// This ensures the LLM always knows whether an error is worth retrying.
	testCases := []struct {
		code    string
		message string
		retry   string
	}{
		{mcp.ErrInvalidJSON, "bad json", "Fix JSON"},
		{mcp.ErrMissingParam, "missing what", "Add 'what'"},
		{mcp.ErrInvalidParam, "bad param", "Fix param"},
		{mcp.ErrUnknownMode, "unknown mode", "Use valid mode"},
		{mcp.ErrExtTimeout, "timeout", "Retry later"},
		{mcp.ErrExtError, "error", "Retry later"},
		{mcp.ErrInternal, "internal", "Do not retry"},
		{mcp.ErrNoData, "no data", "Check state"},
		{mcp.ErrRateLimited, "rate limited", "Wait"},
		{mcp.ErrNotInitialized, "not init", "Initialize first"},
		{mcp.ErrCursorExpired, "cursor expired", "Restart"},
		{mcp.ErrMarshalFailed, "marshal failed", "Do not retry"},
	}

	for _, tc := range testCases {
		t.Run(tc.code, func(t *testing.T) {
			raw := mcp.StructuredErrorResponse(tc.code, tc.message, tc.retry)
			var result MCPToolResult
			if err := json.Unmarshal(raw, &result); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			jsonText := extractJSONFromText(result.Content[0].Text)
			var data map[string]any
			if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
				t.Fatalf("failed to parse structured error: %v", err)
			}

			if _, exists := data["retryable"]; !exists {
				t.Errorf("error code %q missing 'retryable' field", tc.code)
			}
			if code, _ := data["error_code"].(string); code == "" {
				t.Errorf("error code %q missing canonical 'error_code' field", tc.code)
			}
			if playbook, _ := data["recovery_playbook"].(string); playbook == "" {
				t.Errorf("error code %q missing canonical 'recovery_playbook' field", tc.code)
			}
		})
	}
}

// ============================================
// Contract Enforcement: elapsed_ms in command_result
// ============================================

func TestContractEnforcement_CommandResult_HasElapsedMs(t *testing.T) {
	s := newScenario(t)

	// Create and complete a command
	queryID, _ := s.capture.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:          "dom",
			Params:        json.RawMessage(`{"selector":"body"}`),
			CorrelationID: "elapsed-test-123",
		},
		5*time.Second,
		"",
	)
	time.Sleep(10 * time.Millisecond) // small delay to ensure non-zero elapsed
	s.capture.SetQueryResult(queryID, json.RawMessage(`{"html":"<body/>"}`))

	result, ok := s.callObserveWithArgs(t, `{"what":"command_result","correlation_id":"elapsed-test-123"}`)
	if !ok {
		t.Fatal("command_result: no result")
	}

	data := parseResponseJSON(t, result)
	if _, exists := data["elapsed_ms"]; !exists {
		t.Error("command_result response missing 'elapsed_ms' field")
	}
	if elapsed, ok := data["elapsed_ms"].(float64); ok && elapsed <= 0 {
		t.Errorf("elapsed_ms should be > 0, got %v", elapsed)
	}
}

// ============================================
// Contract Enforcement: Unknown Params Produce Warnings
// ============================================

func TestContractEnforcement_UnknownParams_ProduceWarnings(t *testing.T) {
	// Test each tool with an unknown parameter via HandleToolCall
	h, _, _ := makeToolHandler(t)

	tools := []struct {
		name string
		args string
	}{
		{"observe", `{"what":"errors","totally_fake_param_xyz":true}`},
		{"configure", `{"what":"health","totally_fake_param_xyz":true}`},
		{"generate", `{"what":"test","totally_fake_param_xyz":true}`},
		{"analyze", `{"what":"dom","selector":"body","totally_fake_param_xyz":true}`},
		{"interact", `{"what":"list_states","totally_fake_param_xyz":true}`},
	}

	for _, tc := range tools {
		t.Run(tc.name, func(t *testing.T) {
			req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
			resp, handled := h.HandleToolCall(req, tc.name, json.RawMessage(tc.args))
			if !handled {
				t.Fatalf("%s: not handled", tc.name)
			}

			var result MCPToolResult
			if err := json.Unmarshal(resp.Result, &result); err != nil {
				t.Fatalf("%s: failed to unmarshal: %v", tc.name, err)
			}

			// Skip error responses — they skip validation
			if result.IsError {
				t.Skipf("%s: returned error (expected for some tools without extension)", tc.name)
			}

			// Look for warnings in content blocks
			foundWarning := false
			for _, block := range result.Content {
				if strings.Contains(block.Text, "unknown parameter") && strings.Contains(block.Text, "totally_fake_param_xyz") {
					foundWarning = true
					break
				}
			}
			if !foundWarning {
				t.Errorf("%s: expected warning about unknown parameter 'totally_fake_param_xyz' in response content blocks", tc.name)
			}
		})
	}
}

// ============================================
// Helpers
// ============================================

// assertNonErrorResponse and firstText are in tools_test_helpers_test.go.

// callObserveWithArgs is a helper to call observe with custom JSON args.
func (s *scenario) callObserveWithArgs(t *testing.T, argsJSON string) (MCPToolResult, bool) {
	t.Helper()
	args := json.RawMessage(argsJSON)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := s.handler.toolObserve(req, args)
	if resp.Result == nil {
		return MCPToolResult{}, false
	}
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	return result, true
}

// ============================================
// Pending Query Timeout Test
// ============================================

// TestContractPendingQuery_Timeout verifies that a pending query that is never
// fulfilled returns a timeout error rather than hanging forever.
func TestContractPendingQuery_Timeout(t *testing.T) {
	s := newScenario(t)

	// Create a pending query with a very short timeout
	queryID, _ := s.capture.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "test_query",
			Params: json.RawMessage(`{"test": true}`),
		},
		100*time.Millisecond, // Very short timeout
		"",
	)

	// Wait for result — nobody will fulfill this query
	start := time.Now()
	_, err := s.capture.WaitForResult(queryID, 100*time.Millisecond)
	elapsed := time.Since(start)

	// Must return an error (not hang)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	// Must complete within a reasonable time (not hang for 5s+)
	if elapsed > 1*time.Second {
		t.Errorf("timeout took too long: %v (expected ~100ms)", elapsed)
	}
}

// ============================================
// Async Bridge Round-Trip Test
// ============================================

// TestContractAsyncBridge_RoundTrip verifies the full pending query lifecycle:
// create query → retrieve via GetPendingQueries → deliver result → WaitForResult returns.
func TestContractAsyncBridge_RoundTrip(t *testing.T) {
	s := newScenario(t)

	// 1. Create a pending query
	queryID, _ := s.capture.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:          "dom",
			Params:        json.RawMessage(`{"selector": "#test"}`),
			CorrelationID: "test-corr-123",
		},
		5*time.Second,
		"",
	)

	if queryID == "" {
		t.Fatal("CreatePendingQueryWithTimeout returned empty ID")
	}

	// 2. Verify the query appears in GetPendingQueries (simulates extension poll)
	pending := s.capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("GetPendingQueries returned 0 queries after creating one")
	}

	found := false
	for _, pq := range pending {
		if pq.ID == queryID {
			found = true
			if pq.Type != "dom" {
				t.Errorf("pending query type: got %q, want %q", pq.Type, "dom")
			}
			if pq.CorrelationID != "test-corr-123" {
				t.Errorf("pending query correlation_id: got %q, want %q", pq.CorrelationID, "test-corr-123")
			}
			break
		}
	}
	if !found {
		t.Fatalf("query %s not found in GetPendingQueries result", queryID)
	}

	// 3. Deliver result (simulates extension POST /dom-result)
	resultPayload := json.RawMessage(`{"innerHTML": "<div>test</div>"}`)
	s.capture.SetQueryResult(queryID, resultPayload)

	// 4. WaitForResult should return immediately with the result
	start := time.Now()
	result, err := s.capture.WaitForResult(queryID, 1*time.Second)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("WaitForResult returned error: %v", err)
	}

	// Should be fast (result already available)
	if elapsed > 500*time.Millisecond {
		t.Errorf("WaitForResult took too long: %v (result was already delivered)", elapsed)
	}

	// Verify the result content
	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if _, ok := parsed["innerHTML"]; !ok {
		t.Error("result missing 'innerHTML' field")
	}

	// 5. Query should be consumed (not in pending anymore)
	pendingAfter := s.capture.GetPendingQueries()
	for _, pq := range pendingAfter {
		if pq.ID == queryID {
			t.Error("query still in pending list after result was delivered")
		}
	}
}

// TestContractAsyncBridge_ConcurrentDelivery verifies that delivering a result
// while WaitForResult is blocking correctly wakes the waiter.
func TestContractAsyncBridge_ConcurrentDelivery(t *testing.T) {
	s := newScenario(t)

	queryID, _ := s.capture.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "execute",
			Params: json.RawMessage(`{"script": "1+1"}`),
		},
		5*time.Second,
		"",
	)

	// Start waiting in a goroutine
	type waitResult struct {
		data json.RawMessage
		err  error
	}
	ch := make(chan waitResult, 1)
	go func() {
		data, err := s.capture.WaitForResult(queryID, 5*time.Second)
		ch <- waitResult{data, err}
	}()

	// Give the goroutine time to start waiting
	time.Sleep(50 * time.Millisecond)

	// Deliver the result (simulates extension)
	s.capture.SetQueryResult(queryID, json.RawMessage(`{"value": 2}`))

	// Wait for the goroutine to complete
	select {
	case wr := <-ch:
		if wr.err != nil {
			t.Fatalf("WaitForResult returned error: %v", wr.err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(wr.data, &parsed); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}
		if v, _ := parsed["value"].(float64); v != 2 {
			t.Errorf("result value: got %v, want 2", v)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForResult did not return within 2s after result delivery")
	}
}
