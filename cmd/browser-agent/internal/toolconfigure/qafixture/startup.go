// startup.go — Resumes persisted fixture recovery after daemon startup.

package qafixture

import (
	"context"
	"time"
)

const startupRecoveryTimeout = 30 * time.Second

type ExtensionReadinessWaiter func(context.Context, time.Duration) bool

func (handler *Handler) RecoverAtStartup(
	ctx context.Context,
	waitForExtension ExtensionReadinessWaiter,
) {
	records := handler.registry.Records()
	if len(records) == 0 {
		return
	}
	for _, record := range records {
		handler.reportPendingRecovery(record.CorrelationID, "A persisted QA fixture transaction is awaiting browser state restoration.")
	}
	if !waitForExtension(ctx, startupRecoveryTimeout) {
		for _, record := range records {
			handler.reportPendingRecovery(record.CorrelationID, "QA fixture recovery is pending because the extension is unavailable.")
		}
		return
	}
	handler.RecoverPending(ctx)
}
