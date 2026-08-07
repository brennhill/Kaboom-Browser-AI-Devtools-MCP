// startup.go — Resumes persisted fixture recovery after daemon startup.

package qafixture

import (
	"context"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

const startupRecoveryTimeout = 30 * time.Second

type ExtensionReadinessWaiter func(context.Context, time.Duration) bool

// StartStartupRecovery installs the recovery barrier synchronously, then runs
// bounded extension reconciliation asynchronously. Callers may safely publish
// the handler after this method returns: mutations wait for the barrier.
func (handler *Handler) StartStartupRecovery(
	ctx context.Context,
	waitForExtension ExtensionReadinessWaiter,
) <-chan struct{} {
	handler.startupMu.Lock()
	if handler.startupDone != nil {
		done := handler.startupDone
		handler.startupMu.Unlock()
		return done
	}
	done := make(chan struct{})
	handler.startupDone = done
	handler.startupMu.Unlock()
	go func() {
		defer close(done)
		handler.recoverAtStartup(ctx, waitForExtension)
	}()
	return done
}

func (handler *Handler) waitForStartupRecovery(ctx context.Context) error {
	handler.startupMu.RLock()
	done := handler.startupDone
	handler.startupMu.RUnlock()
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return context.Canceled
	}
}

func (handler *Handler) recoverAtStartup(ctx context.Context, waitForExtension ExtensionReadinessWaiter) {
	handler.lifecycleMu.Lock()
	defer handler.lifecycleMu.Unlock()
	records := handler.registry.Records()
	for _, record := range records {
		handler.reportPendingRecovery(record.CorrelationID, "A persisted QA fixture transaction is awaiting browser state restoration.")
	}
	if !waitForExtension(ctx, startupRecoveryTimeout) {
		for _, record := range records {
			handler.reportPendingRecovery(record.CorrelationID, "QA fixture recovery is pending because the extension is unavailable.")
		}
		return
	}
	snapshotIDs := make([]string, 0, len(records))
	for _, record := range records {
		snapshotIDs = append(snapshotIDs, record.SnapshotID)
	}
	pruned, err := handler.reconcile(ctx, snapshotIDs)
	if err != nil {
		handler.diagnostics.Report(statediag.Diagnostic{
			Name: "environment_snapshot_reconciliation", Detail: "Extension snapshot reconciliation failed.",
			Fix: "Reload the extension and retry before applying another QA fixture.",
		})
	} else {
		if pruned > 0 {
			handler.diagnostics.Report(statediag.Diagnostic{
				Name: "environment_snapshot_reconciliation", Detail: "Unowned extension recovery snapshots were pruned without exposing their contents.",
				Fix: "No action is required; durable fixture recovery records were retained.",
			})
		}
		handler.diagnostics.Resolve("environment_snapshot_reconciliation")
	}
	for _, record := range records {
		if _, restoreErr := handler.restoreTransactionLocked(ctx, record.TransactionID); restoreErr != nil {
			// EXPECTED_AGGREGATION: restoreTransactionLocked already records the
			// redacted correlated Doctor failure; startup continues so every
			// independent durable recovery obligation gets an attempt.
			continue
		}
	}
}
