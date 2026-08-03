// Purpose: Tests for query command tracing and correlation.
// Docs: docs/features/feature/query-service/index.md

package queries

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func traceStages(events []CommandTraceEvent) []string {
	stages := make([]string, 0, len(events))
	for _, evt := range events {
		stages = append(stages, evt.Stage)
	}
	return stages
}

func requireStage(t *testing.T, events []CommandTraceEvent, want string) {
	t.Helper()
	for _, evt := range events {
		if evt.Stage == want {
			return
		}
	}
	t.Fatalf("trace missing stage %q; got=%v", want, traceStages(events))
}

func indexOfStage(events []CommandTraceEvent, stage string) int {
	for i, evt := range events {
		if evt.Stage == stage {
			return i
		}
	}
	return -1
}

func TestCommandTraceLifecycle_CompleteFlow(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	queryID, _ := qd.CreatePendingQueryWithTimeout(PendingQuery{
		Type:          "browser_action",
		CorrelationID: "corr-trace-flow",
	}, 30*time.Second, "test-client")
	if queryID == "" {
		t.Fatal("CreatePendingQueryWithTimeout returned empty query ID")
	}

	cmd, found := qd.GetCommandResult("corr-trace-flow")
	if !found {
		t.Fatal("expected pending command result after queueing")
	}
	if cmd.TraceID != "corr-trace-flow" {
		t.Fatalf("trace_id = %q, want corr-trace-flow", cmd.TraceID)
	}
	if cmd.QueryID != queryID {
		t.Fatalf("query_id = %q, want %q", cmd.QueryID, queryID)
	}
	requireStage(t, cmd.TraceEvents, "queued")

	pending := qd.GetPendingQueries()
	if len(pending) != 1 {
		t.Fatalf("pending len=%d, want 1", len(pending))
	}
	if pending[0].TraceID != "corr-trace-flow" {
		t.Fatalf("pending trace_id = %q, want corr-trace-flow", pending[0].TraceID)
	}

	cmd, found = qd.GetCommandResult("corr-trace-flow")
	if !found {
		t.Fatal("expected command result after dispatch")
	}
	requireStage(t, cmd.TraceEvents, "sent")

	qd.AcknowledgePendingQuery(queryID)
	cmd, found = qd.GetCommandResult("corr-trace-flow")
	if !found {
		t.Fatal("expected command result after ack")
	}
	requireStage(t, cmd.TraceEvents, "started")

	qd.ApplyCommandResult("corr-trace-flow", "complete", json.RawMessage(`{"ok":true}`), "")
	cmd, found = qd.GetCommandResult("corr-trace-flow")
	if !found {
		t.Fatal("expected command result after completion")
	}
	requireStage(t, cmd.TraceEvents, "resolved")

	queuedIdx := indexOfStage(cmd.TraceEvents, "queued")
	sentIdx := indexOfStage(cmd.TraceEvents, "sent")
	startedIdx := indexOfStage(cmd.TraceEvents, "started")
	resolvedIdx := indexOfStage(cmd.TraceEvents, "resolved")
	if queuedIdx < 0 || sentIdx < 0 || startedIdx < 0 || resolvedIdx < 0 {
		t.Fatalf("expected full lifecycle order, got=%v", traceStages(cmd.TraceEvents))
	}
	if !(queuedIdx < sentIdx && sentIdx < startedIdx && startedIdx < resolvedIdx) {
		t.Fatalf("trace order invalid: %v", traceStages(cmd.TraceEvents))
	}
}

func TestCommandTraceLifecycle_ErrorAndTimeout(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-trace-error", "q-err", 30*time.Second)
	qd.ApplyCommandResult("corr-trace-error", "error", nil, "forced failure")
	errCmd, found := qd.GetCommandResult("corr-trace-error")
	if !found {
		t.Fatal("expected errored command result")
	}
	requireStage(t, errCmd.TraceEvents, "errored")

	qd.RegisterCommand("corr-trace-timeout", "q-timeout", 30*time.Second)
	qd.ExpireCommand("corr-trace-timeout")
	timeoutCmd, found := qd.GetCommandResult("corr-trace-timeout")
	if !found {
		t.Fatal("expected timed_out command result")
	}
	requireStage(t, timeoutCmd.TraceEvents, "timed_out")
}

func TestGetRecentCommandTraces_RespectsLimitAndIncludesTimeline(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-trace-a", "q-a", 30*time.Second)
	qd.ApplyCommandResult("corr-trace-a", "error", nil, "boom")

	qd.RegisterCommand("corr-trace-b", "q-b", 30*time.Second)
	qd.ApplyCommandResult("corr-trace-b", "complete", json.RawMessage(`{"ok":true}`), "")

	traces := qd.GetRecentCommandTraces(1)
	if len(traces) != 1 {
		t.Fatalf("trace count=%d, want 1", len(traces))
	}
	if traces[0].TraceID == "" {
		t.Fatal("trace_id should not be empty")
	}
	if len(traces[0].TraceEvents) == 0 {
		t.Fatal("trace events should not be empty")
	}
	if traces[0].TraceTimeline == "" {
		t.Fatal("trace_timeline should not be empty")
	}
	if !strings.Contains(traces[0].TraceTimeline, "queued") {
		t.Fatalf("trace_timeline = %q, expected queued stage", traces[0].TraceTimeline)
	}
}

func TestCommandTraceHelpersPreserveIdentityAndIgnoreNoise(t *testing.T) {
	t.Parallel()
	if deriveTraceID("explicit", "correlation", "query") != "explicit" ||
		deriveTraceID("", "correlation", "query") != "correlation" ||
		deriveTraceID("", "", "query") != "trace_query" || deriveTraceID("", "", "") != "" {
		t.Fatal("deriveTraceID priority changed")
	}
	if buildTraceTimeline(nil) != "" || buildTraceTimeline([]CommandTraceEvent{{}, {Stage: "queued"}, {Stage: "sent"}}) != "queued -> sent" {
		t.Fatal("trace timeline did not skip empty stages")
	}
	for status, stage := range map[string]string{
		"pending": "started", "complete": "resolved", "error": "errored",
		"cancelled": "errored", "timeout": "timed_out", "expired": "timed_out", "unknown": "errored",
	} {
		if got := traceStageFromStatus(status); got != stage {
			t.Fatalf("traceStageFromStatus(%q) = %q", status, got)
		}
	}

	qd := NewQueryDispatcher()
	defer qd.Close()
	now := time.Now()
	qd.ensureTraceContextLocked(nil, "corr", "query", "trace", now)
	cmd := &CommandResult{CorrelationID: "kept", TraceID: "kept-trace", QueryID: "kept-query", UpdatedAt: now}
	qd.ensureTraceContextLocked(cmd, "new", "new-query", "new-trace", now.Add(time.Second))
	if cmd.CorrelationID != "kept" || cmd.TraceID != "kept-trace" || cmd.QueryID != "kept-query" || !cmd.UpdatedAt.Equal(now) {
		t.Fatalf("existing trace identity was overwritten: %#v", cmd)
	}
	qd.appendTraceEventLocked(nil, "queued", "test", "pending", "", now)
	qd.appendTraceEventLocked(cmd, "", "test", "pending", "", now)
	qd.appendTraceEventLocked(cmd, "queued", "test", "pending", "", now)
	qd.appendTraceEventLocked(cmd, "queued", "test", "pending", "", now.Add(time.Second))
	if len(cmd.TraceEvents) != 1 || !cmd.UpdatedAt.Equal(now.Add(time.Second)) || !qd.hasTraceStageLocked(cmd, "queued") || qd.hasTraceStageLocked(nil, "queued") {
		t.Fatalf("trace de-duplication = %#v", cmd)
	}
	copy := copyCommandResultWithTrace(cmd)
	copy.TraceEvents[0].Stage = "changed"
	if cmd.TraceEvents[0].Stage == "changed" || copyCommandResultWithTrace(nil) != nil {
		t.Fatal("trace copy aliases internal state")
	}
}

func TestRecentCommandTracesDeduplicateAndUseCreatedAtFallback(t *testing.T) {
	t.Parallel()
	qd := NewQueryDispatcher()
	defer qd.Close()
	older := &CommandResult{CorrelationID: "same", CreatedAt: time.Now().Add(-time.Minute)}
	newer := &CommandResult{CorrelationID: "same", CreatedAt: time.Now()}
	traceOnly := &CommandResult{TraceID: "trace-only", CreatedAt: time.Now().Add(time.Minute)}
	qd.terminalHistory = []*CommandResult{older, traceOnly}
	qd.activeCommands["same"] = newer
	qd.activeCommands["nil"] = nil
	traces := qd.GetRecentCommandTraces(0)
	// Results are defensive copies, so compare stable identity instead of pointers.
	if len(traces) != 2 || traces[0].TraceID != "trace-only" || traces[1].CorrelationID != "same" {
		t.Fatalf("recent traces = %#v", traces)
	}
	qd.recordTraceEvent("", "queued", "test", "", "", time.Now())
	qd.recordTraceEvent("missing", "queued", "test", "", "", time.Now())
}
