// recovery_test.go — Tests fixture recovery and cancellation ordering.

package shutdown

import (
	"context"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

type recoveryStub struct {
	cancelled context.Context
	failures  []string
	called    bool
}

func (stub *recoveryStub) RecoverPending(context.Context) []string {
	stub.called = true
	if stub.cancelled.Err() != nil {
		stub.failures = append(stub.failures, "cancelled_before_recovery")
	}
	return stub.failures
}

func TestRunRecoversBeforeCancellationAndReportsFailures(t *testing.T) {
	t.Parallel()
	shutdownCtx, cancel := context.WithCancel(context.Background())
	diagnostics := statediag.NewCollector()
	recovery := &recoveryStub{cancelled: shutdownCtx, failures: []string{"restore_failed"}}
	Run(recovery, diagnostics, cancel)
	if !recovery.called || shutdownCtx.Err() == nil {
		t.Fatalf("shutdown ordering = called:%t cancelled:%v", recovery.called, shutdownCtx.Err())
	}
	if len(recovery.failures) != 1 {
		t.Fatalf("recovery ran after cancellation: %v", recovery.failures)
	}
	incidents := diagnostics.Snapshot()
	if len(incidents) != 1 || incidents[0].Name != "fixture_transaction_shutdown_recovery" || incidents[0].Fix == "" {
		t.Fatalf("shutdown diagnostics = %#v", incidents)
	}
}

func TestRunHandlesAbsentCollaborators(t *testing.T) {
	t.Parallel()
	Run(nil, nil, nil)
}
