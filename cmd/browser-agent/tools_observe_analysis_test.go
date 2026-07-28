// Purpose: Tests for observe analysis sub-handler dispatch.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_observe_analysis_test.go — Tests for a11ysummary.EnsureAuditSummary function.
// Note: executeA11yQuery is tested indirectly (requires live capture + extension).
package main

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/asynccommand"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/a11ysummary"
)

// ============================================
// a11ysummary.EnsureAuditSummary
// ============================================

func TestEnsureA11ySummary_AddsSummaryWhenMissing(t *testing.T) {
	t.Parallel()
	auditResult := map[string]any{
		"violations": []any{
			map[string]any{"id": "color-contrast"},
			map[string]any{"id": "image-alt"},
		},
		"passes": []any{
			map[string]any{"id": "html-has-lang"},
		},
	}

	a11ysummary.EnsureAuditSummary(auditResult)

	summary, ok := auditResult["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary to be map, got %T", auditResult["summary"])
	}
	if summary["violations"] != 2 {
		t.Errorf("expected violations 2, got %v", summary["violations"])
	}
	if summary["passes"] != 1 {
		t.Errorf("expected passes 1, got %v", summary["passes"])
	}
}

func TestEnsureA11ySummary_PreservesExistingSummary(t *testing.T) {
	t.Parallel()
	existingSummary := map[string]any{
		"violations":   99,
		"passes":       88,
		"custom_field": "preserved",
	}
	auditResult := map[string]any{
		"violations": []any{
			map[string]any{"id": "color-contrast"},
		},
		"passes":  []any{},
		"summary": existingSummary,
	}

	a11ysummary.EnsureAuditSummary(auditResult)

	summary, ok := auditResult["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary to be map, got %T", auditResult["summary"])
	}
	// Canonical values and unrelated metadata are preserved.
	if summary["violations"] != 99 {
		t.Errorf("expected existing violations 99 to be preserved, got %v", summary["violations"])
	}
	if summary["passes"] != 88 {
		t.Errorf("expected existing passes 88 to be preserved, got %v", summary["passes"])
	}
	if summary["custom_field"] != "preserved" {
		t.Errorf("expected custom_field to be preserved, got %v", summary["custom_field"])
	}
}

func TestEnsureA11ySummary_NoViolationsNoPassses(t *testing.T) {
	t.Parallel()
	auditResult := map[string]any{}

	a11ysummary.EnsureAuditSummary(auditResult)

	summary, ok := auditResult["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary to be map, got %T", auditResult["summary"])
	}
	if summary["violations"] != 0 {
		t.Errorf("expected violations 0, got %v", summary["violations"])
	}
	if summary["passes"] != 0 {
		t.Errorf("expected passes 0, got %v", summary["passes"])
	}
}

func TestEnsureA11ySummary_ViolationsNotArray(t *testing.T) {
	t.Parallel()
	auditResult := map[string]any{
		"violations": "not an array",
		"passes":     42,
	}

	a11ysummary.EnsureAuditSummary(auditResult)

	summary, ok := auditResult["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary to be map, got %T", auditResult["summary"])
	}
	// Type assertion fails gracefully, len of nil slice is 0
	if summary["violations"] != 0 {
		t.Errorf("expected violations 0 for non-array violations, got %v", summary["violations"])
	}
	if summary["passes"] != 0 {
		t.Errorf("expected passes 0 for non-array passes, got %v", summary["passes"])
	}
}

func TestEnsureA11ySummary_OnlyViolations(t *testing.T) {
	t.Parallel()
	auditResult := map[string]any{
		"violations": []any{
			map[string]any{"id": "v1"},
			map[string]any{"id": "v2"},
			map[string]any{"id": "v3"},
		},
	}

	a11ysummary.EnsureAuditSummary(auditResult)

	summary, ok := auditResult["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary to be map, got %T", auditResult["summary"])
	}
	if summary["violations"] != 3 {
		t.Errorf("expected violations 3, got %v", summary["violations"])
	}
	if summary["passes"] != 0 {
		t.Errorf("expected passes 0 when passes key missing, got %v", summary["passes"])
	}
}

func TestEnsureA11ySummary_OnlyPasses(t *testing.T) {
	t.Parallel()
	auditResult := map[string]any{
		"passes": []any{
			map[string]any{"id": "p1"},
			map[string]any{"id": "p2"},
		},
	}

	a11ysummary.EnsureAuditSummary(auditResult)

	summary, ok := auditResult["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary to be map, got %T", auditResult["summary"])
	}
	if summary["violations"] != 0 {
		t.Errorf("expected violations 0 when violations key missing, got %v", summary["violations"])
	}
	if summary["passes"] != 2 {
		t.Errorf("expected passes 2, got %v", summary["passes"])
	}
}

func TestEnsureA11ySummary_EmptyArrays(t *testing.T) {
	t.Parallel()
	auditResult := map[string]any{
		"violations": []any{},
		"passes":     []any{},
	}

	a11ysummary.EnsureAuditSummary(auditResult)

	summary, ok := auditResult["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary to be map, got %T", auditResult["summary"])
	}
	if summary["violations"] != 0 {
		t.Errorf("expected violations 0, got %v", summary["violations"])
	}
	if summary["passes"] != 0 {
		t.Errorf("expected passes 0, got %v", summary["passes"])
	}
}

func TestEnsureA11ySummary_NilViolationsKey(t *testing.T) {
	t.Parallel()
	auditResult := map[string]any{
		"violations": nil,
		"passes":     nil,
	}

	a11ysummary.EnsureAuditSummary(auditResult)

	summary, ok := auditResult["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary to be map, got %T", auditResult["summary"])
	}
	if summary["violations"] != 0 {
		t.Errorf("expected violations 0 for nil violations, got %v", summary["violations"])
	}
	if summary["passes"] != 0 {
		t.Errorf("expected passes 0 for nil passes, got %v", summary["passes"])
	}
}

func TestBuildA11yQueryParams_IncludesFrameWhenProvided(t *testing.T) {
	t.Parallel()

	params := asynccommand.BuildA11yQueryParams("#app", []string{"wcag2a"}, "iframe.editor", false)

	if got := params["scope"]; got != "#app" {
		t.Fatalf("scope = %v, want #app", got)
	}
	if _, ok := params["tags"]; !ok {
		t.Fatal("expected tags in query params")
	}
	if got := params["frame"]; got != "iframe.editor" {
		t.Fatalf("frame = %v, want iframe.editor", got)
	}
}

func TestBuildA11yQueryParams_OmitsFrameWhenNil(t *testing.T) {
	t.Parallel()

	params := asynccommand.BuildA11yQueryParams("", nil, nil, false)

	if _, ok := params["frame"]; ok {
		t.Fatal("frame should be omitted when nil")
	}
	if _, ok := params["scope"]; ok {
		t.Fatal("scope should be omitted when empty")
	}
	if _, ok := params["tags"]; ok {
		t.Fatal("tags should be omitted when empty")
	}
}

func TestBuildA11yQueryParams_IncludesForceRefreshWhenTrue(t *testing.T) {
	t.Parallel()

	params := asynccommand.BuildA11yQueryParams("#app", nil, nil, true)
	if got, ok := params["force_refresh"].(bool); !ok || !got {
		t.Fatalf("force_refresh = %v, want true", params["force_refresh"])
	}
}
