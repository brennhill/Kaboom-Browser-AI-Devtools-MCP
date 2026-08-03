// collector_test.go — Verifies canonical persisted-state recovery diagnostics.
package statediag

import (
	"fmt"
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

func TestCollectorPreservesSafeCorrelationID(t *testing.T) {
	collector := NewCollector()
	collector.Report(Diagnostic{Name: "fixture_transaction", CorrelationID: "qa_fixture_123", Detail: "pending", Fix: "restore"})

	got := collector.Snapshot()
	if len(got) != 1 || got[0].CorrelationID != "qa_fixture_123" {
		t.Fatalf("Snapshot() = %#v, want correlated diagnostic", got)
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

func TestCollectorBoundsRecoveredIncidentsWithoutEvictingActiveIncidents(t *testing.T) {
	collector := NewCollector()
	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	collector.now = func() time.Time { return now }

	for index := 0; index < maxRecoveredDiagnostics+25; index++ {
		name := fmt.Sprintf("recovered_%03d", index)
		collector.Report(Diagnostic{Name: name, Detail: "safe", Fix: "retry"})
		collector.Resolve(name)
		now = now.Add(time.Second)
	}
	for index := 0; index < 12; index++ {
		collector.Report(Diagnostic{Name: fmt.Sprintf("active_%03d", index), Detail: "safe", Fix: "retry"})
	}

	snapshot := collector.Snapshot()
	stats := collector.Stats()
	if len(snapshot) != maxRecoveredDiagnostics+12 {
		t.Fatalf("Snapshot size = %d, want %d", len(snapshot), maxRecoveredDiagnostics+12)
	}
	if stats.Active != 12 || stats.Recovered != maxRecoveredDiagnostics || stats.DroppedRecovered != 25 {
		t.Fatalf("Stats() = %#v", stats)
	}
	if snapshot[12].Name != "recovered_025" {
		t.Fatalf("oldest retained recovered incident = %q, want recovered_025", snapshot[12].Name)
	}
}

func TestCollectorRecoveredEvictionBreaksTimestampTiesByName(t *testing.T) {
	collector := NewCollector()
	collector.recoveredLimit = 2
	collector.now = func() time.Time { return time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC) }
	for _, name := range []string{"z_state", "a_state", "m_state"} {
		collector.Report(Diagnostic{Name: name, Detail: "safe", Fix: "retry"})
		collector.Resolve(name)
	}

	got := collector.Snapshot()
	if len(got) != 2 || got[0].Name != "m_state" || got[1].Name != "z_state" {
		t.Fatalf("Snapshot() = %#v, want deterministic eviction of a_state", got)
	}
}

func FuzzCollectorLifecycleTransitions(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5})
	f.Add([]byte{1, 3, 5, 7, 0, 2, 4, 6})
	f.Fuzz(func(t *testing.T, operations []byte) {
		collector := NewCollector()
		now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
		collector.now = func() time.Time { return now }
		for _, operation := range operations {
			name := fmt.Sprintf("incident_%d", operation%8)
			if operation&1 == 0 {
				collector.Report(Diagnostic{Name: name, CorrelationID: "correlation_fuzz", Detail: "safe", Fix: "retry"})
			} else {
				collector.Resolve(name)
			}
			now = now.Add(time.Nanosecond)
		}
		for _, diagnostic := range collector.Snapshot() {
			if diagnostic.Name == "" || diagnostic.FirstSeenAt.IsZero() || len(diagnostic.History) > maxHistoryTransitions {
				t.Fatalf("invalid diagnostic lifecycle: %#v", diagnostic)
			}
			if diagnostic.Lifecycle != LifecycleActive && diagnostic.Lifecycle != LifecycleRecovered {
				t.Fatalf("invalid lifecycle: %q", diagnostic.Lifecycle)
			}
		}
		stats := collector.Stats()
		if stats.Recovered > stats.RecoveredLimit {
			t.Fatalf("recovered diagnostics exceeded bound: %#v", stats)
		}
	})
}
