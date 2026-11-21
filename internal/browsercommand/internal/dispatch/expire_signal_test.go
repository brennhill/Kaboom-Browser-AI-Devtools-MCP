// Purpose: Tests for query expiration signaling and cleanup.
// Docs: docs/features/feature/query-service/index.md

// expire_signal_test.go — Tests for commandNotify signaling on command expiration.
// Verifies that ExpireCommand, expireCommandWithReason, and ExpireAllPendingQueries
// wake blocked WaitForCommand goroutines instead of making them wait the full timeout.
package dispatch

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestExpireCommand_SignalsWaiters verifies that ExpireCommand wakes WaitForCommand.
func TestExpireCommand_SignalsWaiters(t *testing.T) {
	t.Parallel()
	qd := NewQueryDispatcher()
	defer qd.Close()

	// Register a command as pending
	correlationID := "expire-signal-test"
	qd.RegisterCommand(correlationID, "q-1", 30*time.Second)

	// Start WaitForCommand in a goroutine with a long timeout
	done := make(chan struct{})
	var result *queries.CommandResult
	var found bool
	go func() {
		result, found = qd.WaitForCommand(correlationID, 15*time.Second)
		close(done)
	}()

	// Expire the command — should signal the waiter
	qd.ExpireCommand(correlationID)

	// Waiter should unblock well before the 15s timeout
	select {
	case <-done:
		if !found {
			// Command was expired and moved to terminalHistory — found via ring buffer.
			t.Log("Command expired and moved to terminalHistory (expected)")
		}
		if result != nil && result.Status != "expired" {
			t.Errorf("Expected status 'expired', got %q", result.Status)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("WaitForCommand did not unblock within 500ms after ExpireCommand")
	}
}

// TestExpireCommandWithReason_SignalsWaiters verifies that expireCommandWithReason wakes WaitForCommand.
func TestExpireCommandWithReason_SignalsWaiters(t *testing.T) {
	t.Parallel()
	qd := NewQueryDispatcher()
	defer qd.Close()

	correlationID := "expire-reason-signal-test"
	qd.RegisterCommand(correlationID, "q-2", 30*time.Second)

	done := make(chan struct{})
	var result *queries.CommandResult
	go func() {
		result, _ = qd.WaitForCommand(correlationID, 15*time.Second)
		close(done)
	}()

	// Expire with a custom reason
	qd.expireCommandWithReason(correlationID, "extension_disconnected")

	select {
	case <-done:
		if result != nil && result.Status != "expired" {
			t.Errorf("Expected status 'expired', got %q", result.Status)
		}
		if result != nil && result.Error != "extension_disconnected" {
			t.Errorf("Expected error 'extension_disconnected', got %q", result.Error)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("WaitForCommand did not unblock within 500ms after expireCommandWithReason")
	}
}

// TestExpireAllPendingQueries_SignalsWaiters verifies that ExpireAllPendingQueries
// wakes all blocked WaitForCommand goroutines.
func TestExpireAllPendingQueries_SignalsWaiters(t *testing.T) {
	t.Parallel()
	qd := NewQueryDispatcher()
	defer qd.Close()

	// Create two pending queries with correlation IDs
	qd.CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type:          "dom",
		Params:        json.RawMessage(`{}`),
		CorrelationID: "corr-a",
	}, 30*time.Second, "")

	qd.CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type:          "dom",
		Params:        json.RawMessage(`{}`),
		CorrelationID: "corr-b",
	}, 30*time.Second, "")

	// Start WaitForCommand for both
	doneA := make(chan struct{})
	doneB := make(chan struct{})

	go func() {
		qd.WaitForCommand("corr-a", 15*time.Second)
		close(doneA)
	}()
	go func() {
		qd.WaitForCommand("corr-b", 15*time.Second)
		close(doneB)
	}()

	// Expire all pending queries
	qd.ExpireAllPendingQueries("extension_disconnected")

	// Both waiters should unblock promptly
	select {
	case <-doneA:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("WaitForCommand(corr-a) did not unblock within 500ms after ExpireAllPendingQueries")
	}

	select {
	case <-doneB:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("WaitForCommand(corr-b) did not unblock within 500ms after ExpireAllPendingQueries")
	}
}

// TestExpireCommand_NoOpOnNonPending verifies that ExpireCommand does not signal
// when the command has already been completed.
func TestExpireCommand_NoOpOnNonPending(t *testing.T) {
	t.Parallel()
	qd := NewQueryDispatcher()
	defer qd.Close()

	correlationID := "already-complete"
	qd.RegisterCommand(correlationID, "q-3", 30*time.Second)
	qd.ApplyCommandResult(correlationID, "complete", json.RawMessage(`{"ok":true}`), "")

	// ExpireCommand on an already-completed command should be a no-op
	// (should not panic or signal spuriously)
	qd.ExpireCommand(correlationID)

	result, found := qd.GetCommandResult(correlationID)
	if !found {
		t.Fatal("Expected to find completed command")
	}
	if result.Status != "complete" {
		t.Errorf("Expected status 'complete', got %q", result.Status)
	}
}

// TestExpireAndComplete_ConcurrentRace verifies that concurrent ExpireCommand
// and ApplyCommandResult on the same correlation ID do not panic or corrupt state.
// One must win; the other becomes a no-op.
func TestExpireAndComplete_ConcurrentRace(t *testing.T) {
	t.Parallel()

	// Run many iterations to exercise the race window
	for i := 0; i < 50; i++ {
		qd := NewQueryDispatcher()

		correlationID := "race-test"
		qd.RegisterCommand(correlationID, "q-race", 30*time.Second)

		// Start WaitForCommand so there's an active waiter (long timeout — test controls unblock)
		done := make(chan struct{})
		go func() {
			qd.WaitForCommand(correlationID, 10*time.Second)
			close(done)
		}()

		// Fire both concurrently — exactly one should win
		start := make(chan struct{})
		var operations sync.WaitGroup
		operations.Add(2)
		go func() {
			defer operations.Done()
			<-start
			qd.ApplyCommandResult(correlationID, "complete", json.RawMessage(`{"ok":true}`), "")
		}()
		go func() {
			defer operations.Done()
			<-start
			qd.ExpireCommand(correlationID)
		}()
		close(start)
		operations.Wait()

		// Waiter must unblock well before its 10s timeout
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			qd.Close()
			t.Fatal("WaitForCommand did not unblock during concurrent expire/complete")
		}

		// Result must exist and have a terminal status
		result, found := qd.GetCommandResult(correlationID)
		if !found {
			qd.Close()
			t.Fatal("Expected to find command after concurrent expire/complete")
		}
		if result.Status != "complete" && result.Status != "expired" {
			t.Errorf("Expected terminal status, got %q", result.Status)
		}

		qd.Close()
	}
}

// TestExpireAllPendingQueries_EmptyQueue verifies no panic on empty queue.
func TestExpireAllPendingQueries_EmptyQueue(t *testing.T) {
	t.Parallel()
	qd := NewQueryDispatcher()
	defer qd.Close()

	// Should be a no-op, no panic
	qd.ExpireAllPendingQueries("no pending commands")
}

func TestCleanExpiredResultsPrunesStaleStateAndExpiresOrphans(t *testing.T) {
	t.Parallel()
	qd := NewQueryDispatcher()
	t.Cleanup(qd.Close)
	now := time.Now()
	grace := queries.AsyncCommandTimeout + 10*time.Second

	qd.mu.Lock()
	qd.queryResults["stale"] = QueryResultEntry{Result: json.RawMessage(`1`), CreatedAt: now.Add(-2 * queries.QueryResultTTL)}
	qd.queryResults["fresh"] = QueryResultEntry{Result: json.RawMessage(`2`), CreatedAt: now}
	qd.pendingQueries = append(qd.pendingQueries,
		PendingQueryEntry{Query: queries.PendingQueryResponse{ID: "orphan", CorrelationID: "corr-orphan"}, Expires: now.Add(-grace - time.Minute)},
		PendingQueryEntry{Query: queries.PendingQueryResponse{ID: "orphan-untracked"}, Expires: now.Add(-grace - time.Minute)},
		PendingQueryEntry{Query: queries.PendingQueryResponse{ID: "live", CorrelationID: "corr-live"}, Expires: now.Add(time.Minute)},
	)
	qd.mu.Unlock()

	qd.resultsMu.Lock()
	qd.activeCommands["corr-orphan"] = &queries.CommandResult{CorrelationID: "corr-orphan", Status: "pending", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	qd.activeCommands["corr-ancient"] = &queries.CommandResult{CorrelationID: "corr-ancient", Status: "pending", CreatedAt: now.Add(-2 * queries.QueryResultTTL), ExpiresAt: now.Add(time.Hour)}
	qd.resultsMu.Unlock()

	qd.cleanExpiredResults()

	qd.mu.Lock()
	_, staleKept := qd.queryResults["stale"]
	_, freshKept := qd.queryResults["fresh"]
	pending := make([]string, 0, len(qd.pendingQueries))
	for _, entry := range qd.pendingQueries {
		pending = append(pending, entry.Query.ID)
	}
	qd.mu.Unlock()
	if staleKept || !freshKept {
		t.Fatalf("results after cleanup: stale kept=%v fresh kept=%v", staleKept, freshKept)
	}
	if strings.Join(pending, ",") != "live" {
		t.Fatalf("pending after cleanup = %v, want [live]", pending)
	}

	qd.resultsMu.RLock()
	orphan := qd.activeCommands["corr-orphan"]
	_, ancientKept := qd.activeCommands["corr-ancient"]
	qd.resultsMu.RUnlock()
	if ancientKept {
		t.Fatal("command older than the result TTL was not pruned")
	}
	if orphan != nil && orphan.Status != "expired" {
		t.Fatalf("orphaned command status = %q, want expired", orphan.Status)
	}
	if result, ok := qd.GetCommandResult("corr-orphan"); !ok || result.Status != "expired" {
		t.Fatalf("orphaned command = %#v, %v; want expired outcome preserved", result, ok)
	}
}
