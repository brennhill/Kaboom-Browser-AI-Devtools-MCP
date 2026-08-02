// transaction_test.go — Tests atomic QA fixture snapshot, apply, and rollback coordination.

package qafixture

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestCoordinatorAppliesOnlyAfterSnapshot(t *testing.T) {
	var order []string
	coordinator := mustCoordinator(t, TransactionDeps{
		NewCorrelationID: func() string { return "fixture_123" },
		Snapshot: func(context.Context, WireQAFixture) (json.RawMessage, error) {
			order = append(order, "snapshot")
			return json.RawMessage(`{"private":"secret"}`), nil
		},
		Apply: func(context.Context, WireQAFixture) (MutationCounts, error) {
			order = append(order, "apply")
			return MutationCounts{LocalStorage: 2}, nil
		},
		Restore: func(context.Context, json.RawMessage) error {
			order = append(order, "restore")
			return nil
		},
	})

	result, err := coordinator.Apply(context.Background(), WireQAFixture{Version: 1, SetupTimeoutMs: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "snapshot,apply" {
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
		Snapshot: func(context.Context, WireQAFixture) (json.RawMessage, error) {
			return nil, errors.New("private snapshot failure")
		},
		Apply: func(context.Context, WireQAFixture) (MutationCounts, error) {
			applied = true
			return MutationCounts{}, nil
		},
		Restore: func(context.Context, json.RawMessage) error { return nil },
	})
	result, err := coordinator.Apply(context.Background(), WireQAFixture{Version: 1, SetupTimeoutMs: 1000})
	assertTransactionError(t, result, err, StatusSnapshotFailed)
	if applied {
		t.Fatal("fixture applied without a snapshot")
	}
	assertNoSecret(t, result, err)
}

func TestCoordinatorRollsBackPartialApplyFailure(t *testing.T) {
	snapshot := json.RawMessage(`{"token":"private-secret"}`)
	var restored json.RawMessage
	coordinator := mustCoordinator(t, TransactionDeps{
		NewCorrelationID: func() string { return "fixture_2" },
		Snapshot:         func(context.Context, WireQAFixture) (json.RawMessage, error) { return snapshot, nil },
		Apply: func(context.Context, WireQAFixture) (MutationCounts, error) {
			return MutationCounts{Cookies: 1}, errors.New("private apply failure")
		},
		Restore: func(_ context.Context, value json.RawMessage) error {
			restored = append(json.RawMessage(nil), value...)
			return nil
		},
	})
	result, err := coordinator.Apply(context.Background(), WireQAFixture{Version: 1, SetupTimeoutMs: 1000})
	assertTransactionError(t, result, err, StatusApplyFailedRolledBack)
	if string(restored) != string(snapshot) {
		t.Fatalf("restored snapshot = %s", restored)
	}
	assertNoSecret(t, result, err)
}

func TestCoordinatorReportsRollbackFailureWithoutLeakingCause(t *testing.T) {
	coordinator := mustCoordinator(t, TransactionDeps{
		NewCorrelationID: func() string { return "fixture_3" },
		Snapshot: func(context.Context, WireQAFixture) (json.RawMessage, error) {
			return json.RawMessage(`{"private":"secret"}`), nil
		},
		Apply: func(context.Context, WireQAFixture) (MutationCounts, error) {
			return MutationCounts{}, errors.New("secret apply")
		},
		Restore: func(context.Context, json.RawMessage) error { return errors.New("secret restore") },
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
				Snapshot: func(ctx context.Context, _ WireQAFixture) (json.RawMessage, error) {
					<-ctx.Done()
					return nil, ctx.Err()
				},
				Apply:   func(context.Context, WireQAFixture) (MutationCounts, error) { return MutationCounts{}, nil },
				Restore: func(context.Context, json.RawMessage) error { return nil },
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
		Snapshot: func(context.Context, WireQAFixture) (json.RawMessage, error) {
			return nil, context.DeadlineExceeded
		},
		Apply:   func(context.Context, WireQAFixture) (MutationCounts, error) { return MutationCounts{}, nil },
		Restore: func(context.Context, json.RawMessage) error { return nil },
	})
	result, err := coordinator.Apply(context.Background(), WireQAFixture{SetupTimeoutMs: 1000})
	assertTransactionError(t, result, err, StatusTimedOut)
}

func TestCoordinatorRejectsConcurrentRun(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	coordinator := mustCoordinator(t, TransactionDeps{
		NewCorrelationID: func() string { return "fixture_generated" },
		Snapshot: func(context.Context, WireQAFixture) (json.RawMessage, error) {
			close(entered)
			<-release
			return json.RawMessage(`{}`), nil
		},
		Apply:   func(context.Context, WireQAFixture) (MutationCounts, error) { return MutationCounts{}, nil },
		Restore: func(context.Context, json.RawMessage) error { return nil },
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

func TestNewCoordinatorRejectsMissingTransactionSeams(t *testing.T) {
	if _, err := NewCoordinator(TransactionDeps{}); err == nil || err.Error() != "incomplete_transaction_dependencies" {
		t.Fatalf("NewCoordinator error = %v", err)
	}
}

func mustCoordinator(t *testing.T, deps TransactionDeps) *Coordinator {
	t.Helper()
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
