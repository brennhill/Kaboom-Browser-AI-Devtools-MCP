// Purpose: Validate analyze(what="page_structure") structural page analysis.
// Why: Prevents regressions in framework detection, scroll containers, modals, shadow DOM discovery.
// Docs: docs/features/feature/analyze-tool/index.md

// tools_analyze_page_structure_test.go — Tests for analyze(what="page_structure") mode.
package main

import (
	"strings"
	"testing"
)

// ============================================
// Dispatch Tests
// ============================================

func TestToolsAnalyzePageStructure_DispatchesQuery(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"page_structure","sync":false}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("page_structure should queue successfully, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "page_structure_") {
		t.Errorf("correlation_id should start with 'page_structure_', got: %s", corr)
	}

	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}
	if pq.Type != "page_structure" {
		t.Errorf("pending query type = %q, want 'page_structure'", pq.Type)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzePageStructure_InValidModes(t *testing.T) {
	t.Parallel()

	h, _, _ := makeToolHandler(t)
	modes := strings.Join(h.analyzeDispatcher.ValidModes(), ", ")
	if !strings.Contains(modes, "page_structure") {
		t.Errorf("valid analyze modes should include 'page_structure': %s", modes)
	}
}

// ============================================
// Schema Tests
// ============================================

func TestToolsAnalyzeSchema_PageStructureInWhatEnum(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	tools := h.ToolsList()
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
		if v == "page_structure" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("'what' enum should include 'page_structure', got: %v", enumValues)
	}
}

// ============================================
// Response Structure
// ============================================

func TestToolsAnalyzePageStructure_ResponseStructure(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"page_structure","sync":false}`)
	if resp.Result == nil {
		t.Fatal("analyze(page_structure) returned nil result")
	}

	result := parseToolResult(t, resp)
	if len(result.Content) == 0 {
		t.Error("analyze(page_structure) should return at least one content block")
	}
	if result.Content[0].Type != "text" {
		t.Errorf("content type = %q, want 'text'", result.Content[0].Type)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// Invalid Args
// ============================================

func TestToolsAnalyzePageStructure_InvalidJSON(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"page_structure","tab_id":"not_a_number"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("page_structure with invalid tab_id type should return error")
	}
}

// ============================================
// Tab ID Passthrough
// ============================================

func TestToolsAnalyzePageStructure_TabIDPassthrough(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"page_structure","tab_id":42,"sync":false}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("page_structure with tab_id should succeed, got: %s", result.Content[0].Text)
	}

	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}
	if pq.TabID != 42 {
		t.Errorf("pending query TabID = %d, want 42", pq.TabID)
	}
}
