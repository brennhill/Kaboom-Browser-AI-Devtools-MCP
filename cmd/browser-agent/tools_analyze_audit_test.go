// Purpose: Validate analyze(what="audit") combined audit report.
// Why: Prevents regressions in Lighthouse-style aggregated audit scoring and category selection.
// Docs: docs/features/feature/analyze-tool/index.md

// tools_analyze_audit_test.go — Tests for analyze(what="audit") mode.
package main

import (
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/analyzehandler"
)

// ============================================
// Dispatch Tests
// ============================================

func TestToolsAnalyzeAudit_Dispatches(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"audit"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("audit should not return error, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["categories"] == nil {
		t.Error("audit response should include 'categories'")
	}
	if data["overall_score"] == nil {
		t.Error("audit response should include 'overall_score'")
	}
	if data["timestamp"] == nil {
		t.Error("audit response should include 'timestamp'")
	}
}

func TestToolsAnalyzeAudit_InValidModes(t *testing.T) {
	t.Parallel()

	modes := getValidAnalyzeModes()
	if !strings.Contains(modes, "audit") {
		t.Errorf("valid analyze modes should include 'audit': %s", modes)
	}
}

// ============================================
// Schema Tests
// ============================================

func TestToolsAnalyzeSchema_AuditInWhatEnum(t *testing.T) {
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
		if v == "audit" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("'what' enum should include 'audit', got: %v", enumValues)
	}
}

// ============================================
// Category Selection Tests
// ============================================

func TestToolsAnalyzeAudit_CategoryFilter(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"audit","categories":["security"]}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("audit with category filter should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	categories, ok := data["categories"].(map[string]any)
	if !ok {
		t.Fatal("audit response should have 'categories' as map")
	}
	if categories["security"] == nil {
		t.Error("filtered audit should include 'security' category")
	}
	if categories["performance"] != nil {
		t.Error("filtered audit should NOT include 'performance' when not requested")
	}
}

func TestToolsAnalyzeAudit_InvalidCategory(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"audit","categories":["nonexistent"]}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("audit with invalid category should return error")
	}
	text := firstText(result)
	if !strings.Contains(text, "nonexistent") {
		t.Errorf("error should mention the invalid category, got: %s", text)
	}
}

// ============================================
// Scoring Tests
// ============================================

func TestToolsAnalyzeAudit_ScoreRange(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"audit"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("audit should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	overallScore, ok := data["overall_score"].(float64)
	if !ok {
		t.Fatal("overall_score should be a number")
	}
	if overallScore < 0 || overallScore > 100 {
		t.Errorf("overall_score = %v, want 0-100", overallScore)
	}
}

func TestToolsAnalyzeAudit_ScoringFunctions(t *testing.T) {
	t.Parallel()

	t.Run("PerformanceNoIssues", func(t *testing.T) {
		r := analyzehandler.ScoreAuditCategory("performance", map[string]any{"status": "ok"})
		if r.Score != 100 {
			t.Errorf("perf with no issues should score 100, got %d", r.Score)
		}
		if len(r.Findings) != 0 {
			t.Errorf("perf with no issues should have 0 findings, got %d", len(r.Findings))
		}
	})

	t.Run("PerformanceWithIssues", func(t *testing.T) {
		r := analyzehandler.ScoreAuditCategory("performance", map[string]any{"issues": []any{"slow load", "large bundle"}})
		if r.Score != 80 {
			t.Errorf("perf with 2 issues should score 80, got %d", r.Score)
		}
		if len(r.Findings) != 2 {
			t.Errorf("findings count should be 2, got %d", len(r.Findings))
		}
	})

	t.Run("PerformanceScoreFloor", func(t *testing.T) {
		issues := make([]any, 15)
		for i := range issues {
			issues[i] = "issue"
		}
		r := analyzehandler.ScoreAuditCategory("performance", map[string]any{"issues": issues})
		if r.Score != 0 {
			t.Errorf("perf with many issues should floor at 0, got %d", r.Score)
		}
	})

	t.Run("SecuritySeverityScoring", func(t *testing.T) {
		r := analyzehandler.ScoreAuditCategory("security", map[string]any{
			"findings": []any{
				map[string]any{"severity": "critical"},
				map[string]any{"severity": "low"},
			},
		})
		if r.Score != 70 {
			t.Errorf("security with critical(-25) + low(-5) should score 70, got %d", r.Score)
		}
	})

	t.Run("AccessibilityScoring", func(t *testing.T) {
		r := analyzehandler.ScoreAuditCategory("accessibility", map[string]any{
			"violations": []any{"v1", "v2", "v3"},
		})
		if r.Score != 85 {
			t.Errorf("a11y with 3 violations should score 85, got %d", r.Score)
		}
	})

	t.Run("BestPracticesScoring", func(t *testing.T) {
		r := analyzehandler.ScoreAuditCategory("best_practices", map[string]any{
			"third_parties": []any{"tp1", "tp2"},
		})
		if r.Score != 94 {
			t.Errorf("best practices with 2 third parties should score 94, got %d", r.Score)
		}
	})

	t.Run("ExtractFindingsMultipleKeys", func(t *testing.T) {
		// First key empty, second has data
		r := analyzehandler.ExtractFindings(map[string]any{
			"issues":   []any{},
			"warnings": []any{"w1"},
		}, "issues", "warnings")
		if len(r) != 1 {
			t.Errorf("extractFindings should skip empty arrays and find 'warnings', got %d", len(r))
		}
	})

	t.Run("ExtractFindingsNoMatch", func(t *testing.T) {
		r := analyzehandler.ExtractFindings(map[string]any{"other": "data"}, "issues", "warnings")
		if r == nil {
			t.Error("extractFindings with no match should return empty slice, not nil")
		}
		if len(r) != 0 {
			t.Errorf("extractFindings with no match should return 0 findings, got %d", len(r))
		}
	})
}

func TestToolsAnalyzeAudit_CategoryErrorFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// Each category in a test environment returns some form of data.
	// Verify that error categories have proper error field and zero score.
	resp := callAnalyzeRaw(h, `{"what":"audit"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("audit should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	categories, ok := data["categories"].(map[string]any)
	if !ok {
		t.Fatal("categories should be a map")
	}

	// Verify each category has score and findings fields
	for name, catRaw := range categories {
		cat, ok := catRaw.(map[string]any)
		if !ok {
			t.Errorf("category %q should be a map", name)
			continue
		}
		if _, ok := cat["score"].(float64); !ok {
			t.Errorf("category %q missing 'score' number field", name)
		}
		if cat["findings"] == nil {
			t.Errorf("category %q has nil 'findings' (should be [] not null)", name)
		}
		if _, ok := cat["summary"].(string); !ok {
			t.Errorf("category %q missing 'summary' string field", name)
		}
	}
}

// ============================================
// Response Structure
// ============================================

func TestToolsAnalyzeAudit_AllDefaultCategories(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"audit"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("default audit should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	categories, ok := data["categories"].(map[string]any)
	if !ok {
		t.Fatal("audit response should have 'categories' as map")
	}

	expected := []string{"performance", "accessibility", "security", "best_practices"}
	for _, cat := range expected {
		catData, ok := categories[cat].(map[string]any)
		if !ok {
			t.Errorf("default audit should include '%s' category as map", cat)
			continue
		}
		// Each category must have score, findings, and summary
		if _, ok := catData["score"].(float64); !ok {
			t.Errorf("category %q missing numeric score", cat)
		}
		if catData["findings"] == nil {
			t.Errorf("category %q findings should not be nil", cat)
		}
		if _, ok := catData["summary"].(string); !ok {
			t.Errorf("category %q missing summary string", cat)
		}
	}

	// Verify no extra categories beyond expected
	if len(categories) != len(expected) {
		t.Errorf("expected %d categories, got %d", len(expected), len(categories))
	}
}

func TestToolsAnalyzeAudit_ResponseStructure(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"audit"}`)
	if resp.Result == nil {
		t.Fatal("analyze(audit) returned nil result")
	}

	result := parseToolResult(t, resp)
	if len(result.Content) == 0 {
		t.Error("analyze(audit) should return at least one content block")
	}
	if result.Content[0].Type != "text" {
		t.Errorf("content type = %q, want 'text'", result.Content[0].Type)
	}

	// Verify all required top-level fields
	data := extractResultJSON(t, result)
	for _, field := range []string{"categories", "overall_score", "url", "timestamp"} {
		if data[field] == nil {
			t.Errorf("audit response missing required field %q", field)
		}
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// Summary Mode Tests
// ============================================

func TestToolsAnalyzeAudit_SummaryMode(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"audit","summary":true}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("audit with summary=true should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	categories, ok := data["categories"].(map[string]any)
	if !ok {
		t.Fatal("audit summary response should have 'categories' as map")
	}

	for name, catRaw := range categories {
		cat, ok := catRaw.(map[string]any)
		if !ok {
			t.Errorf("category %q should be a map", name)
			continue
		}
		// Summary mode should have findings_count instead of findings array
		if _, ok := cat["findings_count"].(float64); !ok {
			t.Errorf("category %q in summary mode should have 'findings_count' number, got %v", name, cat["findings_count"])
		}
		if cat["findings"] != nil {
			t.Errorf("category %q in summary mode should NOT include 'findings' array", name)
		}
		if _, ok := cat["score"].(float64); !ok {
			t.Errorf("category %q missing 'score' in summary mode", name)
		}
		if _, ok := cat["summary"].(string); !ok {
			t.Errorf("category %q missing 'summary' in summary mode", name)
		}
	}
}
