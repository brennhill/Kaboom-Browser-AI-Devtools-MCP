// Purpose: Validate analyze(what="navigation") SPA route discovery.
// Why: Prevents regressions in navigation link extraction for AI agent route discovery.
// Docs: docs/features/feature/analyze-tool/index.md

// tools_analyze_navigation_test.go — Tests for analyze(what="navigation") mode.
package main

import (
	"strings"
	"testing"
)

// ============================================
// Dispatch Tests
// ============================================

func TestToolsAnalyzeNavigation_DispatchesQuery(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"navigation","sync":false}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("navigation should queue successfully, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "navigation_") {
		t.Errorf("correlation_id should start with 'navigation_', got: %s", corr)
	}

	pq := cap.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}
	if pq.Type != "navigation" {
		t.Errorf("pending query type = %q, want 'navigation'", pq.Type)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeNavigation_InValidModes(t *testing.T) {
	t.Parallel()

	h, _, _ := makeToolHandler(t)
	modes := strings.Join(h.analyzeDispatcher.ValidModes(), ", ")
	if !strings.Contains(modes, "navigation") {
		t.Errorf("valid analyze modes should include 'navigation': %s", modes)
	}
}

// ============================================
// Mode Spec Tests (via describe_capabilities)
// ============================================

func TestToolsAnalyzeNavigation_ModeSpecExists(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"describe_capabilities","tool":"analyze","mode":"navigation"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("describe_capabilities for analyze/navigation should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	// Should contain mode info — if the mode spec is registered, it returns params info
	if data == nil {
		t.Fatal("describe_capabilities should return non-nil data")
	}
}

// ============================================
// Schema Tests
// ============================================

func TestToolsAnalyzeSchema_NavigationInWhatEnum(t *testing.T) {
	t.Parallel()
	_, _, _ = makeToolHandler(t)

	tools := toolSchemasForTest()
	var analyzeSchema map[string]any
	for _, tool := range tools {
		if tool.Name == "analyze" {
			analyzeSchema = tool.InputSchema
			break
		}
	}
	if analyzeSchema == nil {
		t.Fatal("analyze tool not found in ToolsList()")
	}

	props, ok := analyzeSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("analyze schema missing properties")
	}
	whatProp, ok := props["what"].(map[string]any)
	if !ok {
		t.Fatal("analyze schema missing 'what' property")
	}
	enumValues, ok := whatProp["enum"].([]string)
	if !ok {
		t.Fatal("'what' property missing enum")
	}

	found := false
	for _, v := range enumValues {
		if v == "navigation" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("'what' enum should include 'navigation', got: %v", enumValues)
	}
}

// ============================================
// Response Structure (all modes safety net)
// ============================================

func TestToolsAnalyzeNavigation_ResponseStructure(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"navigation","sync":false}`)
	if resp.Result == nil {
		t.Fatal("analyze(navigation) returned nil result")
	}

	result := parseToolResult(t, resp)
	if len(result.Content) == 0 {
		t.Error("analyze(navigation) should return at least one content block")
	}
	if result.Content[0].Type != "text" {
		t.Errorf("content type = %q, want 'text'", result.Content[0].Type)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}
