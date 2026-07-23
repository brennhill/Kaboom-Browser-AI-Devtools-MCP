// Purpose: Tests for generate tool audit trail integration.
// Docs: docs/features/feature/test-generation/index.md

// tools_generate_audit_test.go — Behavioral tests for generate tool
//
// ⚠️ ARCHITECTURAL INVARIANT - ALL GENERATE FORMATS MUST WORK
//
// These tests verify ACTUAL BEHAVIOR, not just "doesn't crash":
// 1. Data flow: Generate format → returns expected output structure
// 2. Parameter validation: Required params return errors when missing
// 3. Response format: Returns correctly structured MCP responses
// 4. Safety: All formats execute without panic
//
// Test Categories:
// - Data flow tests: Verify generate returns expected response data
// - Error handling tests: Verify invalid inputs return structured errors
// - Response format tests: Verify MCP response structure
// - Safety net tests: Verify all 10 formats don't panic
//
// Run: go test ./cmd/browser-agent -run "TestGenerateAudit" -v
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// ============================================
// Test Infrastructure
// ============================================

type generateTestEnv struct {
	handler *ToolHandler
	server  *Server
	capture *capture.Store
}

func newGenerateTestEnv(t *testing.T) *generateTestEnv {
	t.Helper()
	server, err := NewServer(t.TempDir()+"/test-generate-audit.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	handler := mcpHandler.toolHandler.(*ToolHandler)
	return &generateTestEnv{handler: handler, server: server, capture: cap}
}

// callGenerate invokes the generate tool and returns parsed result
func (e *generateTestEnv) callGenerate(t *testing.T, argsJSON string) (MCPToolResult, bool) {
	t.Helper()

	args := json.RawMessage(argsJSON)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := e.handler.toolGenerate(req, args)

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
// Behavioral Tests: Data Flow
// These tests verify generate returns expected response data
// ============================================

// TestGenerateAudit_Reproduction_DataFlow verifies reproduction format returns script
func TestGenerateAudit_Reproduction_DataFlow(t *testing.T) {
	env := newGenerateTestEnv(t)

	// Seed actions so reproduction has data to work with
	env.capture.AddEnhancedActionsForTest([]capture.EnhancedAction{
		{Type: "navigate", Timestamp: 1000, URL: "https://example.com", ToURL: "https://example.com"},
		{Type: "click", Timestamp: 2000, URL: "https://example.com", Selectors: map[string]any{"text": "Go"}},
	})

	result, ok := env.callGenerate(t, `{"what":"reproduction"}`)
	if !ok {
		t.Fatal("reproduction should return result")
	}

	// ASSERTION 1: Not an error
	if result.IsError {
		t.Errorf("reproduction should NOT return isError\nGot: %+v", result)
	}

	// ASSERTION 2: Has content
	if len(result.Content) == 0 {
		t.Fatal("reproduction should return content block")
	}

	// ASSERTION 3: Response mentions script
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "script") &&
		!strings.Contains(strings.ToLower(text), "reproduction") {
		t.Errorf("reproduction response should mention script\nGot: %s", text)
	}
}

// TestGenerateAudit_Test_DataFlow verifies test format returns test code
func TestGenerateAudit_Test_DataFlow(t *testing.T) {
	env := newGenerateTestEnv(t)

	result, ok := env.callGenerate(t, `{"what":"test"}`)
	if !ok {
		t.Fatal("test format should return result")
	}

	// ASSERTION 1: Not an error
	if result.IsError {
		t.Errorf("test format should NOT return isError\nGot: %+v", result)
	}

	// ASSERTION 2: Has content
	if len(result.Content) == 0 {
		t.Fatal("test format should return content block")
	}
}

// TestGenerateAudit_Test_ScriptContent verifies test format returns non-empty Playwright skeleton
func TestGenerateAudit_Test_ScriptContent(t *testing.T) {
	env := newGenerateTestEnv(t)

	result, ok := env.callGenerate(t, `{"what":"test","test_name":"smoke-test"}`)
	if !ok {
		t.Fatal("test format should return result")
	}

	if result.IsError {
		t.Fatalf("test format should NOT return isError\nGot: %+v", result)
	}
	if len(result.Content) == 0 {
		t.Fatal("test format should return content block")
	}

	text := result.Content[0].Text

	// Summary line should mention Playwright test name and action count
	if !strings.Contains(text, "Playwright test 'smoke-test'") {
		t.Errorf("summary should contain Playwright test name\nGot: %s", text)
	}

	// Script must contain Playwright imports and test skeleton
	for _, want := range []string{
		`"script":`,
		`import { test, expect }`,
		`test.describe('smoke-test'`,
		`page.goto`,
		`expect(page)`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("response missing expected pattern %q\nGot: %s", want, text)
		}
	}

	// Script must NOT be empty
	if strings.Contains(text, `"script":""`) {
		t.Error("script must NOT be empty — even with 0 actions, a skeleton should be generated")
	}
}

// TestGenerateAudit_PRSummary_DataFlow verifies pr_summary format returns summary
func TestGenerateAudit_PRSummary_DataFlow(t *testing.T) {
	env := newGenerateTestEnv(t)

	result, ok := env.callGenerate(t, `{"what":"pr_summary"}`)
	if !ok {
		t.Fatal("pr_summary should return result")
	}

	// ASSERTION 1: Not an error
	if result.IsError {
		t.Errorf("pr_summary should NOT return isError\nGot: %+v", result)
	}

	// ASSERTION 2: Has content
	if len(result.Content) == 0 {
		t.Fatal("pr_summary should return content block")
	}

	// ASSERTION 3: Response mentions summary
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "summary") &&
		!strings.Contains(strings.ToLower(text), "pr") {
		t.Errorf("pr_summary response should mention summary\nGot: %s", text)
	}
}

// TestGenerateAudit_SARIF_DataFlow verifies sarif format returns SARIF structure
func TestGenerateAudit_SARIF_DataFlow(t *testing.T) {
	env := newGenerateTestEnv(t)

	result, ok := env.callGenerate(t, `{"what":"sarif","scope":"security"}`)
	if !ok {
		t.Fatal("sarif should return result")
	}

	// ASSERTION 1: Not an error
	if result.IsError {
		t.Errorf("sarif should NOT return isError\nGot: %+v", result)
	}

	// ASSERTION 2: Has content
	if len(result.Content) == 0 {
		t.Fatal("sarif should return content block")
	}

	// ASSERTION 3: Response mentions sarif export
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "sarif") {
		t.Errorf("sarif response should mention sarif\nGot: %s", text)
	}
}

// TestGenerateAudit_HAR_DataFlow verifies har format returns HAR structure
func TestGenerateAudit_HAR_DataFlow(t *testing.T) {
	env := newGenerateTestEnv(t)

	result, ok := env.callGenerate(t, `{"what":"har"}`)
	if !ok {
		t.Fatal("har should return result")
	}

	// ASSERTION 1: Not an error
	if result.IsError {
		t.Errorf("har should NOT return isError\nGot: %+v", result)
	}

	// ASSERTION 2: Has content
	if len(result.Content) == 0 {
		t.Fatal("har should return content block")
	}

	// ASSERTION 3: Response mentions HAR
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "har") {
		t.Errorf("har response should mention har\nGot: %s", text)
	}
}

// TestGenerateAudit_CSP_DataFlow verifies csp format returns CSP policy
func TestGenerateAudit_CSP_DataFlow(t *testing.T) {
	env := newGenerateTestEnv(t)

	result, ok := env.callGenerate(t, `{"what":"csp","mode":"strict"}`)
	if !ok {
		t.Fatal("csp should return result")
	}

	// ASSERTION 1: Not an error
	if result.IsError {
		t.Errorf("csp should NOT return isError\nGot: %+v", result)
	}

	// ASSERTION 2: Has content
	if len(result.Content) == 0 {
		t.Fatal("csp should return content block")
	}

	// ASSERTION 3: Response mentions CSP
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "csp") &&
		!strings.Contains(strings.ToLower(text), "policy") {
		t.Errorf("csp response should mention csp or policy\nGot: %s", text)
	}
}

// TestGenerateAudit_SRI_DataFlow verifies sri format returns SRI hashes
func TestGenerateAudit_SRI_DataFlow(t *testing.T) {
	env := newGenerateTestEnv(t)

	result, ok := env.callGenerate(t, `{"what":"sri"}`)
	if !ok {
		t.Fatal("sri should return result")
	}

	// ASSERTION 1: Not an error
	if result.IsError {
		t.Errorf("sri should NOT return isError\nGot: %+v", result)
	}

	// ASSERTION 2: Has content
	if len(result.Content) == 0 {
		t.Fatal("sri should return content block")
	}

	// ASSERTION 3: Response mentions SRI
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "sri") &&
		!strings.Contains(strings.ToLower(text), "hash") {
		t.Errorf("sri response should mention sri or hash\nGot: %s", text)
	}
}

// ============================================
// Behavioral Tests: Error Handling
// Invalid inputs should return structured errors
// ============================================

// TestGenerateAudit_UnknownFormat_ReturnsStructuredError verifies unknown format error
func TestGenerateAudit_UnknownFormat_ReturnsStructuredError(t *testing.T) {
	env := newGenerateTestEnv(t)

	result, ok := env.callGenerate(t, `{"what":"completely_invalid_format_xyz"}`)
	if !ok {
		t.Fatal("unknown format should return result with isError")
	}

	// ASSERTION 1: IsError is true
	if !result.IsError {
		t.Error("unknown format MUST set isError:true")
	}

	// ASSERTION 2: Error message is helpful
	if len(result.Content) == 0 {
		t.Fatal("error response should have content")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "unknown") &&
		!strings.Contains(strings.ToLower(text), "invalid") {
		t.Errorf("error should mention 'unknown' or 'invalid'\nGot: %s", text)
	}
}

// TestGenerateAudit_MissingFormat_ReturnsError verifies missing format returns error
func TestGenerateAudit_MissingFormat_ReturnsError(t *testing.T) {
	env := newGenerateTestEnv(t)

	result, ok := env.callGenerate(t, `{}`)
	if !ok {
		t.Fatal("missing 'format' should return result with error")
	}

	// ASSERTION: IsError is true
	if !result.IsError {
		t.Error("missing 'format' parameter MUST set isError:true")
	}
}

// TestGenerateAudit_InvalidJSON_ReturnsParseError verifies invalid JSON returns error
func TestGenerateAudit_InvalidJSON_ReturnsParseError(t *testing.T) {
	env := newGenerateTestEnv(t)

	args := json.RawMessage(`{invalid json here}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.toolGenerate(req, args)

	// ASSERTION: Returns some response (not nil/panic)
	if resp.Result == nil && resp.Error == nil {
		t.Fatal("invalid JSON should return response, not nil")
	}

	// If result, should be error
	if resp.Result != nil {
		var result MCPToolResult
		_ = json.Unmarshal(resp.Result, &result)
		if !result.IsError {
			t.Error("invalid JSON MUST return isError:true")
		}
	}
}

// ============================================
// Behavioral Tests: Empty State Handling
// Empty state should return empty results, NOT errors
// ============================================

func TestGenerateAudit_EmptyState_ReturnsEmptyNotError(t *testing.T) {
	env := newGenerateTestEnv(t)

	// These formats should return success even with no data.
	// Note: reproduction requires actions and correctly returns an error when empty.
	formats := []struct {
		name string
		args string
	}{
		{"test", `{"what":"test"}`},
		{"pr_summary", `{"what":"pr_summary"}`},
		{"sarif", `{"what":"sarif"}`},
		{"har", `{"what":"har"}`},
		{"csp", `{"what":"csp"}`},
		{"sri", `{"what":"sri"}`},
	}

	for _, tc := range formats {
		t.Run(tc.name, func(t *testing.T) {
			result, ok := env.callGenerate(t, tc.args)

			// ASSERTION 1: Returns result (not nil)
			if !ok {
				t.Fatalf("generate %s should return result even when empty", tc.name)
			}

			// ASSERTION 2: Not an error
			if result.IsError {
				t.Errorf("generate %s with no data should NOT be an error", tc.name)
			}

			// ASSERTION 3: Has content
			if len(result.Content) == 0 {
				t.Errorf("generate %s should return content block", tc.name)
			}
		})
	}
}

// ============================================
// Safety Net: All Formats Execute Without Panic
// ============================================

func TestGenerateAudit_AllFormats_NoPanic(t *testing.T) {
	env := newGenerateTestEnv(t)

	// Complete list from tools_generate.go
	allFormats := []struct {
		format string
		args   string
	}{
		{"reproduction", `{"what":"reproduction"}`},
		{"test", `{"what":"test"}`},
		{"pr_summary", `{"what":"pr_summary"}`},
		{"sarif", `{"what":"sarif"}`},
		{"har", `{"what":"har"}`},
		{"csp", `{"what":"csp"}`},
		{"sri", `{"what":"sri"}`},
		{"test_from_context", `{"what":"test_from_context","context":"error"}`},
		{"test_heal", `{"what":"test_heal","action":"analyze"}`},
		{"test_classify", `{"what":"test_classify"}`},
	}

	for _, tc := range allFormats {
		t.Run(tc.format, func(t *testing.T) {
			// Catch panics
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("generate(%s) PANICKED: %v", tc.format, r)
				}
			}()

			args := json.RawMessage(tc.args)
			req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
			resp := env.handler.toolGenerate(req, args)

			// ASSERTION: Returns something
			if resp.Result == nil && resp.Error == nil {
				t.Errorf("generate(%s) returned nil response", tc.format)
			}
		})
	}
}
