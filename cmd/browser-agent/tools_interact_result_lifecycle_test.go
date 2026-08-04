// tools_interact_result_lifecycle_test.go — Tests async result lifecycle and correlation.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// Issue #92: queued/final markers on async responses
// ============================================

func TestQueuedResponse_HasQueuedAndFinalMarkers(t *testing.T) {
	env := newInteractTestEnv(t)
	capturefixture.SetPilot(env.capture, true)

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
	capturefixture.SetPilot(env.capture, true)

	// Queue async to avoid sync-wait-for-extension
	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	var resultData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	corrID := resultData["correlation_id"].(string)

	env.capture.Queries().ApplyCommandResult(corrID, "complete", json.RawMessage(`{"success":true}`), "")

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
	capturefixture.SetPilot(env.capture, true)

	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	var resultData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	corrID := resultData["correlation_id"].(string)

	env.capture.Queries().ApplyCommandResult(corrID, "complete", nil, "element_not_found")

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
	capturefixture.SetPilot(env.capture, true)

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
	env.capture.Queries().ApplyCommandResult(corrID, "complete", extensionResult, "")

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
	capturefixture.SetPilot(env.capture, true)

	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	var resultData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	corrID := resultData["correlation_id"].(string)

	env.capture.Queries().ApplyCommandResult(corrID, "complete", json.RawMessage(`{"success":true}`), "")

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
	pq := env.capture.Queries().GetLastPendingQuery()
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
