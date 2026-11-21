// a11yaudit_test.go — Tests the accessibility audit use case and its summary projection.
// Docs: docs/features/feature/observe/index.md

package a11yaudit

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestBuildA11ySummary_Compact(t *testing.T) {
	t.Parallel()
	auditResult := map[string]any{
		"passes": []any{
			map[string]any{"id": "rule1"},
			map[string]any{"id": "rule2"},
		},
		"violations": []any{
			map[string]any{
				"id":     "color-contrast",
				"impact": "serious",
				"nodes":  []any{map[string]any{}, map[string]any{}, map[string]any{}},
			},
			map[string]any{
				"id":     "image-alt",
				"impact": "critical",
				"nodes":  []any{map[string]any{}},
			},
		},
		"incomplete": []any{
			map[string]any{"id": "aria-label"},
		},
	}

	result := buildSummary(auditResult)

	if result["pass"] != 2 {
		t.Errorf("pass = %v, want 2", result["pass"])
	}
	if result["violations"] != 2 {
		t.Errorf("violations = %v, want 2", result["violations"])
	}
	if result["incomplete"] != 1 {
		t.Errorf("incomplete = %v, want 1", result["incomplete"])
	}

	topIssues, ok := result["top_issues"].([]map[string]any)
	if !ok {
		t.Fatalf("top_issues wrong type: %T", result["top_issues"])
	}
	if len(topIssues) != 2 {
		t.Fatalf("expected 2 top issues, got %d", len(topIssues))
	}
	// Should be sorted by node count descending
	if topIssues[0]["rule"] != "color-contrast" {
		t.Errorf("first issue = %v, want color-contrast", topIssues[0]["rule"])
	}
	if topIssues[0]["count"] != 3 {
		t.Errorf("first issue count = %v, want 3", topIssues[0]["count"])
	}
	if topIssues[0]["severity"] != "serious" {
		t.Errorf("first issue severity = %v, want serious", topIssues[0]["severity"])
	}
}

func TestBuildA11ySummary_Empty(t *testing.T) {
	t.Parallel()
	result := buildSummary(map[string]any{})
	if result["pass"] != 0 {
		t.Errorf("pass = %v, want 0", result["pass"])
	}
	if result["violations"] != 0 {
		t.Errorf("violations = %v, want 0", result["violations"])
	}
}

func TestBuildA11ySummary_TopIssuesLimitedTo5(t *testing.T) {
	t.Parallel()
	violations := make([]any, 7)
	for i := range violations {
		violations[i] = map[string]any{
			"id":     "rule-" + string(rune('a'+i)),
			"impact": "minor",
			"nodes":  []any{map[string]any{}},
		}
	}
	auditResult := map[string]any{"violations": violations}

	result := buildSummary(auditResult)
	topIssues := result["top_issues"].([]map[string]any)
	if len(topIssues) != 5 {
		t.Errorf("expected 5 top issues (capped), got %d", len(topIssues))
	}
}

// ============================================
// Issue #276: A11y Audit Partial Results Tests
// ============================================

func TestAuditGatesTrackingAndReturnsPartialOnExecuteError(t *testing.T) {
	t.Parallel()
	if _, err := New(Deps{TrackingEnabled: func() bool { return false }}).Audit(Options{}); err == nil {
		t.Fatal("untracked audit succeeded")
	}
	service := New(Deps{
		TrackingEnabled: func() bool { return true },
		Execute: func(string, []string, any, bool) (json.RawMessage, error) {
			return nil, errors.New("Axe is already running")
		},
	})
	result, err := service.Audit(Options{})
	if err != nil || result.PartialError == "" || result.Data["partial"] != true {
		t.Fatalf("partial result = %+v, err = %v", result, err)
	}
}

func TestAuditRejectsMalformedResultAndSummarizes(t *testing.T) {
	t.Parallel()
	payload := `{"violations":[{"id":"r","impact":"serious","nodes":[1,2]}],"passes":[{}]}`
	service := New(Deps{
		TrackingEnabled: func() bool { return true },
		Execute:         func(string, []string, any, bool) (json.RawMessage, error) { return json.RawMessage(payload), nil },
	})
	result, err := service.Audit(Options{Summary: true})
	if err != nil || result.Data["violations"] != 1 || result.Data["pass"] != 1 {
		t.Fatalf("summary = %+v, err = %v", result.Data, err)
	}
	bad := New(Deps{
		TrackingEnabled: func() bool { return true },
		Execute:         func(string, []string, any, bool) (json.RawMessage, error) { return json.RawMessage(`{bad`), nil },
	})
	var failure *Failure
	if _, err := bad.Audit(Options{}); !errors.As(err, &failure) || failure.Kind != KindInvalidResult {
		t.Fatalf("malformed audit error = %v", err)
	}
}
