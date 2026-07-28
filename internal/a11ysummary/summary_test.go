// summary_test.go — Canonical accessibility summary contract tests.
package a11ysummary

import "testing"

func TestBuildSummary_ExposesCanonicalKeysOnly(t *testing.T) {
	t.Parallel()
	got := BuildSummary(Counts{Violations: 1, Passes: 2, Incomplete: 3, Inapplicable: 4})

	want := map[string]int{"violations": 1, "passes": 2, "incomplete": 3, "inapplicable": 4}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("%s = %v, want %d", key, got[key], value)
		}
	}
	assertNoLegacyCounts(t, got)
}

func TestEnsureAuditSummary_AddsSummaryFromArrays(t *testing.T) {
	t.Parallel()
	audit := map[string]any{
		"violations":   []any{1, 2},
		"passes":       []any{1},
		"incomplete":   []any{1, 2, 3},
		"inapplicable": []any{},
	}

	EnsureAuditSummary(audit)
	summary := requireSummary(t, audit)
	if summary["violations"] != 2 || summary["passes"] != 1 ||
		summary["incomplete"] != 3 || summary["inapplicable"] != 0 {
		t.Fatalf("unexpected derived summary: %+v", summary)
	}
	assertNoLegacyCounts(t, summary)
}

func TestEnsureAuditSummary_PreservesCanonicalValuesAndMetadata(t *testing.T) {
	t.Parallel()
	audit := map[string]any{
		"violations": []any{1},
		"passes":     []any{1, 2},
		"summary": map[string]any{
			"violations": 9,
			"passes":     8,
			"custom":     "keep",
		},
	}

	EnsureAuditSummary(audit)
	summary := requireSummary(t, audit)
	if summary["violations"] != 9 || summary["passes"] != 8 || summary["custom"] != "keep" {
		t.Fatalf("canonical summary was not preserved: %+v", summary)
	}
	assertNoLegacyCounts(t, summary)
}

func TestEnsureAuditSummary_DropsLegacyInputFields(t *testing.T) {
	t.Parallel()
	audit := map[string]any{
		"violations": []any{1},
		"passes":     []any{1, 2},
		"summary": map[string]any{
			"violation_count": 99,
			"pass_count":      88,
			"custom":          "keep",
		},
	}

	EnsureAuditSummary(audit)
	summary := requireSummary(t, audit)
	if summary["violations"] != 1 || summary["passes"] != 2 || summary["custom"] != "keep" {
		t.Fatalf("legacy fields affected canonical fallback: %+v", summary)
	}
	assertNoLegacyCounts(t, summary)
}

func TestEnsureAuditSummary_RebuildsInvalidSummaryType(t *testing.T) {
	t.Parallel()
	audit := map[string]any{"violations": []any{1, 2, 3}, "summary": "bad"}

	EnsureAuditSummary(audit)
	summary := requireSummary(t, audit)
	if summary["violations"] != 3 {
		t.Fatalf("violations = %v, want 3", summary["violations"])
	}
	assertNoLegacyCounts(t, summary)
}

func requireSummary(t *testing.T, audit map[string]any) map[string]any {
	t.Helper()
	summary, ok := audit["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary map, got %T", audit["summary"])
	}
	return summary
}

func assertNoLegacyCounts(t *testing.T, summary map[string]any) {
	t.Helper()
	for _, key := range []string{"violation_count", "pass_count", "incomplete_count", "inapplicable_count"} {
		if _, exists := summary[key]; exists {
			t.Fatalf("compatibility field %q must not be emitted: %+v", key, summary)
		}
	}
}
