// collector_test.go — Verifies canonical persisted-state recovery diagnostics.
package statediag

import "testing"

func TestCollectorReplacesDiagnosticByStableName(t *testing.T) {
	t.Parallel()

	collector := NewCollector()
	collector.Report(Diagnostic{Name: "response_mode_state", Detail: "first", Fix: "reset"})
	collector.Report(Diagnostic{Name: "response_mode_state", Detail: "latest", Fix: "reset"})

	got := collector.Snapshot()
	if len(got) != 1 || got[0].Detail != "latest" {
		t.Fatalf("Snapshot() = %#v, want latest diagnostic only", got)
	}
}

func TestCollectorReturnsIndependentSortedSnapshot(t *testing.T) {
	t.Parallel()

	collector := NewCollector()
	collector.Report(Diagnostic{Name: "z_state", Detail: "z", Fix: "z"})
	collector.Report(Diagnostic{Name: "a_state", Detail: "a", Fix: "a"})

	first := collector.Snapshot()
	first[0].Detail = "mutated"
	second := collector.Snapshot()
	if len(second) != 2 || second[0].Name != "a_state" || second[0].Detail != "a" {
		t.Fatalf("Snapshot() = %#v, want sorted independent copy", second)
	}
}

func TestNilCollectorIsSafe(t *testing.T) {
	t.Parallel()

	var collector *Collector
	collector.Report(Diagnostic{Name: "ignored"})
	if got := collector.Snapshot(); got != nil {
		t.Fatalf("nil Snapshot() = %#v, want nil", got)
	}
}
