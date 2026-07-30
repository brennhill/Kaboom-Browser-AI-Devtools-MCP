// collector_test.go — Verifies canonical persisted-state recovery diagnostics.
package statediag

import (
	"testing"
	"time"
)

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

func TestCollectorTracksActiveRecoveredAndHistoricalTransitions(t *testing.T) {
	now := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	collector := NewCollector()
	collector.now = func() time.Time { return now }

	collector.Report(Diagnostic{Name: "noise_rules_state", Detail: "fallback active", Fix: "save rules"})
	now = now.Add(time.Minute)
	collector.Report(Diagnostic{Name: "noise_rules_state", Detail: "fallback still active", Fix: "save rules"})
	now = now.Add(time.Minute)
	collector.Resolve("noise_rules_state")

	got := collector.Snapshot()
	if len(got) != 1 {
		t.Fatalf("Snapshot() = %#v, want one lifecycle entry", got)
	}
	diagnostic := got[0]
	if diagnostic.Lifecycle != LifecycleRecovered || diagnostic.Occurrences != 2 {
		t.Fatalf("diagnostic = %#v, want recovered with two occurrences", diagnostic)
	}
	if diagnostic.FirstSeenAt.IsZero() || diagnostic.LastSeenAt.IsZero() || diagnostic.RecoveredAt.IsZero() {
		t.Fatalf("diagnostic timestamps = %#v, want complete lifecycle timestamps", diagnostic)
	}
	if len(diagnostic.History) != 2 ||
		diagnostic.History[0].Lifecycle != LifecycleActive ||
		diagnostic.History[1].Lifecycle != LifecycleRecovered {
		t.Fatalf("history = %#v, want active then recovered transitions", diagnostic.History)
	}
}

func TestCollectorReactivatesRecoveredDiagnosticWithoutLosingHistory(t *testing.T) {
	now := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	collector := NewCollector()
	collector.now = func() time.Time { return now }
	collector.Report(Diagnostic{Name: "recording_state", Detail: "bad", Fix: "recapture"})
	now = now.Add(time.Minute)
	collector.Resolve("recording_state")
	now = now.Add(time.Minute)
	collector.Report(Diagnostic{Name: "recording_state", Detail: "bad again", Fix: "recapture"})

	got := collector.Snapshot()[0]
	if got.Lifecycle != LifecycleActive || got.Occurrences != 2 || !got.RecoveredAt.IsZero() {
		t.Fatalf("reactivated diagnostic = %#v", got)
	}
	if len(got.History) != 3 {
		t.Fatalf("history = %#v, want active/recovered/active", got.History)
	}
}

func TestResolveUnknownDiagnosticIsNoop(t *testing.T) {
	collector := NewCollector()
	collector.Resolve("unknown")
	if got := collector.Snapshot(); len(got) != 0 {
		t.Fatalf("Snapshot() = %#v, want empty", got)
	}
}
