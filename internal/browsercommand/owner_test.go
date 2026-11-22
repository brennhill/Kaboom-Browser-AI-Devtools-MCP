// owner_test.go — Pins result-source validation and generation-gated exchange behavior.
// Docs: docs/features/feature/query-service/index.md

package browsercommand

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/browsercommand/query"
)

func newTestOwner(t *testing.T) *Owner {
	t.Helper()
	owner := NewOwner()
	t.Cleanup(owner.Close)
	return owner
}

func enqueueCommand(t *testing.T, owner *Owner, correlationID string) string {
	t.Helper()
	queryID, err := owner.Enqueue(queries.PendingQuery{
		Type: "browser_action", Params: json.RawMessage(`{"action":"highlight"}`), CorrelationID: correlationID,
	}, queries.AsyncCommandTimeout, "")
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	return queryID
}

func currentGate(generation uint64) GenerationGate {
	return func(apply func()) (uint64, bool) {
		apply()
		return generation, true
	}
}

func supersededGate(current uint64) GenerationGate {
	return func(func()) (uint64, bool) { return current, false }
}

func assertStatus(t *testing.T, owner *Owner, correlationID, want string) *queries.CommandResult {
	t.Helper()
	command, found := owner.Command(correlationID)
	if !found || command.Status != want {
		t.Fatalf("command %s = %#v found=%v; want status %q", correlationID, command, found, want)
	}
	return command
}

func TestSubmitResultValidatesEachSource(t *testing.T) {
	t.Parallel()
	owner := newTestOwner(t)
	queryID := enqueueCommand(t, owner, "corr-1")

	cases := []struct {
		name   string
		result Result
		want   Outcome
	}{
		{"sync results must use the gated exchange", Result{Source: SourceSync, QueryID: queryID, CorrelationID: "corr-1", Status: "complete"}, OutcomeRejected},
		{"unknown source", Result{CorrelationID: "corr-1", Status: "complete"}, OutcomeRejected},
		{"query result needs identity", Result{Source: SourceQueryResult, Status: "complete"}, OutcomeRejected},
		{"daemon failure needs an error", Result{Source: SourceDaemon, CorrelationID: "corr-1", Status: "error"}, OutcomeRejected},
		{"daemon failure needs a correlation", Result{Source: SourceDaemon, Status: "error", Error: "x"}, OutcomeRejected},
	}
	for _, tc := range cases {
		if got := owner.SubmitResult(tc.result); got != tc.want {
			t.Fatalf("%s: outcome = %q, want %q", tc.name, got, tc.want)
		}
	}
	assertStatus(t, owner, "corr-1", "pending")

	if got := owner.SubmitResult(Result{
		Source: SourceQueryResult, QueryID: queryID, CorrelationID: "corr-1", Status: "complete",
		Payload: json.RawMessage(`{"ok":true}`),
	}); got != OutcomeApplied {
		t.Fatalf("valid query result outcome = %q, want applied", got)
	}
	assertStatus(t, owner, "corr-1", "complete")
	if owner.QueueDepth() != 0 {
		t.Fatalf("queue depth = %d, want 0 after result", owner.QueueDepth())
	}
}

func TestApplySyncExchangeMutatesNothingWhenGenerationSuperseded(t *testing.T) {
	t.Parallel()
	owner := newTestOwner(t)
	queryID := enqueueCommand(t, owner, "corr-current")

	report := owner.ApplySyncExchange(supersededGate(2), SyncExchange{
		Generation:   1,
		Results:      []Result{{QueryID: queryID, CorrelationID: "corr-current", Status: "complete"}},
		LastAck:      queryID,
		Disconnected: true,
		InProgress:   []Progress{},
		Now:          time.Now(),
	})
	if report.Applied || report.CurrentGeneration != 2 {
		t.Fatalf("report = %#v, want not applied at generation 2", report)
	}
	assertStatus(t, owner, "corr-current", "pending")
	if owner.QueuePosition("corr-current") < 0 {
		t.Fatal("superseded exchange acknowledged or expired current work")
	}
}

func TestApplySyncExchangeSkipsStaleResultsInsideCurrentExchange(t *testing.T) {
	t.Parallel()
	owner := newTestOwner(t)
	queryID := enqueueCommand(t, owner, "corr-1")

	report := owner.ApplySyncExchange(currentGate(2), SyncExchange{
		Generation: 2,
		Results:    []Result{{QueryID: queryID, CorrelationID: "corr-1", Status: "complete", Generation: 1}},
		InProgress: []Progress{{CorrelationID: "corr-1", Generation: 1}},
		Now:        time.Now(),
	})
	if !report.Applied || len(report.StaleResults) != 1 || len(report.StaleProgress) != 1 {
		t.Fatalf("report = %#v, want applied with one stale result and one stale progress entry", report)
	}
	assertStatus(t, owner, "corr-1", "pending")
}

func TestApplySyncExchangeFailsStartedCommandMissingPastGrace(t *testing.T) {
	t.Parallel()
	owner := newTestOwner(t)
	queryID := enqueueCommand(t, owner, "corr-lost")
	start := time.Now()

	owner.ApplySyncExchange(currentGate(1), SyncExchange{Generation: 1, LastAck: queryID, InProgress: []Progress{}, Now: start})
	report := owner.ApplySyncExchange(currentGate(1), SyncExchange{Generation: 1, InProgress: []Progress{}, Now: start.Add(MissingProgressGrace - time.Millisecond)})
	if len(report.Lost) != 0 {
		t.Fatalf("lost before grace elapsed: %#v", report.Lost)
	}
	report = owner.ApplySyncExchange(currentGate(1), SyncExchange{Generation: 1, InProgress: []Progress{}, Now: start.Add(MissingProgressGrace)})
	if len(report.Lost) != 1 || report.Lost[0].QueryID != queryID {
		t.Fatalf("lost = %#v, want the started command", report.Lost)
	}
	command := assertStatus(t, owner, "corr-lost", "error")
	if !strings.Contains(command.Error, "extension_lost_command") {
		t.Fatalf("error = %q, want extension_lost_command", command.Error)
	}
}

func TestApplySyncExchangeKeepsReportedProgressAlive(t *testing.T) {
	t.Parallel()
	owner := newTestOwner(t)
	queryID := enqueueCommand(t, owner, "corr-alive")
	start := time.Now()

	owner.ApplySyncExchange(currentGate(1), SyncExchange{Generation: 1, LastAck: queryID, InProgress: []Progress{}, Now: start})
	report := owner.ApplySyncExchange(currentGate(1), SyncExchange{
		Generation: 1, InProgress: []Progress{{CorrelationID: "corr-alive"}}, Now: start.Add(10 * MissingProgressGrace),
	})
	if len(report.Lost) != 0 {
		t.Fatalf("reported command failed as lost: %#v", report.Lost)
	}
	assertStatus(t, owner, "corr-alive", "pending")
}

func TestFailDisconnectedTerminatesPendingCommand(t *testing.T) {
	t.Parallel()
	owner := newTestOwner(t)
	enqueueCommand(t, owner, "corr-dc")

	command, found := owner.FailDisconnected("corr-dc")
	if !found || command.Status != "error" || command.Error != "extension_disconnected" {
		t.Fatalf("command = %#v found=%v; want extension_disconnected error", command, found)
	}
}

func TestAwaitPendingReturnsQueuedWorkWithoutWaiting(t *testing.T) {
	t.Parallel()
	owner := newTestOwner(t)
	enqueueCommand(t, owner, "corr-q")
	if got := owner.AwaitPending(time.Hour); len(got) != 1 {
		t.Fatalf("AwaitPending() = %d commands, want 1", len(got))
	}
}
