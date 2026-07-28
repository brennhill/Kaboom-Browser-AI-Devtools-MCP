// Purpose: Tests for generate tool handler dispatch.
// Docs: docs/features/feature/test-generation/index.md

// tools_generate_handler_test.go — Comprehensive unit tests for generate tool dispatch and response fields.
// Validates all response fields, snake_case JSON convention, and output structure.
package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// Dispatch Tests
// ============================================

func TestToolsGenerateDispatch_InvalidJSON(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callGenerateRaw(h, `{bad json`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid JSON should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "invalid_json") {
		t.Errorf("error code should be 'invalid_json', got: %s", result.Content[0].Text)
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsGenerateDispatch_MissingFormat(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callGenerateRaw(h, `{}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("missing 'format' should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "missing_param") {
		t.Errorf("error code should be 'missing_param', got: %s", result.Content[0].Text)
	}
	// Verify hint lists valid formats
	text := result.Content[0].Text
	for _, format := range []string{"reproduction", "test", "sarif", "har", "csp"} {
		if !strings.Contains(text, format) {
			t.Errorf("hint should list valid format %q", format)
		}
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsGenerateDispatch_UnknownFormat(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callGenerateRaw(h, `{"what":"nonexistent_format"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("unknown format should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "unknown_mode") {
		t.Errorf("error code should be 'unknown_mode', got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "nonexistent_format") {
		t.Error("error should mention the invalid format name")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsGenerateDispatch_UnknownFormatAliasAddsCanonicalWhatWarning(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callGenerateRaw(h, `{"format":"nonexistent_format"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("unknown format alias should return isError:true")
	}
	foundCanonicalWarning := false
	for _, block := range result.Content {
		if strings.Contains(block.Text, "deprecated") {
			foundCanonicalWarning = true
			break
		}
	}
	if !foundCanonicalWarning {
		t.Fatalf("expected canonical what warning block on error path, got %d content blocks", len(result.Content))
	}
}

func TestToolsGenerateDispatch_EmptyArgs(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.generateDispatcher.Handle(req, nil)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("nil args (no 'format') should return isError:true")
	}
}

func TestToolsGenerateDispatch_FormatAliasAddsCanonicalWhatWarning(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callGenerateRaw(h, `{"format":"pr_summary"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("format alias should be accepted, got: %s", result.Content[0].Text)
	}
	foundCanonicalWarning := false
	for _, block := range result.Content {
		if strings.Contains(block.Text, "deprecated") {
			foundCanonicalWarning = true
			break
		}
	}
	if !foundCanonicalWarning {
		t.Fatalf("expected canonical what warning block, got %d content blocks", len(result.Content))
	}
}

func TestToolsGenerateDispatch_ConflictingWhatAndFormat(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callGenerateRaw(h, `{"what":"test","format":"har"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("conflicting what/format should return isError:true")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "invalid_param") {
		t.Fatalf("expected invalid_param, got: %s", text)
	}
	if !strings.Contains(text, "Conflicting parameters") {
		t.Fatalf("expected conflict explanation, got: %s", text)
	}
}

// ============================================
// getValidGenerateFormats Tests
// ============================================

func TestToolsGenerate_GetValidGenerateFormats(t *testing.T) {
	t.Parallel()

	formats := toolgenerate.ValidFormats()
	formatList := strings.Split(formats, ", ")
	for i := 1; i < len(formatList); i++ {
		if formatList[i-1] > formatList[i] {
			t.Errorf("formats not sorted: %q > %q", formatList[i-1], formatList[i])
		}
	}

	for _, required := range []string{"reproduction", "test", "sarif", "har", "csp", "sri", "pr_summary"} {
		if !strings.Contains(formats, required) {
			t.Errorf("valid formats missing %q: %s", required, formats)
		}
	}
}

// ============================================
// generate(format:"test") — Response Fields
// ============================================

func TestToolsGenerateTest_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callGenerateRaw(h, `{"what":"test","test_name":"smoke"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("test format should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"script", "test_name", "action_count", "metadata"} {
		if _, ok := data[field]; !ok {
			t.Errorf("test response missing field %q", field)
		}
	}

	if data["test_name"] != "smoke" {
		t.Errorf("test_name = %v, want 'smoke'", data["test_name"])
	}

	// Verify metadata fields
	meta, _ := data["metadata"].(map[string]any)
	if meta == nil {
		t.Fatal("metadata should be a map")
	}
	for _, field := range []string{"generated_at", "actions_available", "actions_included", "assert_network", "assert_no_errors"} {
		if _, ok := meta[field]; !ok {
			t.Errorf("test metadata missing field %q", field)
		}
	}

	// Verify script content
	script, _ := data["script"].(string)
	if !strings.Contains(script, "import { test, expect }") {
		t.Error("test script should contain Playwright imports")
	}
	if !strings.Contains(script, "test.describe('smoke'") {
		t.Error("test script should contain test.describe with test name")
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsGenerateTest_DefaultTestName(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callGenerateRaw(h, `{"what":"test"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("test format should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["test_name"] != "generated test" {
		t.Errorf("default test_name = %v, want 'generated test'", data["test_name"])
	}
}

func TestToolsGenerateTest_WithActions(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	cap.AddEnhancedActionsForTest([]types.EnhancedAction{
		{Type: "navigate", Timestamp: 1000, URL: "https://example.com", ToURL: "https://example.com"},
		{Type: "click", Timestamp: 2000, URL: "https://example.com", Selectors: map[string]any{"css": "#btn"}},
	})

	resp := callGenerateRaw(h, `{"what":"test","test_name":"e2e"}`)
	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	actionCount, _ := data["action_count"].(float64)
	if actionCount != 2 {
		t.Errorf("action_count = %v, want 2", actionCount)
	}

	meta, _ := data["metadata"].(map[string]any)
	actionsAvail, _ := meta["actions_available"].(float64)
	if actionsAvail != 2 {
		t.Errorf("actions_available = %v, want 2", actionsAvail)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// generate(format:"pr_summary") — Response Fields
// ============================================

func TestToolsGeneratePRSummary_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callGenerateRaw(h, `{"what":"pr_summary"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("pr_summary should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"summary", "stats"} {
		if _, ok := data[field]; !ok {
			t.Errorf("pr_summary response missing field %q", field)
		}
	}

	// Verify stats fields
	stats, _ := data["stats"].(map[string]any)
	if stats == nil {
		t.Fatal("stats should be a map")
	}
	for _, field := range []string{"actions", "commands_completed", "commands_failed", "console_errors", "network_errors", "network_captured"} {
		if _, ok := stats[field]; !ok {
			t.Errorf("stats missing field %q", field)
		}
	}

	// Summary should be markdown
	summary, _ := data["summary"].(string)
	if !strings.Contains(summary, "## Session Summary") {
		t.Error("summary should contain markdown heading")
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsGeneratePRSummary_WithActivity(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	cap.AddEnhancedActionsForTest([]types.EnhancedAction{
		{Type: "click", Timestamp: time.Now().UnixMilli(), URL: "https://example.com"},
	})

	resp := callGenerateRaw(h, `{"what":"pr_summary"}`)
	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	stats, _ := data["stats"].(map[string]any)
	actions, _ := stats["actions"].(float64)
	if actions != 1 {
		t.Errorf("stats.actions = %v, want 1", actions)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// generate(format:"csp") — Response Fields
// ============================================

func TestToolsGenerateCSP_EmptyNetwork(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callGenerateRaw(h, `{"what":"csp"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("csp empty should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "unavailable" {
		t.Errorf("status = %v, want 'unavailable' (no network data)", data["status"])
	}
	if _, ok := data["reason"]; !ok {
		t.Error("response missing 'reason' field")
	}
	if _, ok := data["hint"]; !ok {
		t.Error("response missing 'hint' field")
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsGenerateCSP_WithNetworkData(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	cap.AddNetworkBodies([]types.NetworkBody{
		{URL: "https://cdn.example.com/app.js", ContentType: "application/javascript", Status: 200, Timestamp: time.Now().UTC().Format(time.RFC3339)},
		{URL: "https://fonts.googleapis.com/css", ContentType: "text/css", Status: 200, Timestamp: time.Now().UTC().Format(time.RFC3339)},
	})

	resp := callGenerateRaw(h, `{"what":"csp"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("csp should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}
	for _, field := range []string{"mode", "policy", "directives", "origins_observed"} {
		if _, ok := data[field]; !ok {
			t.Errorf("csp response missing field %q", field)
		}
	}

	// Policy string should contain default-src
	policy, _ := data["policy"].(string)
	if !strings.Contains(policy, "default-src") {
		t.Error("CSP policy should contain default-src directive")
	}

	// Directives should be a map
	directives, _ := data["directives"].(map[string]any)
	if directives == nil {
		t.Fatal("directives should be a map")
	}
	if _, ok := directives["default-src"]; !ok {
		t.Error("directives should include default-src")
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsGenerateCSP_DefaultMode(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	cap.AddNetworkBodies([]types.NetworkBody{
		{URL: "https://cdn.example.com/app.js", ContentType: "application/javascript", Status: 200, Timestamp: time.Now().UTC().Format(time.RFC3339)},
	})

	// No mode param should default to "moderate"
	resp := callGenerateRaw(h, `{"what":"csp"}`)
	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)
	if data["mode"] != "moderate" {
		t.Errorf("default mode = %v, want 'moderate'", data["mode"])
	}
}

// ============================================
// generate(format:"sri") — Response Fields
// ============================================

func TestToolsGenerateSRI_EmptyNetwork(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callGenerateRaw(h, `{"what":"sri"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("sri empty should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "unavailable" {
		t.Errorf("status = %v, want 'unavailable' (no network data)", data["status"])
	}
	if _, ok := data["hint"]; !ok {
		t.Error("response missing 'hint' field")
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// generate(format:"har") — Response Fields
// ============================================

func TestToolsGenerateHAR_EmptyNetwork(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callGenerateRaw(h, `{"what":"har"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("har empty should succeed, got: %s", result.Content[0].Text)
	}

	// HAR response should contain the HAR structure
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "har") {
		t.Errorf("har response should mention 'HAR', got: %s", text)
	}
}

// ============================================
// generate(format:"sarif") — Response Fields
// ============================================

func TestToolsGenerateSARIF_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callGenerateRaw(h, `{"what":"sarif"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("sarif should succeed, got: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "sarif") {
		t.Errorf("sarif response should mention 'SARIF', got: %s", text)
	}
}

// NOTE: generateTestScript, groupActionsByNavigation, and testLabelForGroup
// tests moved to internal/tools/generate/test_script_test.go

// ============================================
// All generate formats safety net
// ============================================

func TestToolsGenerate_AllFormats_ResponseHasContent(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	formats := []struct {
		format string
		args   string
	}{
		{"test", `{"what":"test"}`},
		{"pr_summary", `{"what":"pr_summary"}`},
		{"csp", `{"what":"csp"}`},
		{"sri", `{"what":"sri"}`},
		{"har", `{"what":"har"}`},
		{"sarif", `{"what":"sarif"}`},
	}

	for _, tc := range formats {
		t.Run(tc.format, func(t *testing.T) {
			resp := callGenerateRaw(h, tc.args)
			result := parseToolResult(t, resp)

			if len(result.Content) == 0 {
				t.Fatalf("generate(%s) should return at least one content block", tc.format)
			}
			if result.Content[0].Type != "text" {
				t.Errorf("generate(%s) content type = %q, want 'text'", tc.format, result.Content[0].Type)
			}
			if result.Content[0].Text == "" {
				t.Errorf("generate(%s) content text should not be empty", tc.format)
			}

			// All successful responses should have snake_case fields
			if !result.IsError {
				assertSnakeCaseFields(t, string(resp.Result))
			}
		})
	}
}
