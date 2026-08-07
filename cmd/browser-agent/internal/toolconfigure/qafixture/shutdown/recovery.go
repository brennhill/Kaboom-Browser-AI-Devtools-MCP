// recovery.go — Recovers pending QA fixtures before handler shutdown cancellation.

package shutdown

import (
	"context"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

// Run performs bounded fixture recovery before cancelling dependent runtime work.
func Run(recovery interface {
	RecoverPending(context.Context) []string
}, diagnostics statediag.Reporter, cancel context.CancelFunc) {
	if recovery != nil {
		recoveryCtx, stopRecovery := context.WithTimeout(context.Background(), 5*time.Second)
		failures := recovery.RecoverPending(recoveryCtx)
		stopRecovery()
		if len(failures) > 0 && diagnostics != nil {
			diagnostics.Report(statediag.Diagnostic{
				Name:   "fixture_transaction_shutdown_recovery",
				Detail: "One or more fixture transactions could not be restored during daemon shutdown.",
				Fix:    "Restart Kaboom, reconnect the extension, and inspect fixture transaction status.",
			})
		}
	}
	if cancel != nil {
		cancel()
	}
}
