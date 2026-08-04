// tools_interact_result_ambiguity_test.go — Tests ambiguous-target result guidance.
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
// ambiguous_target: candidates + suggested_element_id surfaced
// ============================================

func TestCommandResult_AmbiguousTarget_CandidatesPromotedToTopLevel(t *testing.T) {
	env := newInteractTestEnv(t)
	capturefixture.SetPilot(env.capture, true)

	result, ok := env.callInteract(t, `{"what":"click","selector":"text=About","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should queue successfully")
	}

	pq := env.capture.Queries().GetLastPendingQuery()
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
	env.capture.Queries().ApplyCommandResult(corrID, "error", json.RawMessage(ambiguousResult), "ambiguous_target")

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
	capturefixture.SetPilot(env.capture, true)

	result, ok := env.callInteract(t, `{"what":"click","selector":"text=Submit","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should queue successfully")
	}

	pq := env.capture.Queries().GetLastPendingQuery()
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
	env.capture.Queries().ApplyCommandResult(corrID, "error", json.RawMessage(ambiguousResult), "ambiguous_target")

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
	capturefixture.SetPilot(env.capture, true)

	result, ok := env.callInteract(t, `{"what":"click","selector":"text=OK","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should queue successfully")
	}

	pq := env.capture.Queries().GetLastPendingQuery()
	corrID := pq.CorrelationID

	ambiguousResult := `{
		"success": false,
		"error": "ambiguous_target",
		"candidates": [
			{"element_id":"el_1","visible":true},
			{"element_id":"el_2","visible":true}
		]
	}`
	env.capture.Queries().ApplyCommandResult(corrID, "error", json.RawMessage(ambiguousResult), "ambiguous_target")

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
	capturefixture.SetPilot(env.capture, true)

	result, ok := env.callInteract(t, `{"what":"click","selector":"text=OK","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should queue successfully")
	}

	pq := env.capture.Queries().GetLastPendingQuery()
	corrID := pq.CorrelationID

	// ambiguous_target with no candidates array (edge case)
	ambiguousResult := `{"success":false,"error":"ambiguous_target","match_count":2}`
	env.capture.Queries().ApplyCommandResult(corrID, "error", json.RawMessage(ambiguousResult), "ambiguous_target")

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
