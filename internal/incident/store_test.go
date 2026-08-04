// store_test.go — Verifies canonical operational incident lifecycle invariants.
package incident

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestStoreRejectsUnknownCodesAndStaleGenerations(t *testing.T) {
	t.Parallel()
	store := NewStore(4)
	if _, err := store.Detect(Report{Code: Code("invented"), Generation: 1}); err == nil {
		t.Fatal("unknown incident code was accepted")
	}
	key, err := store.Detect(Report{Code: CodeContentReadinessTimeout, CorrelationID: "nav-1", Generation: 2})
	if err != nil {
		t.Fatal(err)
	}
	if changed := store.Retry(key, 1, 1); changed {
		t.Fatal("stale generation changed current incident")
	}
	got := store.Snapshot()[0]
	if got.State != StateDetected || got.Attempts != 0 || got.Generation != 2 {
		t.Fatalf("incident changed after stale transition: %#v", got)
	}
}

func TestStoreTransitionsAreIdempotentAndRecoveryResolvesIncident(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 4, 8, 0, 0, 0, time.UTC)
	store := NewStore(4)
	store.now = func() time.Time { return now }
	key, err := store.Detect(Report{
		Code: CodeExtensionReconnectExhausted, CorrelationID: "connect-1", Generation: 3,
		Evidence: LocalEvidence{Detail: "socket reset", Fix: "reload extension"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if duplicate, err := store.Detect(Report{Code: CodeExtensionReconnectExhausted, CorrelationID: "connect-1", Generation: 3}); err != nil || duplicate != key {
		t.Fatalf("duplicate detection = %q, %v", duplicate, err)
	}
	if !store.Retry(key, 3, 1) || store.Retry(key, 3, 1) {
		t.Fatal("retry transition was not idempotent")
	}
	now = now.Add(2 * time.Second)
	if !store.Recover(key, 3) || store.Recover(key, 3) {
		t.Fatal("recovery transition was not idempotent")
	}
	got := store.Snapshot()[0]
	if got.State != StateRecovered || got.Attempts != 1 || len(got.History) != 3 {
		t.Fatalf("recovered incident = %#v", got)
	}
	if got.ResolvedAt.IsZero() || got.LocalEvidence.Detail != "socket reset" {
		t.Fatalf("local recovery evidence = %#v", got)
	}
}

func TestAnalyticsProjectionIsClosedAndContainsNoLocalEvidence(t *testing.T) {
	t.Parallel()
	store := NewStore(2)
	key, err := store.Detect(Report{
		Code: CodeStateRecoveryFailed, CorrelationID: "private-correlation", Generation: 1,
		Evidence: LocalEvidence{Detail: "https://private.test token=secret", Fix: "/Users/private/file"},
	})
	if err != nil {
		t.Fatal(err)
	}
	store.Retry(key, 1, 1)
	store.Exhaust(key, 1)
	event, ok := store.Analytics(key)
	if !ok {
		t.Fatal("missing analytics projection")
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	payload := string(encoded)
	for _, forbidden := range []string{"private", "token", "secret", "correlation", "detail", "fix", "generation"} {
		if strings.Contains(strings.ToLower(payload), forbidden) {
			t.Fatalf("analytics projection leaked %q: %s", forbidden, payload)
		}
	}
	if event.Code != CodeStateRecoveryFailed || event.Outcome != OutcomeExhausted || event.AttemptBucket != AttemptOne {
		t.Fatalf("analytics projection = %#v", event)
	}
}

func TestStoreBoundsIncidentsAndReportsPressure(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	store := NewStore(2)
	store.now = func() time.Time { return now }
	first, _ := store.Detect(Report{Code: CodeStateRecoveryFailed, CorrelationID: "first", Generation: 1})
	store.Recover(first, 1)
	now = now.Add(time.Second)
	_, _ = store.Detect(Report{Code: CodeContentReadinessTimeout, CorrelationID: "second", Generation: 1})
	now = now.Add(time.Second)
	_, _ = store.Detect(Report{Code: CodeQueueSaturated, CorrelationID: "third", Generation: 1})

	got := store.Snapshot()
	if len(got) != 2 || got[0].CorrelationID != "second" || got[1].CorrelationID != "third" {
		t.Fatalf("bounded snapshot = %#v", got)
	}
	stats := store.Stats()
	if stats.Capacity != 2 || stats.Dropped != 1 || stats.Active != 2 || stats.Terminal != 0 {
		t.Fatalf("pressure stats = %#v", stats)
	}
}

func TestRegistryProvidesDoctorPresentationWithoutCallerProse(t *testing.T) {
	t.Parallel()
	definition, ok := Lookup(CodeContentReadinessTimeout)
	if !ok {
		t.Fatal("registered incident code missing")
	}
	if definition.Subsystem != SubsystemBridge || definition.Stage != StageReadiness || definition.Severity != SeverityError || !definition.Retryable {
		t.Fatalf("incident definition = %#v", definition)
	}
	if definition.DoctorDetail == "" || definition.DoctorFix == "" {
		t.Fatal("Doctor presentation must be owned by the registry")
	}
}

func TestDoctorProjectionCombinesLocalEvidenceWithRegistryPresentation(t *testing.T) {
	t.Parallel()
	store := NewStore(2)
	key, err := store.Detect(Report{
		Code: CodeContentReadinessTimeout, CorrelationID: "nav-42", Generation: 7,
		Evidence: LocalEvidence{Detail: "attempt token=private", Fix: "local reload context"},
	})
	if err != nil {
		t.Fatal(err)
	}
	view, ok := store.Doctor(key)
	if !ok {
		t.Fatal("missing Doctor projection")
	}
	if view.Code != CodeContentReadinessTimeout || view.CorrelationID != "nav-42" || view.Generation != 7 || view.State != StateDetected {
		t.Fatalf("Doctor identity = %#v", view)
	}
	if view.Detail == "" || view.Fix == "" || !strings.Contains(view.LocalDetail, "token=[REDACTED]") || view.LocalFix != "local reload context" {
		t.Fatalf("Doctor presentation = %#v", view)
	}
}
