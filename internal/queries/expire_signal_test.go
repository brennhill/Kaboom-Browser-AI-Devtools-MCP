// Purpose: Tests for query expiration signaling and cleanup.
// Docs: docs/features/feature/query-service/index.md

// expire_signal_test.go — Tests for commandNotify signaling on command expiration.
// Verifies that ExpireCommand, expireCommandWithReason, and ExpireAllPendingQueries
// wake blocked WaitForCommand goroutines instead of making them wait the full timeout.
package queries

import (
	"encoding/json"
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
	var result *CommandResult
	var found bool
	go func() {
		result, found = qd.WaitForCommand(correlationID, 15*time.Second)
		close(done)
	}()

	// Give the waiter time to block
	time.Sleep(50 * time.Millisecond)

	// Expire the command — should signal the waiter
	qd.ExpireCommand(correlationID)

	// Waiter should unblock well before the 15s timeout
	select {
	case <-done:
		if !found {
			// Command was expired and moved to failedCommands — found via ring buffer
			t.Log("Command expired and moved to failedCommands (expected)")
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
	var result *CommandResult
	go func() {
		result, _ = qd.WaitForCommand(correlationID, 15*time.Second)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

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
	qd.CreatePendingQueryWithTimeout(PendingQuery{
		Type:          "dom",
		Params:        json.RawMessage(`{}`),
		CorrelationID: "corr-a",
	}, 30*time.Second, "")

	qd.CreatePendingQueryWithTimeout(PendingQuery{
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

	time.Sleep(50 * time.Millisecond)

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

		// Let waiter reach the select
		time.Sleep(10 * time.Millisecond)

		// Fire both concurrently — exactly one should win
		go qd.ApplyCommandResult(correlationID, "complete", json.RawMessage(`{"ok":true}`), "")
		go qd.ExpireCommand(correlationID)

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
