// store_test.go — Verifies canonical operational incident lifecycle invariants.
package incident

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
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

func TestStoreEnforcesLifecycleGraphAndRetryability(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		code       Code
		transition func(*Store, string) bool
		want       bool
	}{
		{"detected can recover without a retry", CodeContentReadinessTimeout, func(s *Store, key string) bool { return s.Recover(key, 1) }, true},
		{"detected can exhaust without a retry", CodeContentReadinessTimeout, func(s *Store, key string) bool { return s.Exhaust(key, 1) }, true},
		{"non-retryable cannot retry", CodeDaemonRestartLoop, func(s *Store, key string) bool { return s.Retry(key, 1, 1) }, false},
		{"non-retryable can exhaust", CodeDaemonRestartLoop, func(s *Store, key string) bool { return s.Exhaust(key, 1) }, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := NewStore(2)
			key, err := store.Detect(Report{Code: test.code, CorrelationID: test.name, Generation: 1})
			if err != nil {
				t.Fatal(err)
			}
			if got := test.transition(store, key); got != test.want {
				t.Fatalf("transition = %t, want %t", got, test.want)
			}
		})
	}
}

func TestCorrelationIdentityDoesNotUseRedactedOrTruncatedDisplayText(t *testing.T) {
	t.Parallel()
	store := NewStore(4)
	first, err := store.Detect(Report{Code: CodeStateRecoveryFailed, CorrelationID: "token=alpha", Generation: 1})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Detect(Report{Code: CodeStateRecoveryFailed, CorrelationID: "token=beta", Generation: 1})
	if err != nil {
		t.Fatal(err)
	}
	longPrefix := strings.Repeat("x", 500)
	third, _ := store.Detect(Report{Code: CodeStateRecoveryFailed, CorrelationID: longPrefix + "a", Generation: 1})
	fourth, _ := store.Detect(Report{Code: CodeStateRecoveryFailed, CorrelationID: longPrefix + "b", Generation: 1})
	if first == second || third == fourth || len(store.Snapshot()) != 4 {
		t.Fatalf("incident identities collided: %q %q %q %q", first, second, third, fourth)
	}
}

func TestLocalEvidenceRedactsSupportedSecretForms(t *testing.T) {
	t.Parallel()
	secrets := []string{
		"Bearer bearer-secret",
		"authorization: bearer header-secret",
		"Authorization:\tBearer   tab-secret",
		`{"token":"json secret with spaces"}`,
		"Cookie: session=abcdefghijklmnop",
		"api-key: colon-secret",
		"password=equals-secret",
	}
	for index, secret := range secrets {
		store := NewStore(1)
		key, err := store.Detect(Report{Code: CodeStateRecoveryFailed, CorrelationID: fmt.Sprintf("redact-%d", index), Generation: 1, Evidence: LocalEvidence{Detail: secret}})
		if err != nil {
			t.Fatal(err)
		}
		view, _ := store.Doctor(key)
		for _, private := range []string{"bearer-secret", "header-secret", "tab-secret", "json secret with spaces", "abcdefghijklmnop", "colon-secret", "equals-secret"} {
			if strings.Contains(view.LocalDetail, private) {
				t.Fatalf("secret %q leaked from %q as %q", private, secret, view.LocalDetail)
			}
		}
	}
}

func TestStoreConcurrentTransitionsRemainMonotonic(t *testing.T) {
	t.Parallel()
	store := NewStore(8)
	key, err := store.Detect(Report{Code: CodeContentReadinessTimeout, CorrelationID: "concurrent", Generation: 4})
	if err != nil {
		t.Fatal(err)
	}
	const attempts = 32
	start := make(chan struct{})
	var workers sync.WaitGroup
	for attempt := 1; attempt <= attempts; attempt++ {
		workers.Add(1)
		go func(value uint) {
			defer workers.Done()
			<-start
			store.Retry(key, 4, value)
		}(uint(attempt))
	}
	close(start)
	workers.Wait()
	got := store.Snapshot()[0]
	if got.State != StateRetrying || got.Attempts != attempts {
		t.Fatalf("concurrent retry state = %#v", got)
	}
	if !store.Recover(key, 4) {
		t.Fatal("current generation did not recover")
	}
	if store.Exhaust(key, 4) || store.Retry(key, 4, attempts+1) {
		t.Fatal("terminal incident accepted a later transition")
	}
}

func TestStoreTransitionsAreIdempotentAndRecoveryResolvesIncident(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 4, 8, 0, 0, 0, time.UTC)
	store := NewStore(4)
	store.now = func() time.Time { return now }
	key, err := store.Detect(Report{
		Code: CodeExtensionReconnectExhausted, CorrelationID: "connect-1", Generation: 3,
		Evidence: LocalEvidence{Detail: "socket reset"},
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
		Evidence: LocalEvidence{Detail: "https://private.test token=secret"},
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

func TestRegistryClassifiesEveryCodeForPrivacyAndAnalytics(t *testing.T) {
	t.Parallel()
	if len(definitions) != 19 {
		t.Fatalf("registered code count = %d, want 19", len(definitions))
	}
	for code, definition := range definitions {
		if code == "" || definition.Subsystem == "" || definition.Stage == "" || definition.Severity == "" ||
			definition.ErrorKind == "" || definition.Privacy != PrivacyBoundedProductMetadata ||
			definition.DoctorDetail == "" || definition.DoctorFix == "" {
			t.Errorf("incomplete definition for %q: %#v", code, definition)
		}
	}
}

func TestDoctorProjectionCombinesLocalEvidenceWithRegistryPresentation(t *testing.T) {
	t.Parallel()
	store := NewStore(2)
	key, err := store.Detect(Report{
		Code: CodeContentReadinessTimeout, CorrelationID: "nav-42", Generation: 7,
		Evidence: LocalEvidence{Detail: "attempt token=private"},
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
	if view.Detail == "" || view.Fix == "" || !strings.Contains(view.LocalDetail, "[REDACTED:") {
		t.Fatalf("Doctor presentation = %#v", view)
	}
}

func TestStorePublishesProjectionsAfterCommittedTransitions(t *testing.T) {
	t.Parallel()
	var eventsMu sync.Mutex
	var events []ReliabilityEvent
	store := NewStore(2, func(event ReliabilityEvent) {
		eventsMu.Lock()
		events = append(events, event)
		eventsMu.Unlock()
	})
	key, err := store.Detect(Report{Code: CodeContentReadinessTimeout, CorrelationID: "publish", Generation: 1})
	if err != nil {
		t.Fatal(err)
	}
	store.Retry(key, 1, 1)
	store.Recover(key, 1)
	if len(events) != 3 || events[0].Outcome != OutcomePending || events[2].Outcome != OutcomeRecovered {
		t.Fatalf("published events = %#v", events)
	}
	views := store.DoctorSnapshot()
	if len(views) != 1 || views[0].State != StateRecovered || views[0].CorrelationID != "publish" {
		t.Fatalf("Doctor snapshot = %#v", views)
	}
}
