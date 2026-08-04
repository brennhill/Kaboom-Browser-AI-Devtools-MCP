// tools_csp_blocked_test.go — Tests for blocked_actions injection when CSP restricts.
// Verifies that observe(what:"page") and navigate responses include blocked_actions
// and blocked_reason when CSP level is not "none", and omit them when CSP is clear.
//
// Run: go test ./cmd/browser-agent -run "TestCSP_Blocked" -v -count=1
package main

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// Helper: parse observe(what:"page") response
// ============================================

func parsePageInfoResponse(t *testing.T, resp mcp.JSONRPCResponse) map[string]any {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal MCPToolResult: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success response, got error: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("response has no content blocks")
	}
	// The text starts with a summary line, followed by JSON on the next line.
	text := result.Content[0].Text
	jsonStart := -1
	for i, ch := range text {
		if ch == '{' {
			jsonStart = i
			break
		}
	}
	if jsonStart < 0 {
		t.Fatalf("no JSON found in response text: %s", text)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text[jsonStart:]), &data); err != nil {
		t.Fatalf("unmarshal response JSON: %v\nraw: %s", err, text[jsonStart:])
	}
	return data
}

// ============================================
// Test: CSP blocked_actions omitted when "none"
// ============================================

func TestCSP_BlockedActions_None_Omitted(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.simulateConnection(t)
	env.simulateTabTracking(t)
	capturefixture.SetCSP(env.capture, false, "none")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"page"}`)
	resp := env.handler.observeDispatcher.Handle(req, args)

	data := parsePageInfoResponse(t, resp)

	// When CSP is "none", blocked_actions and blocked_reason MUST be absent
	if _, exists := data["blocked_actions"]; exists {
		t.Fatal("blocked_actions should be omitted when csp_level is 'none'")
	}
	if _, exists := data["blocked_reason"]; exists {
		t.Fatal("blocked_reason should be omitted when csp_level is 'none'")
	}
}

// ============================================
// Test: CSP blocked_actions for script_exec
// ============================================

func TestCSP_BlockedActions_ScriptExec(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.simulateConnection(t)
	env.simulateTabTracking(t)
	capturefixture.SetCSP(env.capture, true, "script_exec")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"page"}`)
	resp := env.handler.observeDispatcher.Handle(req, args)

	data := parsePageInfoResponse(t, resp)

	// When CSP is "script_exec", blocked_actions should contain execute_js
	rawActions, exists := data["blocked_actions"]
	if !exists {
		t.Fatal("expected blocked_actions in response when csp_level is 'script_exec'")
	}
	actions, ok := rawActions.([]any)
	if !ok {
		t.Fatalf("blocked_actions should be an array, got %T", rawActions)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 blocked action for script_exec, got %d: %v", len(actions), actions)
	}
	if actions[0] != "execute_js" {
		t.Fatalf("expected blocked_actions[0]='execute_js', got %v", actions[0])
	}

	// blocked_reason should be present and non-empty
	reason, exists := data["blocked_reason"]
	if !exists {
		t.Fatal("expected blocked_reason in response when csp_level is 'script_exec'")
	}
	reasonStr, ok := reason.(string)
	if !ok || reasonStr == "" {
		t.Fatal("blocked_reason should be a non-empty string")
	}
}

// ============================================
// Test: CSP blocked_actions for page_blocked
// ============================================

func TestCSP_BlockedActions_PageBlocked(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.simulateConnection(t)
	env.simulateTabTracking(t)
	capturefixture.SetCSP(env.capture, true, "page_blocked")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"page"}`)
	resp := env.handler.observeDispatcher.Handle(req, args)

	data := parsePageInfoResponse(t, resp)

	// When CSP is "page_blocked", blocked_actions should contain all DOM actions
	rawActions, exists := data["blocked_actions"]
	if !exists {
		t.Fatal("expected blocked_actions in response when csp_level is 'page_blocked'")
	}
	actions, ok := rawActions.([]any)
	if !ok {
		t.Fatalf("blocked_actions should be an array, got %T", rawActions)
	}

	expectedActions := []string{
		"execute_js", "click", "type", "select", "check", "scroll_to", "focus",
		"get_text", "get_value", "get_attribute", "set_attribute",
		"list_interactive", "get_readable", "get_markdown",
		"fill_form", "fill_form_and_submit",
	}
	if len(actions) != len(expectedActions) {
		t.Fatalf("expected %d blocked actions for page_blocked, got %d: %v", len(expectedActions), len(actions), actions)
	}

	actionSet := make(map[string]bool)
	for _, a := range actions {
		s, ok := a.(string)
		if !ok {
			t.Fatalf("blocked_actions should contain strings, got %T", a)
		}
		actionSet[s] = true
	}
	for _, expected := range expectedActions {
		if !actionSet[expected] {
			t.Fatalf("expected blocked_actions to contain %q for page_blocked", expected)
		}
	}

	// blocked_reason should be present
	reason, exists := data["blocked_reason"]
	if !exists {
		t.Fatal("expected blocked_reason in response when csp_level is 'page_blocked'")
	}
	reasonStr, ok := reason.(string)
	if !ok || reasonStr == "" {
		t.Fatal("blocked_reason should be a non-empty string")
	}
}

// ============================================
// Test: observe(what:"page") includes blocked_actions when CSP restricted
// ============================================

func TestCSP_Page_IncludesBlockedActions(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.simulateConnection(t)
	env.simulateTabTracking(t)
	capturefixture.SetCSP(env.capture, true, "script_exec")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"page"}`)
	resp := env.handler.observeDispatcher.Handle(req, args)

	data := parsePageInfoResponse(t, resp)

	// Verify all three CSP-related fields are present
	if _, exists := data["csp_restricted"]; !exists {
		t.Fatal("expected csp_restricted in page response")
	}
	if _, exists := data["csp_level"]; !exists {
		t.Fatal("expected csp_level in page response")
	}
	if _, exists := data["blocked_actions"]; !exists {
		t.Fatal("expected blocked_actions in page response when CSP restricted")
	}
	if _, exists := data["blocked_reason"]; !exists {
		t.Fatal("expected blocked_reason in page response when CSP restricted")
	}

	// Verify csp_level matches
	if data["csp_level"] != "script_exec" {
		t.Fatalf("expected csp_level='script_exec', got %v", data["csp_level"])
	}
}

// ============================================
// Test: navigate response includes blocked_actions when CSP restricted
// ============================================

func TestCSP_Navigate_IncludesBlockedActions(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.enablePilot(t)
	env.simulateConnection(t)
	capturefixture.SetCSP(env.capture, true, "script_exec")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"navigate","url":"https://example.com","sync":false}`)
	resp := env.handler.browserActions.Handle("navigate", req, args)

	// Parse the queued response
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal MCPToolResult: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success (queued) response, got error: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("response has no content blocks")
	}

	text := result.Content[0].Text
	jsonStart := -1
	for i, ch := range text {
		if ch == '{' {
			jsonStart = i
			break
		}
	}
	if jsonStart < 0 {
		t.Fatalf("no JSON found in navigate response: %s", text)
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(text[jsonStart:]), &data); err != nil {
		t.Fatalf("unmarshal response JSON: %v\nraw: %s", err, text[jsonStart:])
	}

	// Navigate with sync=false returns a "queued" response. Even the queued response
	// should include blocked_actions when CSP is restricted.
	rawActions, exists := data["blocked_actions"]
	if !exists {
		t.Fatal("expected blocked_actions in navigate response when CSP restricted")
	}
	actions, ok := rawActions.([]any)
	if !ok {
		t.Fatalf("blocked_actions should be an array, got %T", rawActions)
	}
	if len(actions) != 1 || actions[0] != "execute_js" {
		t.Fatalf("expected blocked_actions=['execute_js'] for script_exec, got %v", actions)
	}

	reason, exists := data["blocked_reason"]
	if !exists {
		t.Fatal("expected blocked_reason in navigate response when CSP restricted")
	}
	if _, ok := reason.(string); !ok {
		t.Fatal("blocked_reason should be a string")
	}
}

// ============================================
// Test: navigate response omits blocked_actions when CSP clear
// ============================================

func TestCSP_Navigate_OmitsBlockedActions_WhenClear(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.enablePilot(t)
	env.simulateConnection(t)
	capturefixture.SetCSP(env.capture, false, "none")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"navigate","url":"https://example.com","sync":false}`)
	resp := env.handler.browserActions.Handle("navigate", req, args)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal MCPToolResult: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success (queued) response, got error: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("response has no content blocks")
	}

	text := result.Content[0].Text
	jsonStart := -1
	for i, ch := range text {
		if ch == '{' {
			jsonStart = i
			break
		}
	}
	if jsonStart < 0 {
		t.Fatalf("no JSON found in navigate response: %s", text)
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(text[jsonStart:]), &data); err != nil {
		t.Fatalf("unmarshal response JSON: %v\nraw: %s", err, text[jsonStart:])
	}

	// When CSP is clear, blocked_actions should be absent (zero token cost)
	if _, exists := data["blocked_actions"]; exists {
		t.Fatal("blocked_actions should be omitted in navigate response when CSP is clear")
	}
	if _, exists := data["blocked_reason"]; exists {
		t.Fatal("blocked_reason should be omitted in navigate response when CSP is clear")
	}
}
