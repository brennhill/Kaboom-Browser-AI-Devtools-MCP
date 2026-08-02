// transaction_test.go — Tests atomic QA fixture snapshot, apply, and rollback coordination.

package qafixture

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCoordinatorAppliesOnlyAfterSnapshot(t *testing.T) {
	var order []string
	coordinator := mustCoordinator(t, TransactionDeps{
		NewCorrelationID: func() string { return "fixture_123" },
		Snapshot: func(context.Context, WireQAFixture) (string, error) {
			order = append(order, "snapshot")
			return "opaque_snapshot", nil
		},
		Apply: func(context.Context, WireQAFixture) (MutationCounts, error) {
			order = append(order, "apply")
			return MutationCounts{LocalStorage: 2}, nil
		},
		Persist: func(*Registry) error {
			order = append(order, "persist")
			return nil
		},
		Restore: func(context.Context, string) error {
			order = append(order, "restore")
			return nil
		},
	})

	result, err := coordinator.Apply(context.Background(), WireQAFixture{Version: 1, SetupTimeoutMs: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "snapshot,persist,apply,persist" {
		t.Fatalf("operation order = %v", order)
	}
	if result.Status != StatusApplied || result.CorrelationID != "fixture_123" || result.Mutations.LocalStorage != 2 {
		t.Fatalf("result = %+v", result)
	}
	assertNoSecret(t, result, err)
}

func TestCoordinatorStopsWhenSnapshotFails(t *testing.T) {
	applied := false
	coordinator := mustCoordinator(t, TransactionDeps{
		NewCorrelationID: func() string { return "fixture_1" },
		Snapshot: func(context.Context, WireQAFixture) (string, error) {
			return "", errors.New("private snapshot failure")
		},
		Apply: func(context.Context, WireQAFixture) (MutationCounts, error) {
			applied = true
			return MutationCounts{}, nil
		},
		Restore: func(context.Context, string) error { return nil },
	})
	result, err := coordinator.Apply(context.Background(), WireQAFixture{Version: 1, SetupTimeoutMs: 1000})
	assertTransactionError(t, result, err, StatusSnapshotFailed)
	if applied {
		t.Fatal("fixture applied without a snapshot")
	}
	assertNoSecret(t, result, err)
}

func TestCoordinatorRollsBackPartialApplyFailure(t *testing.T) {
	snapshot := "opaque_snapshot"
	var restored string
	coordinator := mustCoordinator(t, TransactionDeps{
		NewCorrelationID: func() string { return "fixture_2" },
		Snapshot:         func(context.Context, WireQAFixture) (string, error) { return snapshot, nil },
		Apply: func(context.Context, WireQAFixture) (MutationCounts, error) {
			return MutationCounts{Cookies: 1}, errors.New("private apply failure")
		},
		Restore: func(_ context.Context, value string) error {
			restored = value
			return nil
		},
	})
	result, err := coordinator.Apply(context.Background(), WireQAFixture{Version: 1, SetupTimeoutMs: 1000})
	assertTransactionError(t, result, err, StatusApplyFailedRolledBack)
	if restored != snapshot {
		t.Fatalf("restored snapshot = %s", restored)
	}
	assertNoSecret(t, result, err)
}

func TestCoordinatorReportsRollbackFailureWithoutLeakingCause(t *testing.T) {
	coordinator := mustCoordinator(t, TransactionDeps{
		NewCorrelationID: func() string { return "fixture_3" },
		Snapshot: func(context.Context, WireQAFixture) (string, error) {
			return "opaque_snapshot", nil
		},
		Apply: func(context.Context, WireQAFixture) (MutationCounts, error) {
			return MutationCounts{}, errors.New("secret apply")
		},
		Restore: func(context.Context, string) error { return errors.New("secret restore") },
	})
	result, err := coordinator.Apply(context.Background(), WireQAFixture{Version: 1, SetupTimeoutMs: 1000})
	assertTransactionError(t, result, err, StatusRollbackFailed)
	assertNoSecret(t, result, err)
}

func TestCoordinatorBoundsTimeoutAndCancellation(t *testing.T) {
	tests := []struct {
		name   string
		ctx    func() context.Context
		status string
	}{
		{"timeout", context.Background, StatusTimedOut},
		{"canceled", canceledContext, StatusCanceled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coordinator := mustCoordinator(t, TransactionDeps{
				NewCorrelationID: func() string { return "fixture_4" },
				Snapshot: func(ctx context.Context, _ WireQAFixture) (string, error) {
					<-ctx.Done()
					return "", ctx.Err()
				},
				Apply:   func(context.Context, WireQAFixture) (MutationCounts, error) { return MutationCounts{}, nil },
				Restore: func(context.Context, string) error { return nil },
			})
			fixture := WireQAFixture{Version: 1, SetupTimeoutMs: 5}
			result, err := coordinator.Apply(tt.ctx(), fixture)
			assertTransactionError(t, result, err, tt.status)
		})
	}
}

func TestCoordinatorClassifiesExplicitDriverDeadline(t *testing.T) {
	coordinator := mustCoordinator(t, TransactionDeps{
		NewCorrelationID: func() string { return "fixture_deadline" },
		Snapshot: func(context.Context, WireQAFixture) (string, error) {
			return "", context.DeadlineExceeded
		},
		Apply:   func(context.Context, WireQAFixture) (MutationCounts, error) { return MutationCounts{}, nil },
		Restore: func(context.Context, string) error { return nil },
	})
	result, err := coordinator.Apply(context.Background(), WireQAFixture{SetupTimeoutMs: 1000})
	assertTransactionError(t, result, err, StatusTimedOut)
}

func TestCoordinatorRejectsConcurrentRun(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	coordinator := mustCoordinator(t, TransactionDeps{
		NewCorrelationID: func() string { return "fixture_generated" },
		Snapshot: func(context.Context, WireQAFixture) (string, error) {
			close(entered)
			<-release
			return "opaque_snapshot", nil
		},
		Apply:   func(context.Context, WireQAFixture) (MutationCounts, error) { return MutationCounts{}, nil },
		Restore: func(context.Context, string) error { return nil },
	})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = coordinator.Apply(context.Background(), WireQAFixture{SetupTimeoutMs: 1000})
	}()
	<-entered
	result, err := coordinator.Apply(context.Background(), WireQAFixture{SetupTimeoutMs: 1000})
	assertTransactionError(t, result, err, StatusBusy)
	close(release)
	wg.Wait()
}

func TestCoordinatorPersistsOpaqueRecoveryRecordBeforeReportingSuccess(t *testing.T) {
	registry := NewRegistry(2)
	persisted := false
	coordinator := mustCoordinator(t, TransactionDeps{
		NewCorrelationID:    func() string { return "correlation_1" },
		NewTransactionID:    func() string { return "transaction_1" },
		ExtensionGeneration: func() string { return "generation_1" },
		Now:                 func() time.Time { return time.Unix(10, 0) },
		Registry:            registry,
		Persist: func(got *Registry) error {
			persisted = got.Len() == 1
			return nil
		},
		Snapshot: func(context.Context, WireQAFixture) (string, error) { return "opaque_snapshot", nil },
		Apply:    func(context.Context, WireQAFixture) (MutationCounts, error) { return MutationCounts{Cookies: 1}, nil },
		Restore:  func(context.Context, string) error { return nil },
	})

	result, err := coordinator.Apply(context.Background(), WireQAFixture{Version: 1, SetupTimeoutMs: 1000})
	if err != nil || !persisted || result.TransactionID != "transaction_1" {
		t.Fatalf("Apply() result=%+v persisted=%v error=%v", result, persisted, err)
	}
	record, ok := registry.Get("transaction_1")
	if !ok || record.SnapshotID != "opaque_snapshot" || record.ExtensionGeneration != "generation_1" {
		t.Fatalf("recovery record = %+v, present=%v", record, ok)
	}
}

func TestCoordinatorRollsBackWhenRecoveryPersistenceFails(t *testing.T) {
	registry := NewRegistry(1)
	restored := ""
	applied := false
	coordinator := mustCoordinator(t, TransactionDeps{
		NewCorrelationID: func() string { return "correlation_1" },
		Registry:         registry,
		Persist:          func(*Registry) error { return errors.New("private disk failure") },
		Snapshot:         func(context.Context, WireQAFixture) (string, error) { return "opaque_snapshot", nil },
		Apply: func(context.Context, WireQAFixture) (MutationCounts, error) {
			applied = true
			return MutationCounts{}, nil
		},
		Restore: func(_ context.Context, snapshot string) error {
			restored = snapshot
			return nil
		},
	})

	result, err := coordinator.Apply(context.Background(), WireQAFixture{Version: 1, SetupTimeoutMs: 1000})
	assertTransactionError(t, result, err, StatusRecoveryRegisterFailed)
	if restored != "opaque_snapshot" || registry.Len() != 0 || applied {
		t.Fatalf("restored=%q registry_len=%d applied=%v", restored, registry.Len(), applied)
	}
	assertNoSecret(t, result, err)
}

func TestCoordinatorNeverEvictsActiveRecoveryAtCapacity(t *testing.T) {
	registry := NewRegistry(1)
	if err := registry.Add(TransactionRecord{
		TransactionID: "existing", SnapshotID: "existing_snapshot",
		ExtensionGeneration: "generation_1", State: TransactionRestoreRequired,
	}); err != nil {
		t.Fatal(err)
	}
	restored := false
	coordinator := mustCoordinator(t, TransactionDeps{
		NewCorrelationID: func() string { return "correlation_1" },
		Registry:         registry,
		Snapshot:         func(context.Context, WireQAFixture) (string, error) { return "new_snapshot", nil },
		Apply:            func(context.Context, WireQAFixture) (MutationCounts, error) { return MutationCounts{}, nil },
		Restore: func(context.Context, string) error {
			restored = true
			return nil
		},
	})

	result, err := coordinator.Apply(context.Background(), WireQAFixture{Version: 1, SetupTimeoutMs: 1000})
	assertTransactionError(t, result, err, StatusRecoveryRegisterFailed)
	if !restored {
		t.Fatal("unregistered mutation was not rolled back")
	}
	if _, ok := registry.Get("existing"); !ok {
		t.Fatal("existing recovery obligation was evicted")
	}
}

func TestNewCoordinatorRejectsMissingTransactionSeams(t *testing.T) {
	if _, err := NewCoordinator(TransactionDeps{}); err == nil || err.Error() != "incomplete_transaction_dependencies" {
		t.Fatalf("NewCoordinator error = %v", err)
	}
}

func mustCoordinator(t *testing.T, deps TransactionDeps) *Coordinator {
	t.Helper()
	if deps.NewTransactionID == nil {
		deps.NewTransactionID = func() string { return "transaction_1" }
	}
	if deps.ExtensionGeneration == nil {
		deps.ExtensionGeneration = func() string { return "generation_1" }
	}
	if deps.Now == nil {
		deps.Now = func() time.Time { return time.Unix(1, 0) }
	}
	if deps.Registry == nil {
		deps.Registry = NewRegistry(32)
	}
	if deps.Persist == nil {
		deps.Persist = func(*Registry) error { return nil }
	}
	if deps.OnNotice == nil {
		deps.OnNotice = func(string) {}
	}
	coordinator, err := NewCoordinator(deps)
	if err != nil {
		t.Fatal(err)
	}
	return coordinator
}

func canceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func assertTransactionError(t *testing.T, result TransactionResult, err error, status string) {
	t.Helper()
	if err == nil || result.Status != status || err.Error() != status {
		t.Fatalf("result=%+v error=%v, want status/error %q", result, err, status)
	}
}

func assertNoSecret(t *testing.T, result TransactionResult, err error) {
	t.Helper()
	encoded, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	combined := string(encoded)
	if err != nil {
		combined += err.Error()
	}
	if strings.Contains(combined, "secret") || strings.Contains(combined, "private") || strings.Contains(combined, "token") {
		t.Fatalf("transaction output leaked private data: %s", combined)
	}
}
