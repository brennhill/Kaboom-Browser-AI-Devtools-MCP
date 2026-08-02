// transaction.go — Coordinates bounded QA fixture snapshot, apply, and rollback.

package qafixture

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

const (
	StatusApplied               = "applied"
	StatusSnapshotFailed        = "snapshot_failed"
	StatusApplyFailedRolledBack = "apply_failed_rolled_back"
	StatusRollbackFailed        = "rollback_failed"
	StatusTimedOut              = "timed_out"
	StatusCanceled              = "canceled"
	StatusBusy                  = "fixture_transaction_busy"
)

type MutationCounts struct {
	Cookies        int `json:"cookies"`
	LocalStorage   int `json:"local_storage"`
	SessionStorage int `json:"session_storage"`
	FeatureFlags   int `json:"feature_flags"`
	SeedData       int `json:"seed_data"`
}

type TransactionResult struct {
	Status        string         `json:"status"`
	CorrelationID string         `json:"correlation_id"`
	RolledBack    bool           `json:"rolled_back"`
	Mutations     MutationCounts `json:"mutations"`
}

type TransactionDeps struct {
	// Snapshot, Apply, and Restore must honor context cancellation. This keeps
	// setup bounded without allowing a timed-out mutation to race its rollback.
	NewCorrelationID func() string
	Snapshot         func(context.Context, WireQAFixture) (json.RawMessage, error)
	Apply            func(context.Context, WireQAFixture) (MutationCounts, error)
	Restore          func(context.Context, WireQAFixture, json.RawMessage) error
}

type Coordinator struct {
	deps TransactionDeps
	mu   sync.Mutex
	busy bool
}

func NewCoordinator(deps TransactionDeps) (*Coordinator, error) {
	if deps.NewCorrelationID == nil || deps.Snapshot == nil || deps.Apply == nil || deps.Restore == nil {
		return nil, errors.New("incomplete_transaction_dependencies")
	}
	return &Coordinator{deps: deps}, nil
}

// Apply captures private state before invoking the first mutation. Driver
// causes are intentionally collapsed into stable status codes so persisted
// values cannot escape through MCP responses or diagnostics.
func (coordinator *Coordinator) Apply(
	ctx context.Context,
	fixture WireQAFixture,
) (TransactionResult, error) {
	if !coordinator.acquire() {
		return transactionFailure(StatusBusy, "", false, MutationCounts{})
	}
	defer coordinator.release()
	correlationID := coordinator.deps.NewCorrelationID()

	timeout := time.Duration(fixture.SetupTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = time.Duration(DefaultSetupTimeoutMs) * time.Millisecond
	}
	transactionCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	snapshot, err := coordinator.deps.Snapshot(transactionCtx, fixture)
	if err != nil {
		return transactionFailure(contextStatus(transactionCtx, err, StatusSnapshotFailed), correlationID, false, MutationCounts{})
	}

	mutations, err := coordinator.deps.Apply(transactionCtx, fixture)
	if err == nil {
		return TransactionResult{
			Status: StatusApplied, CorrelationID: correlationID, Mutations: mutations,
		}, nil
	}

	rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), timeout)
	defer rollbackCancel()
	if restoreErr := coordinator.deps.Restore(rollbackCtx, fixture, snapshot); restoreErr != nil {
		return transactionFailure(StatusRollbackFailed, correlationID, false, mutations)
	}
	status := contextStatus(transactionCtx, err, StatusApplyFailedRolledBack)
	return transactionFailure(status, correlationID, true, mutations)
}

func (coordinator *Coordinator) acquire() bool {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	if coordinator.busy {
		return false
	}
	coordinator.busy = true
	return true
}

func (coordinator *Coordinator) release() {
	coordinator.mu.Lock()
	coordinator.busy = false
	coordinator.mu.Unlock()
}

func contextStatus(ctx context.Context, cause error, fallback string) string {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(cause, context.DeadlineExceeded) {
		return StatusTimedOut
	}
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(cause, context.Canceled) {
		return StatusCanceled
	}
	return fallback
}

func transactionFailure(
	status, correlationID string,
	rolledBack bool,
	mutations MutationCounts,
) (TransactionResult, error) {
	return TransactionResult{
		Status: status, CorrelationID: correlationID, RolledBack: rolledBack, Mutations: mutations,
	}, errors.New(status)
}
