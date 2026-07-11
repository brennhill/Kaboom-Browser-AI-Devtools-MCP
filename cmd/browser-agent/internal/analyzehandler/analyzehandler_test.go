// analyzehandler_test.go — Unit tests for the analyzehandler sub-package exported API.

package analyzehandler

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
)

// ---------------------------------------------------------------------------
// ScoreAuditCategory
// ---------------------------------------------------------------------------

func TestScoreAuditCategory_Performance(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string]any
		wantScore int
		wantEmpty bool
	}{
		{
			name:      "no issues",
			data:      map[string]any{},
			wantScore: 100,
			wantEmpty: true,
		},
		{
			name:      "3 issues = 70",
			data:      map[string]any{"issues": []any{"slow load", "large bundle", "unoptimized images"}},
			wantScore: 70,
			wantEmpty: false,
		},
		{
			name:      "11+ issues floors at 0",
			data:      map[string]any{"issues": make([]any, 15)},
			wantScore: 0,
			wantEmpty: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ScoreAuditCategory("performance", tt.data)
			if result.Score != tt.wantScore {
				t.Errorf("Score: want %d, got %d", tt.wantScore, result.Score)
			}
			if tt.wantEmpty && len(result.Findings) != 0 {
				t.Errorf("expected empty findings, got %d", len(result.Findings))
			}
		})
	}
}

func TestScoreAuditCategory_Accessibility(t *testing.T) {
	data := map[string]any{
		"violations": []any{"missing-alt", "low-contrast", "no-label"},
	}
	result := ScoreAuditCategory("accessibility", data)
	// 3 violations * 5 = 15, so 100-15 = 85
	if result.Score != 85 {
		t.Errorf("Score: want 85, got %d", result.Score)
	}
	if len(result.Findings) != 3 {
		t.Errorf("Findings count: want 3, got %d", len(result.Findings))
	}
}

func TestScoreAuditCategory_Security(t *testing.T) {
	data := map[string]any{
		"findings": []any{
			map[string]any{"severity": "critical"},
			map[string]any{"severity": "high"},
			map[string]any{"severity": "low"},
		},
	}
	result := ScoreAuditCategory("security", data)
	// 100 - 25 (critical) - 15 (high) - 5 (low) = 55
	if result.Score != 55 {
		t.Errorf("Score: want 55, got %d", result.Score)
	}
}

func TestScoreAuditCategory_Security_NonMapFinding(t *testing.T) {
	data := map[string]any{
		"findings": []any{"string-finding", "another"},
	}
	result := ScoreAuditCategory("security", data)
	// 2 non-map findings * 5 = 10 penalty
	if result.Score != 90 {
		t.Errorf("Score: want 90, got %d", result.Score)
	}
}

func TestScoreAuditCategory_BestPractices(t *testing.T) {
	data := map[string]any{
		"third_parties": []any{"lib1", "lib2"},
	}
	result := ScoreAuditCategory("best_practices", data)
	// 2 * 3 = 6, so 100-6 = 94
	if result.Score != 94 {
		t.Errorf("Score: want 94, got %d", result.Score)
	}
}

func TestScoreAuditCategory_Unknown(t *testing.T) {
	result := ScoreAuditCategory("unknown_category", map[string]any{})
	if result.Score != 100 {
		t.Errorf("unknown category should default to 100, got %d", result.Score)
	}
}

// ---------------------------------------------------------------------------
// ExtractFindings
// ---------------------------------------------------------------------------

func TestExtractFindings(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		keys []string
		want int
	}{
		{"first key matches", map[string]any{"issues": []any{"a", "b"}}, []string{"issues"}, 2},
		{"second key matches", map[string]any{"warnings": []any{"a"}}, []string{"issues", "warnings"}, 1},
		{"no keys match", map[string]any{"other": "val"}, []string{"issues"}, 0},
		{"empty array", map[string]any{"issues": []any{}}, []string{"issues"}, 0},
		{"nil data", map[string]any{}, []string{"issues"}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractFindings(tt.data, tt.keys...)
			if len(got) != tt.want {
				t.Errorf("len(findings) = %d, want %d", len(got), tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BuildPageIssuesSummary
// ---------------------------------------------------------------------------

func TestBuildPageIssuesSummary_Empty(t *testing.T) {
	result := PageIssuesResult{
		TotalIssues:     0,
		BySeverity:      map[string]int{},
		Sections:        map[string]any{},
		ChecksCompleted: []string{"a11y"},
		ChecksSkipped:   []string{"perf"},
		PageURL:         "https://example.com",
	}
	summary := BuildPageIssuesSummary(result)

	if summary["total_issues"] != 0 {
		t.Errorf("total_issues: want 0, got %v", summary["total_issues"])
	}
	topIssues, ok := summary["top_issues"].([]map[string]any)
	if !ok {
		t.Fatal("top_issues should be []map[string]any")
	}
	if len(topIssues) != 0 {
		t.Errorf("top_issues length: want 0, got %d", len(topIssues))
	}
}

func TestBuildPageIssuesSummary_WithIssues(t *testing.T) {
	result := PageIssuesResult{
		TotalIssues: 3,
		BySeverity:  map[string]int{"critical": 1, "low": 2},
		Sections: map[string]any{
			"accessibility": map[string]any{
				"total": 3,
				"issues": []map[string]any{
					{"severity": "critical", "message": "Missing alt text"},
					{"severity": "low", "message": "Color contrast"},
					{"severity": "low", "message": "Heading order"},
				},
			},
		},
		ChecksCompleted: []string{"accessibility"},
		PageURL:         "https://example.com",
	}

	summary := BuildPageIssuesSummary(result)
	topIssues, ok := summary["top_issues"].([]map[string]any)
	if !ok {
		t.Fatal("top_issues should be []map[string]any")
	}
	if len(topIssues) != 3 {
		t.Errorf("top_issues: want 3, got %d", len(topIssues))
	}
	// Critical issue should be first (sorted by severity).
	if topIssues[0]["severity"] != "critical" {
		t.Errorf("first issue should be critical, got %v", topIssues[0]["severity"])
	}
}

func TestBuildPageIssuesSummary_SectionErrors(t *testing.T) {
	result := PageIssuesResult{
		Sections: map[string]any{
			"security": map[string]any{
				"total": 0,
				"error": "scan timed out",
			},
		},
	}
	summary := BuildPageIssuesSummary(result)
	sections, ok := summary["sections"].(map[string]any)
	if !ok {
		t.Fatal("sections should be map")
	}
	sec, ok := sections["security"].(map[string]any)
	if !ok {
		t.Fatal("security section should be map")
	}
	if sec["error"] != "scan timed out" {
		t.Errorf("error should be propagated, got %v", sec["error"])
	}
}

// ---------------------------------------------------------------------------
// SeverityOrder
// ---------------------------------------------------------------------------

func TestSeverityOrder(t *testing.T) {
	if SeverityOrder["critical"] >= SeverityOrder["high"] {
		t.Error("critical should sort before high")
	}
	if SeverityOrder["high"] >= SeverityOrder["medium"] {
		t.Error("high should sort before medium")
	}
	if SeverityOrder["medium"] >= SeverityOrder["low"] {
		t.Error("medium should sort before low")
	}
	if SeverityOrder["low"] >= SeverityOrder["info"] {
		t.Error("low should sort before info")
	}
}

// ---------------------------------------------------------------------------
// AnnotationProjectBaseURL
// ---------------------------------------------------------------------------

func TestAnnotationProjectBaseURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://example.com/page/1?q=foo", "https://example.com"},
		{"http://localhost:3000/dashboard", "http://localhost:3000"},
		{"", ""},
		{"  ", ""},
		{"not-a-url", "not-a-url"},
		{"https://api.example.com:8443/v2/resource", "https://api.example.com:8443"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := AnnotationProjectBaseURL(tt.input)
			if got != tt.want {
				t.Errorf("AnnotationProjectBaseURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BuildProjectSummaries
// ---------------------------------------------------------------------------

func TestBuildProjectSummaries_NilPages(t *testing.T) {
	summaries := BuildProjectSummaries(nil)
	if summaries != nil {
		t.Errorf("expected nil for no pages, got %v", summaries)
	}
}

func TestBuildProjectSummaries_SingleProject(t *testing.T) {
	pages := []*annotation.Session{
		{PageURL: "https://example.com/page1", Annotations: []annotation.Annotation{{ID: "a1"}, {ID: "a2"}}},
		{PageURL: "https://example.com/page2", Annotations: []annotation.Annotation{{ID: "a3"}}},
	}
	summaries := BuildProjectSummaries(pages)
	if len(summaries) != 1 {
		t.Fatalf("expected 1 project, got %d", len(summaries))
	}
	if summaries[0]["base_url"] != "https://example.com" {
		t.Errorf("base_url: want https://example.com, got %v", summaries[0]["base_url"])
	}
	if summaries[0]["annotation_count"] != 3 {
		t.Errorf("annotation_count: want 3, got %v", summaries[0]["annotation_count"])
	}
	if summaries[0]["page_count"] != 2 {
		t.Errorf("page_count: want 2, got %v", summaries[0]["page_count"])
	}
}

func TestBuildProjectSummaries_MultipleProjects(t *testing.T) {
	pages := []*annotation.Session{
		{PageURL: "https://example.com/a", Annotations: []annotation.Annotation{{ID: "a1"}}},
		{PageURL: "https://other.com/b", Annotations: []annotation.Annotation{{ID: "a2"}}},
	}
	summaries := BuildProjectSummaries(pages)
	if len(summaries) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(summaries))
	}
}

// ---------------------------------------------------------------------------
// BuildScopeWarning
// ---------------------------------------------------------------------------

func TestBuildScopeWarning(t *testing.T) {
	projects := []map[string]any{
		{"base_url": "https://a.com", "recommended_filter": "https://a.com/*"},
		{"base_url": "https://b.com", "recommended_filter": "https://b.com/*"},
	}
	warning := BuildScopeWarning(projects)
	if _, ok := warning["warning"]; !ok {
		t.Error("should contain 'warning' key")
	}
	filters, ok := warning["suggested_filters"].([]string)
	if !ok || len(filters) != 2 {
		t.Errorf("should have 2 suggested filters, got %v", warning["suggested_filters"])
	}
}

// ---------------------------------------------------------------------------
// BuildSessionHints
// ---------------------------------------------------------------------------

func TestBuildSessionHints(t *testing.T) {
	hints := BuildSessionHints("")
	if _, ok := hints["checklist"]; !ok {
		t.Error("should always have checklist")
	}
	if _, ok := hints["screenshot_baseline"]; ok {
		t.Error("should not have screenshot_baseline when path is empty")
	}

	hints = BuildSessionHints("/tmp/screenshot.png")
	if _, ok := hints["screenshot_baseline"]; !ok {
		t.Error("should have screenshot_baseline when path is provided")
	}
}

// ---------------------------------------------------------------------------
// BuildDetailHints
// ---------------------------------------------------------------------------

func TestBuildDetailHints(t *testing.T) {
	tests := []struct {
		name           string
		css            string
		js             string
		a11y           []string
		errors         bool
		wantNil        bool
		wantDesignKey  bool
		wantRuntimeKey bool
		wantA11yKey    bool
		wantErrorKey   bool
	}{
		{"all empty", "", "", nil, false, true, false, false, false, false},
		{"tailwind", "tailwind", "", nil, false, false, true, false, false, false},
		{"bootstrap", "bootstrap", "", nil, false, false, true, false, false, false},
		{"react", "", "react", nil, false, false, false, true, false, false},
		{"a11y flags", "", "", []string{"low-contrast"}, false, false, false, false, true, false},
		{"correlated errors", "", "", nil, true, false, false, false, false, true},
		{"all set", "tailwind", "vue", []string{"flag"}, true, false, true, true, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hints := BuildDetailHints(tt.css, tt.js, tt.a11y, tt.errors)
			if tt.wantNil {
				if hints != nil {
					t.Errorf("expected nil, got %v", hints)
				}
				return
			}
			if hints == nil {
				t.Fatal("expected non-nil hints")
			}
			_, hasDesign := hints["design_system"]
			_, hasRuntime := hints["runtime_framework"]
			_, hasA11y := hints["accessibility"]
			_, hasError := hints["error_context"]
			if hasDesign != tt.wantDesignKey {
				t.Errorf("design_system: present=%v, want=%v", hasDesign, tt.wantDesignKey)
			}
			if hasRuntime != tt.wantRuntimeKey {
				t.Errorf("runtime_framework: present=%v, want=%v", hasRuntime, tt.wantRuntimeKey)
			}
			if hasA11y != tt.wantA11yKey {
				t.Errorf("accessibility: present=%v, want=%v", hasA11y, tt.wantA11yKey)
			}
			if hasError != tt.wantErrorKey {
				t.Errorf("error_context: present=%v, want=%v", hasError, tt.wantErrorKey)
			}
		})
	}
}
