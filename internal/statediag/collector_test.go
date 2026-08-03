// collector_test.go — Verifies canonical persisted-state recovery diagnostics.
package statediag

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefile"
)

func TestCollectorBuildsCorrelatedIncidentTimeline(t *testing.T) {
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	collector := NewCollector()
	collector.now = func() time.Time { return now }
	collector.Report(Diagnostic{
		Name: "content_readiness", CorrelationID: "nav-123", Detail: "probe timed out", Fix: "retry",
		ExpectedNextTransition: "content_ready", Deadline: now.Add(5 * time.Second),
	})
	now = now.Add(time.Second)
	collector.Report(Diagnostic{Name: "content_readiness", CorrelationID: "nav-123", Detail: "retry failed", Fix: "reload"})
	now = now.Add(time.Second)
	collector.Resolve("content_readiness")

	got := collector.Snapshot()[0]
	if got.CorrelationID != "nav-123" || got.RecoveryAttempt != 2 || got.RecoveryOutcome != "recovered" {
		t.Fatalf("incident metadata = %#v", got)
	}
	if got.LastSuccessfulTransition != "state_verified" || got.ExpectedNextTransition != "" || !got.Deadline.IsZero() {
		t.Fatalf("recovered transition contract = %#v", got)
	}
	if len(got.History) != 3 || got.History[1].Event != "failure_recurred" || got.History[2].Outcome != "recovered" {
		t.Fatalf("timeline = %#v", got.History)
	}
}

func TestCollectorMakesIncompleteTimelineExplicitAndRedactsExports(t *testing.T) {
	collector := NewCollector()
	collector.Report(Diagnostic{
		Name: "command_state", CorrelationID: "cmd-123", Detail: "Bearer private-token at https://example.test/path?token=private",
		Fix: "retry with api_key=private",
	})
	got := collector.Snapshot()[0]
	if got.ExpectedNextTransition != "state_verified" || got.Deadline.IsZero() == false {
		t.Fatalf("incomplete timeline fallback = %#v", got)
	}
	encoded := fmt.Sprintf("%#v", got)
	if strings.Contains(encoded, "private-token") || strings.Contains(encoded, "token=private") || strings.Contains(encoded, "api_key=private") {
		t.Fatalf("diagnostic export leaked private values: %s", encoded)
	}
}

func TestPersistentCollectorRestoresBoundedTimelineAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doctor-incidents.json")
	first, err := NewPersistentCollector(path)
	if err != nil {
		t.Fatal(err)
	}
	first.Report(Diagnostic{Name: "tracking_state", CorrelationID: "track-1", Detail: "recovering", Fix: "wait"})
	first.Resolve("tracking_state")
	first.Report(Diagnostic{Name: "command_state", CorrelationID: "cmd-1", Detail: "retry", Fix: "retry"})

	restarted, err := NewPersistentCollector(path)
	if err != nil {
		t.Fatal(err)
	}
	got := restarted.Snapshot()
	if len(got) != 2 || got[0].Name != "command_state" || got[1].Lifecycle != LifecycleRecovered {
		t.Fatalf("restored snapshot = %#v", got)
	}
	if restarted.Stats().Active != 1 || restarted.Stats().Recovered != 1 {
		t.Fatalf("restored stats = %#v", restarted.Stats())
	}
}

func TestPersistentCollectorMakesWriteFailureVisibleWithoutLeakingCause(t *testing.T) {
	collector, err := NewPersistentCollector(filepath.Join(t.TempDir(), "doctor-incidents.json"))
	if err != nil {
		t.Fatal(err)
	}
	collector.writeFile = func(string, []byte, os.FileMode) error {
		return errors.New("private persistence cause")
	}
	collector.Report(Diagnostic{Name: "tracking_state", Detail: "recovering", Fix: "retry"})

	got := collector.Snapshot()
	if len(got) != 2 || got[0].Name != "doctor_timeline_persistence" || got[0].Lifecycle != LifecycleActive {
		t.Fatalf("persistence failure timeline = %#v", got)
	}
	if strings.Contains(fmt.Sprintf("%#v", got), "private persistence cause") {
		t.Fatalf("persistence cause leaked: %#v", got)
	}
	collector.writeFile = statefile.Write
	collector.Report(Diagnostic{Name: "tracking_state", Detail: "still recovering", Fix: "retry"})
	got = collector.Snapshot()
	if got[0].Name != "doctor_timeline_persistence" || got[0].Lifecycle != LifecycleRecovered {
		t.Fatalf("persistence recovery timeline = %#v", got)
	}
}

func TestPersistentCollectorRejectsCorruptRestartState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doctor-incidents.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"diagnostics":[{"detail":"private"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	collector, err := NewPersistentCollector(path)
	if err == nil || len(collector.Snapshot()) != 0 {
		t.Fatalf("corrupt restore collector=%#v error=%v", collector.Snapshot(), err)
	}
}

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
	if len(diagnostic.History) != 3 ||
		diagnostic.History[0].Lifecycle != LifecycleActive ||
		diagnostic.History[1].Event != "failure_recurred" ||
		diagnostic.History[2].Lifecycle != LifecycleRecovered {
		t.Fatalf("history = %#v, want active recurrence then recovery", diagnostic.History)
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

func TestCollectorBoundsHistoryAtFirstOverflowTransition(t *testing.T) {
	collector := NewCollector()
	for cycle := 0; cycle < maxHistoryTransitions/2; cycle++ {
		collector.Report(Diagnostic{Name: "bounded", Detail: "safe", Fix: "retry"})
		collector.Resolve("bounded")
	}
	collector.Report(Diagnostic{Name: "bounded", Detail: "safe", Fix: "retry"})

	diagnostics := collector.Snapshot()
	if len(diagnostics) != 1 || len(diagnostics[0].History) != maxHistoryTransitions {
		t.Fatalf("history length after first overflow = %d, want %d", len(diagnostics[0].History), maxHistoryTransitions)
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
