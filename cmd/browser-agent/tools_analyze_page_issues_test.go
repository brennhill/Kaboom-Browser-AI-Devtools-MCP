// tools_analyze_page_issues_test.go — Tests for page_issues summary builder and helpers.
// Why: Validates aggregation, severity sorting, and edge cases (empty, overflow, partial).
// Docs: docs/features/feature/auto-fix/index.md

package main

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/analyzehandler"
)

// ============================================
// Summary Builder Tests
// ============================================

func TestBuildPageIssuesSummary_Basic(t *testing.T) {
	t.Parallel()
	result := pageIssuesResult{
		TotalIssues: 5,
		BySeverity:  map[string]int{"critical": 1, "high": 2, "medium": 2},
		Sections: map[string]any{
			"console_errors": map[string]any{
				"issues": []map[string]any{
					{"severity": "high", "message": "Uncaught TypeError", "level": "error"},
					{"severity": "high", "message": "ReferenceError: x is not defined", "level": "error"},
				},
				"total": 2,
			},
			"network_failures": map[string]any{
				"issues": []map[string]any{
					{"severity": "high", "url": "/api/data", "status": 500},
				},
				"total": 1,
			},
			"accessibility": map[string]any{
				"issues": []map[string]any{
					{"severity": "critical", "rule": "color-contrast", "description": "Low contrast"},
					{"severity": "medium", "rule": "label", "description": "Missing label"},
				},
				"total": 2,
			},
		},
		ChecksCompleted: []string{"console_errors", "network_failures", "accessibility"},
		ChecksSkipped:   []string{},
		PageURL:         "http://localhost:3000",
	}

	summary := analyzehandler.BuildPageIssuesSummary(analyzehandler.PageIssuesResult{
		TotalIssues: result.TotalIssues, BySeverity: result.BySeverity,
		Sections: result.Sections, ChecksCompleted: result.ChecksCompleted,
		ChecksSkipped: result.ChecksSkipped, PageURL: result.PageURL,
		Timestamp: result.Timestamp,
	})

	if summary["total_issues"] != 5 {
		t.Errorf("total_issues = %v, want 5", summary["total_issues"])
	}

	topIssues, ok := summary["top_issues"].([]map[string]any)
	if !ok {
		t.Fatalf("top_issues wrong type: %T", summary["top_issues"])
	}
	if len(topIssues) != 5 {
		t.Fatalf("expected 5 top issues, got %d", len(topIssues))
	}
	// Critical should be first
	if topIssues[0]["severity"] != "critical" {
		t.Errorf("first top issue severity = %v, want critical", topIssues[0]["severity"])
	}

	completed, ok := summary["checks_completed"].([]string)
	if !ok {
		t.Fatalf("checks_completed wrong type: %T", summary["checks_completed"])
	}
	if len(completed) != 3 {
		t.Errorf("checks_completed = %d, want 3", len(completed))
	}
}

func TestBuildPageIssuesSummary_Empty(t *testing.T) {
	t.Parallel()
	result := pageIssuesResult{
		TotalIssues:     0,
		BySeverity:      map[string]int{},
		Sections:        map[string]any{},
		ChecksCompleted: []string{"console_errors", "network_failures", "accessibility", "security"},
		ChecksSkipped:   []string{},
		PageURL:         "http://localhost:3000",
	}

	summary := analyzehandler.BuildPageIssuesSummary(analyzehandler.PageIssuesResult{
		TotalIssues: result.TotalIssues, BySeverity: result.BySeverity,
		Sections: result.Sections, ChecksCompleted: result.ChecksCompleted,
		ChecksSkipped: result.ChecksSkipped, PageURL: result.PageURL,
		Timestamp: result.Timestamp,
	})

	if summary["total_issues"] != 0 {
		t.Errorf("total_issues = %v, want 0", summary["total_issues"])
	}
	topIssues := summary["top_issues"].([]map[string]any)
	if len(topIssues) != 0 {
		t.Errorf("expected 0 top issues, got %d", len(topIssues))
	}
}

func TestBuildPageIssuesSummary_CapsAt10(t *testing.T) {
	t.Parallel()
	issues := make([]map[string]any, 15)
	for i := range issues {
		issues[i] = map[string]any{"severity": "medium", "message": "issue"}
	}
	result := pageIssuesResult{
		TotalIssues: 15,
		BySeverity:  map[string]int{"medium": 15},
		Sections: map[string]any{
			"console_errors": map[string]any{
				"issues": issues,
				"total":  15,
			},
		},
		ChecksCompleted: []string{"console_errors"},
		ChecksSkipped:   []string{},
	}

	summary := analyzehandler.BuildPageIssuesSummary(analyzehandler.PageIssuesResult{
		TotalIssues: result.TotalIssues, BySeverity: result.BySeverity,
		Sections: result.Sections, ChecksCompleted: result.ChecksCompleted,
		ChecksSkipped: result.ChecksSkipped, PageURL: result.PageURL,
		Timestamp: result.Timestamp,
	})
	topIssues := summary["top_issues"].([]map[string]any)
	if len(topIssues) > 10 {
		t.Errorf("top_issues should be capped at 10, got %d", len(topIssues))
	}
}

func TestBuildPageIssuesSummary_WithSkippedChecks(t *testing.T) {
	t.Parallel()
	result := pageIssuesResult{
		TotalIssues: 1,
		BySeverity:  map[string]int{"high": 1},
		Sections: map[string]any{
			"console_errors": map[string]any{
				"issues": []map[string]any{
					{"severity": "high", "message": "error"},
				},
				"total": 1,
			},
		},
		ChecksCompleted: []string{"console_errors"},
		ChecksSkipped:   []string{"accessibility", "security"},
		PageURL:         "http://localhost:3000",
	}

	summary := analyzehandler.BuildPageIssuesSummary(analyzehandler.PageIssuesResult{
		TotalIssues: result.TotalIssues, BySeverity: result.BySeverity,
		Sections: result.Sections, ChecksCompleted: result.ChecksCompleted,
		ChecksSkipped: result.ChecksSkipped, PageURL: result.PageURL,
		Timestamp: result.Timestamp,
	})

	skipped, ok := summary["checks_skipped"].([]string)
	if !ok {
		t.Fatalf("checks_skipped wrong type: %T", summary["checks_skipped"])
	}
	if len(skipped) != 2 {
		t.Errorf("checks_skipped = %d, want 2", len(skipped))
	}
}

func TestBuildPageIssuesSummary_SectionWithError(t *testing.T) {
	t.Parallel()
	result := pageIssuesResult{
		TotalIssues: 0,
		BySeverity:  map[string]int{},
		Sections: map[string]any{
			"accessibility": map[string]any{
				"issues": []map[string]any{},
				"total":  0,
				"error":  "extension disconnected",
			},
		},
		ChecksCompleted: []string{"accessibility"},
		ChecksSkipped:   []string{},
	}

	summary := analyzehandler.BuildPageIssuesSummary(analyzehandler.PageIssuesResult{
		TotalIssues: result.TotalIssues, BySeverity: result.BySeverity,
		Sections: result.Sections, ChecksCompleted: result.ChecksCompleted,
		ChecksSkipped: result.ChecksSkipped, PageURL: result.PageURL,
		Timestamp: result.Timestamp,
	})

	sections, ok := summary["sections"].(map[string]any)
	if !ok {
		t.Fatalf("sections wrong type: %T", summary["sections"])
	}
	a11y, ok := sections["accessibility"].(map[string]any)
	if !ok {
		t.Fatalf("accessibility section wrong type: %T", sections["accessibility"])
	}
	if a11y["error"] != "extension disconnected" {
		t.Errorf("error = %v, want 'extension disconnected'", a11y["error"])
	}
}

// ============================================
// Helper Tests
// ============================================

func TestDefaultCategories_AllWhenEmpty(t *testing.T) {
	t.Parallel()
	cats := defaultCategories(nil)
	expected := []string{"console_errors", "network_failures", "accessibility", "security"}
	for _, e := range expected {
		if !cats[e] {
			t.Errorf("missing default category: %s", e)
		}
	}
}

func TestDefaultCategories_OnlyRequested(t *testing.T) {
	t.Parallel()
	cats := defaultCategories([]string{"console_errors", "security"})
	if !cats["console_errors"] || !cats["security"] {
		t.Error("requested categories should be present")
	}
	if cats["accessibility"] || cats["network_failures"] {
		t.Error("unrequested categories should be absent")
	}
}

func TestMapA11yImpact(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input, want string
	}{
		{"critical", "critical"},
		{"serious", "high"},
		{"moderate", "medium"},
		{"minor", "low"},
		{"", "info"},
		{"unknown", "info"},
	}
	for _, tc := range tests {
		got := mapA11yImpact(tc.input)
		if got != tc.want {
			t.Errorf("mapA11yImpact(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestExtractIssueMessage_PriorityOrder(t *testing.T) {
	t.Parallel()
	// message takes priority
	issue := map[string]any{"message": "msg", "title": "ttl", "url": "/foo"}
	if got := extractIssueMessage(issue); got != "msg" {
		t.Errorf("got %q, want %q", got, "msg")
	}

	// title if no message
	issue2 := map[string]any{"title": "ttl", "url": "/foo"}
	if got := extractIssueMessage(issue2); got != "ttl" {
		t.Errorf("got %q, want %q", got, "ttl")
	}

	// empty map
	issue3 := map[string]any{}
	if got := extractIssueMessage(issue3); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
