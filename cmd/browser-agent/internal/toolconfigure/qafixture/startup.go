// startup.go — Resumes persisted fixture recovery after daemon startup.

package qafixture

import (
	"context"
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

const startupRecoveryTimeout = 30 * time.Second

type ExtensionReadinessWaiter func(context.Context, time.Duration) bool

func (handler *Handler) RecoverAtStartup(
	ctx context.Context,
	waitForExtension ExtensionReadinessWaiter,
	diagnostics interface {
		statediag.Reporter
		statediag.Resolver
	},
) {
	count := handler.registry.Len()
	if count == 0 {
		diagnostics.Resolve("fixture_transaction_recovery")
		return
	}
	if !waitForExtension(ctx, startupRecoveryTimeout) {
		diagnostics.Report(statediag.Diagnostic{
			Name:   "fixture_transaction_recovery",
			Detail: fmt.Sprintf("%d fixture transaction(s) remain pending because the extension is unavailable.", count),
			Fix:    "Reconnect the extension; Kaboom will retry recovery on the next daemon start or explicit restore.",
		})
		return
	}
	failures := handler.RecoverPending(ctx)
	if len(failures) > 0 {
		diagnostics.Report(statediag.Diagnostic{
			Name:   "fixture_transaction_recovery",
			Detail: fmt.Sprintf("%d of %d fixture transaction recovery attempt(s) failed with stable diagnostic codes.", len(failures), count),
			Fix:    "Inspect transaction status and retry configure({what:'qa_fixture',fixture_action:'restore'}).",
		})
		return
	}
	diagnostics.Resolve("fixture_transaction_recovery")
}
