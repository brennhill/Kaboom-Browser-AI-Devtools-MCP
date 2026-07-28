// Purpose: Tests for configure noise rule management.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_configure_noise_test.go — Coverage tests for noise and analyze(api_validation) handlers.
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// toolConfigureNoise — 0% → 80%+
// ============================================

func TestToolConfigureNoise_List(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"noise_rule","noise_action":"list"}`)
	if !ok {
		t.Fatal("noise list should return result")
	}
	if result.IsError {
		t.Fatalf("noise list should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if _, ok := data["rules"]; !ok {
		t.Error("response should contain rules")
	}
	if _, ok := data["statistics"]; !ok {
		t.Error("response should contain statistics")
	}
}

func TestToolConfigureNoise_Reset(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"noise_rule","noise_action":"reset"}`)
	if !ok {
		t.Fatal("noise reset should return result")
	}
	if result.IsError {
		t.Fatalf("noise reset should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if status, _ := data["status"].(string); status != "ok" {
		t.Fatalf("status = %q, want ok", status)
	}
}

func TestToolConfigureNoise_AddRule(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"noise_rule","noise_action":"add","rules":[{"category":"console","classification":"noisy","match_spec":{"message_regex":"test.*pattern"}}]}`)
	if !ok {
		t.Fatal("noise add should return result")
	}
	if result.IsError {
		t.Fatalf("noise add should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	rulesAdded, _ := data["rules_added"].(float64)
	if rulesAdded != 1 {
		t.Fatalf("rules_added = %v, want 1", rulesAdded)
	}
}

func TestToolConfigureNoise_AddRule_FlatParams(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"noise_rule","noise_action":"add","pattern":"flat-single-noise"}`)
	if !ok {
		t.Fatal("noise add flat params should return result")
	}
	if result.IsError {
		t.Fatalf("noise add flat params should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	rulesAdded, _ := data["rules_added"].(float64)
	if rulesAdded != 1 {
		t.Fatalf("rules_added = %v, want 1", rulesAdded)
	}

	listResult, ok := env.callConfigure(t, `{"what":"noise_rule","noise_action":"list"}`)
	if !ok {
		t.Fatal("noise list should return result")
	}
	if listResult.IsError {
		t.Fatalf("noise list should not error, got: %s", listResult.Content[0].Text)
	}
	listData := parseResponseJSON(t, listResult)
	rules, ok := listData["rules"].([]any)
	if !ok {
		t.Fatalf("rules type = %T, want []any", listData["rules"])
	}
	found := false
	for _, r := range rules {
		rule, ok := r.(map[string]any)
		if !ok {
			continue
		}
		ms, _ := rule["match_spec"].(map[string]any)
		if ms == nil {
			continue
		}
		if ms["message_regex"] == "flat-single-noise" {
			found = true
			if rule["category"] != "console" {
				t.Fatalf("flat rule category = %v, want console", rule["category"])
			}
		}
	}
	if !found {
		t.Fatal("flat single noise rule was not added")
	}
}

func TestToolConfigureNoise_RemoveMissingRuleID(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"noise_rule","noise_action":"remove"}`)
	if !ok {
		t.Fatal("noise remove should return result")
	}
	if !result.IsError {
		t.Fatal("noise remove without rule_id should return error")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "rule_id") {
		t.Errorf("error should mention rule_id, got: %s", text)
	}
}

func TestToolConfigureNoise_AutoDetect(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"noise_rule","noise_action":"auto_detect"}`)
	if !ok {
		t.Fatal("noise auto_detect should return result")
	}
	if result.IsError {
		t.Fatalf("noise auto_detect should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if _, ok := data["proposals"]; !ok {
		t.Error("response should contain proposals")
	}
}

func TestToolConfigureNoise_UnknownAction(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"what":"noise_rule","noise_action":"invalid_action"}`)
	if !ok {
		t.Fatal("noise unknown action should return result")
	}
	if !result.IsError {
		t.Fatal("noise unknown action should return error")
	}
}

func TestToolConfigureNoise_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	args := json.RawMessage(`{bad json}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := toolconfigure.HandleNoise(env.handler.configureLocalDeps, req, args)

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}

// TestToolConfigureNoise_FullLifecycle tests add -> list -> verify -> remove -> list -> verify gone.
func TestToolConfigureNoise_FullLifecycle(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	// Step 1: Add a rule
	addResult, ok := env.callConfigure(t, `{"what":"noise_rule","noise_action":"add","rules":[{"category":"console","match_spec":{"message_regex":"smoke-test-noise"}}]}`)
	if !ok {
		t.Fatal("noise add should return result")
	}
	if addResult.IsError {
		t.Fatalf("noise add should not error, got: %s", addResult.Content[0].Text)
	}
	t.Logf("ADD response text: %s", addResult.Content[0].Text)

	addData := parseResponseJSON(t, addResult)
	rulesAdded, _ := addData["rules_added"].(float64)
	if rulesAdded != 1 {
		t.Fatalf("rules_added = %v, want 1", rulesAdded)
	}

	// Step 2: List rules and verify the added rule is present
	listResult, ok := env.callConfigure(t, `{"what":"noise_rule","noise_action":"list"}`)
	if !ok {
		t.Fatal("noise list should return result")
	}
	if listResult.IsError {
		t.Fatalf("noise list should not error, got: %s", listResult.Content[0].Text)
	}
	listText := listResult.Content[0].Text
	t.Logf("LIST response text: %s", listText)

	if !strings.Contains(listText, "smoke-test-noise") {
		t.Fatalf("noise list should contain 'smoke-test-noise', got: %s", listText)
	}

	// Step 3: Extract rule_id from the list response
	listData := parseResponseJSON(t, listResult)
	rules, ok := listData["rules"].([]any)
	if !ok || len(rules) == 0 {
		t.Fatalf("expected rules array, got: %v", listData["rules"])
	}

	var ruleID string
	for _, r := range rules {
		rMap, ok := r.(map[string]any)
		if !ok {
			continue
		}
		matchSpec, _ := rMap["match_spec"].(map[string]any)
		if matchSpec != nil {
			if msgRegex, _ := matchSpec["message_regex"].(string); msgRegex == "smoke-test-noise" {
				ruleID, _ = rMap["id"].(string)
				break
			}
		}
	}
	if ruleID == "" {
		t.Fatalf("could not find rule_id for 'smoke-test-noise' in rules: %v", rules)
	}
	t.Logf("Found rule_id: %s", ruleID)

	// Step 4: Remove the rule
	removeResult, ok := env.callConfigure(t, `{"what":"noise_rule","noise_action":"remove","rule_id":"`+ruleID+`"}`)
	if !ok {
		t.Fatal("noise remove should return result")
	}
	if removeResult.IsError {
		t.Fatalf("noise remove should not error, got: %s", removeResult.Content[0].Text)
	}
	t.Logf("REMOVE response text: %s", removeResult.Content[0].Text)

	// Step 5: List again and verify the rule is gone
	list2Result, ok := env.callConfigure(t, `{"what":"noise_rule","noise_action":"list"}`)
	if !ok {
		t.Fatal("noise list2 should return result")
	}
	if list2Result.IsError {
		t.Fatalf("noise list2 should not error, got: %s", list2Result.Content[0].Text)
	}
	list2Text := list2Result.Content[0].Text
	t.Logf("LIST2 response text: %s", list2Text)

	if strings.Contains(list2Text, "smoke-test-noise") {
		t.Fatalf("noise list should NOT contain 'smoke-test-noise' after removal, got: %s", list2Text)
	}

	// Step 6: Also test the wire-format JSON-RPC response (what the smoke test would see)
	// This simulates what the HTTP endpoint returns
	listArgs := json.RawMessage(`{"what":"noise_rule","noise_action":"list"}`)
	listReq := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 42}
	listResp := env.handler.configureDispatcher.Handle(listReq, listArgs)
	wireJSON, _ := json.Marshal(listResp)
	t.Logf("Wire-format JSON-RPC response: %s", string(wireJSON))
}

// ============================================
// toolValidateAPI — 0% → 100%
// Routed via analyze({what: "api_validation"})
// ============================================

func TestToolValidateAPI_Analyze(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	args := json.RawMessage(`{"operation":"analyze"}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.analyzeDispatcher.ValidateAPI(req, args)

	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("validate_api analyze should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if status, _ := data["status"].(string); status != "ok" {
		t.Fatalf("status = %q, want ok", status)
	}
	if op, _ := data["operation"].(string); op != "analyze" {
		t.Fatalf("operation = %q, want analyze", op)
	}
}

func TestToolValidateAPI_Report(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	args := json.RawMessage(`{"operation":"report"}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.analyzeDispatcher.ValidateAPI(req, args)

	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("validate_api report should not error, got: %s", result.Content[0].Text)
	}
}

func TestToolValidateAPI_Clear(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	args := json.RawMessage(`{"operation":"clear"}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.analyzeDispatcher.ValidateAPI(req, args)

	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("validate_api clear should not error, got: %s", result.Content[0].Text)
	}
}

func TestToolValidateAPI_UnknownOperation(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	args := json.RawMessage(`{"operation":"invalid"}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.analyzeDispatcher.ValidateAPI(req, args)

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("validate_api with invalid operation should return error")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "operation") {
		t.Errorf("error should mention operation, got: %s", text)
	}
}

func TestToolValidateAPI_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	args := json.RawMessage(`{bad json}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.analyzeDispatcher.ValidateAPI(req, args)

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}
