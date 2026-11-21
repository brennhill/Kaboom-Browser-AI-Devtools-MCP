// local_test.go — Tests daemon-local commands, daemon completions and read views on the owner.
// Docs: docs/features/feature/query-service/index.md

package browsercommand

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRegisterLocalCreatesPendingCommandWithoutDelivery(t *testing.T) {
	t.Parallel()
	owner := newTestOwner(t)
	owner.RegisterLocal("ann-1", time.Minute)

	assertStatus(t, owner, "ann-1", "pending")
	if depth := owner.QueueDepth(); depth != 0 {
		t.Fatalf("QueueDepth() = %d, want 0 (local commands are never delivered)", depth)
	}
	if got := len(owner.PendingCommands()); got != 1 {
		t.Fatalf("PendingCommands() len = %d, want 1", got)
	}
}

func TestDaemonSourceCompletesLocalCommand(t *testing.T) {
	t.Parallel()
	owner := newTestOwner(t)
	owner.RegisterLocal("ann-2", time.Minute)

	if got := owner.SubmitResult(Result{Source: SourceDaemon, CorrelationID: "ann-2", Status: "complete"}); got != OutcomeApplied {
		t.Fatalf("daemon completion outcome = %q, want applied", got)
	}
	command := assertStatus(t, owner, "ann-2", "complete")
	if command.Result != nil {
		t.Fatalf("Result = %s, want nil payload", command.Result)
	}
	if got := len(owner.CompletedCommands()); got != 1 {
		t.Fatalf("CompletedCommands() len = %d, want 1", got)
	}
}

func TestDaemonSourceRejectsNonTerminalOrUnknownStatus(t *testing.T) {
	t.Parallel()
	owner := newTestOwner(t)
	owner.RegisterLocal("ann-3", time.Minute)

	for _, status := range []string{"", "pending", "bogus"} {
		if got := owner.SubmitResult(Result{Source: SourceDaemon, CorrelationID: "ann-3", Status: status}); got != OutcomeRejected {
			t.Fatalf("status %q: outcome = %q, want rejected", status, got)
		}
	}
	assertStatus(t, owner, "ann-3", "pending")
}

func TestDaemonSourceCompletionCarriesPayload(t *testing.T) {
	t.Parallel()
	owner := newTestOwner(t)
	owner.RegisterLocal("ann-4", time.Minute)
	owner.SubmitResult(Result{Source: SourceDaemon, CorrelationID: "ann-4", Status: "complete", Payload: json.RawMessage(`{"n":1}`)})

	command := assertStatus(t, owner, "ann-4", "complete")
	if string(command.Result) != `{"n":1}` {
		t.Fatalf("Result = %s", command.Result)
	}
}

func TestReadViewsReportFailuresTracesAndSnapshot(t *testing.T) {
	t.Parallel()
	owner := newTestOwner(t)
	enqueueCommand(t, owner, "corr-f")
	owner.SubmitResult(Result{Source: SourceDaemon, CorrelationID: "corr-f", Status: "error", Error: "boom"})

	if got := len(owner.FailedCommands()); got != 1 {
		t.Fatalf("FailedCommands() len = %d, want 1", got)
	}
	if got := len(owner.RecentTraces(10)); got == 0 {
		t.Fatal("RecentTraces() empty, want the failed command trace")
	}
	if snapshot := owner.Snapshot(); snapshot.PendingQueryCount != 1 {
		t.Fatalf("Snapshot().PendingQueryCount = %d, want 1 (undelivered query)", snapshot.PendingQueryCount)
	}
}
