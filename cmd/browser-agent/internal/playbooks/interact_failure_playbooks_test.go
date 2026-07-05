// interact_failure_playbooks_test.go -- Tests for interact-failure recovery
// playbook lookup, normalization, and tutorial serialization. Pure logic; no I/O.

package playbooks

import "testing"

func TestNormalizeInteractFailureCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"exact", "element_not_found", "element_not_found"},
		{"uppercase", "AMBIGUOUS_TARGET", "ambiguous_target"},
		{"trimmed", "  stale_element_id  ", "stale_element_id"},
		{"error prefix contains", "error=scope_not_found", "scope_not_found"},
		{"embedded in sentence", "the element was blocked_by_overlay here", "blocked_by_overlay"},
		{"empty", "", ""},
		{"whitespace only", "   ", ""},
		{"unknown", "totally_unknown_code", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeInteractFailureCode(tc.in); got != tc.want {
				t.Fatalf("NormalizeInteractFailureCode(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestLookupInteractFailurePlaybook(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		in       string
		wantCode string
		wantOK   bool
	}{
		{"exact element_not_found", "element_not_found", "element_not_found", true},
		{"error-prefixed ambiguous", "error=ambiguous_target", "ambiguous_target", true},
		{"uppercase stale", "STALE_ELEMENT_ID", "stale_element_id", true},
		{"unknown", "no_such_code", "", false},
		{"empty", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, pb, ok := LookupInteractFailurePlaybook(tc.in)
			if ok != tc.wantOK {
				t.Fatalf("LookupInteractFailurePlaybook(%q) ok = %v, want %v", tc.in, ok, tc.wantOK)
			}
			if code != tc.wantCode {
				t.Fatalf("LookupInteractFailurePlaybook(%q) code = %q, want %q", tc.in, code, tc.wantCode)
			}
			if tc.wantOK {
				if pb.DetectionSignal == "" {
					t.Fatalf("expected non-empty DetectionSignal for %q", tc.in)
				}
				if len(pb.OrderedRecoverySteps) == 0 {
					t.Fatalf("expected recovery steps for %q", tc.in)
				}
				if pb.StopAndReportCondition == "" {
					t.Fatalf("expected StopAndReportCondition for %q", tc.in)
				}
				if pb.RetrySuggestion == "" {
					t.Fatalf("expected RetrySuggestion for %q", tc.in)
				}
			} else {
				if pb.DetectionSignal != "" || len(pb.OrderedRecoverySteps) != 0 {
					t.Fatalf("expected zero-value playbook for miss %q", tc.in)
				}
			}
		})
	}
}

func TestTutorialFailureRecoveryPlaybooks(t *testing.T) {
	t.Parallel()
	out := TutorialFailureRecoveryPlaybooks()

	if len(out) != len(InteractFailurePlaybooks) {
		t.Fatalf("got %d playbooks, want %d", len(out), len(InteractFailurePlaybooks))
	}

	for code, src := range InteractFailurePlaybooks {
		raw, ok := out[code]
		if !ok {
			t.Fatalf("missing code %q in tutorial output", code)
		}
		m, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("code %q value is %T, want map[string]any", code, raw)
		}
		if m["detection_signal"] != src.DetectionSignal {
			t.Fatalf("code %q detection_signal = %v, want %v", code, m["detection_signal"], src.DetectionSignal)
		}
		if m["stop_and_report_condition"] != src.StopAndReportCondition {
			t.Fatalf("code %q stop_and_report_condition mismatch", code)
		}
		if m["retry_guidance"] != src.RetrySuggestion {
			t.Fatalf("code %q retry_guidance mismatch", code)
		}
		steps, ok := m["ordered_recovery_steps"].([]string)
		if !ok {
			t.Fatalf("code %q ordered_recovery_steps is %T, want []string", code, m["ordered_recovery_steps"])
		}
		if len(steps) != len(src.OrderedRecoverySteps) {
			t.Fatalf("code %q has %d steps, want %d", code, len(steps), len(src.OrderedRecoverySteps))
		}
	}
}

// TestInteractFailurePlaybooks_AllWellFormed guards against regressions where a
// new playbook is added without all four required fields populated.
func TestInteractFailurePlaybooks_AllWellFormed(t *testing.T) {
	t.Parallel()
	for code, pb := range InteractFailurePlaybooks {
		if code == "" {
			t.Fatal("empty playbook code")
		}
		if pb.DetectionSignal == "" {
			t.Fatalf("%q: empty DetectionSignal", code)
		}
		if len(pb.OrderedRecoverySteps) == 0 {
			t.Fatalf("%q: no OrderedRecoverySteps", code)
		}
		if pb.StopAndReportCondition == "" {
			t.Fatalf("%q: empty StopAndReportCondition", code)
		}
		if pb.RetrySuggestion == "" {
			t.Fatalf("%q: empty RetrySuggestion", code)
		}
		// Round-trip: the code must normalize back to itself.
		if got := NormalizeInteractFailureCode(code); got != code {
			t.Fatalf("%q does not normalize to itself (got %q)", code, got)
		}
	}
}
