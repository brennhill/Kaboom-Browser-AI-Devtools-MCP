// summary_test.go — Canonical accessibility summary contract tests.
package a11ysummary

import (
	"encoding/json"
	"testing"
)

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

func TestEnsureAuditSummary_NormalizesEverySupportedNumericWireType(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		value any
		want  int
	}{
		{"int", int(1), 1},
		{"int8", int8(2), 2},
		{"int16", int16(3), 3},
		{"int32", int32(4), 4},
		{"int64", int64(5), 5},
		{"uint", uint(6), 6},
		{"uint8", uint8(7), 7},
		{"uint16", uint16(8), 8},
		{"uint32", uint32(9), 9},
		{"uint64", uint64(10), 10},
		{"float32", float32(11), 11},
		{"float64", float64(12), 12},
		{"json_number", json.Number("13"), 13},
		{"numeric_string", "14", 14},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			audit := map[string]any{"summary": map[string]any{"violations": tc.value}}
			EnsureAuditSummary(audit)
			if got := requireSummary(t, audit)["violations"]; got != tc.want {
				t.Fatalf("violations = %v, want %d", got, tc.want)
			}
		})
	}
}

func TestEnsureAuditSummary_FallsBackForMalformedCountsAndArrays(t *testing.T) {
	t.Parallel()
	audit := map[string]any{
		"violations": "not-an-array",
		"passes":     []any{1, 2},
		"summary": map[string]any{
			"violations":   json.Number("invalid"),
			"passes":       "invalid",
			"incomplete":   struct{}{},
			"inapplicable": nil,
		},
	}
	EnsureAuditSummary(audit)
	summary := requireSummary(t, audit)
	if summary["violations"] != 0 || summary["passes"] != 2 ||
		summary["incomplete"] != 0 || summary["inapplicable"] != 0 {
		t.Fatalf("fallback summary = %#v", summary)
	}
}

func TestEnsureAuditSummary_NilInputIsNoop(t *testing.T) {
	t.Parallel()
	EnsureAuditSummary(nil)
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
