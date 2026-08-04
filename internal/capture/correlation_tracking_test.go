// Purpose: Tests for capture request correlation tracking.
// Docs: docs/features/feature/backend-log-streaming/index.md

// correlation_tracking_test.go — Test correlation ID tracking for async commands
// Ensures AI always knows command status: pending, complete, expired
package capture

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// TestCorrelationIDTracking verifies command lifecycle tracking
func TestCorrelationIDTracking(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Create async command with correlation ID
	correlationID := "test_cmd_12345"
	query := queries.PendingQuery{
		Type:          "execute",
		Params:        json.RawMessage(`{"script":"console.log('test')"}`),
		CorrelationID: correlationID,
	}

	queryID, _ := capture.Queries().CreatePendingQueryWithTimeout(query, 5*time.Second, "")

	// Command should be "pending"
	cmd, found := capture.Queries().GetCommandResult(correlationID)
	if !found {
		t.Fatal("Command not found after creation")
	}
	if cmd.Status != "pending" {
		t.Errorf("Expected status 'pending', got '%s'", cmd.Status)
	}
	if cmd.CorrelationID != correlationID {
		t.Errorf("Expected correlation ID '%s', got '%s'", correlationID, cmd.CorrelationID)
	}

	// Simulate extension completing the command
	result := json.RawMessage(`{"success": true}`)
	capture.Queries().SetQueryResult(queryID, result)

	// Command should be "complete"
	cmd, found = capture.Queries().GetCommandResult(correlationID)
	if !found {
		t.Fatal("Command not found after completion")
	}
	if cmd.Status != "complete" {
		t.Errorf("Expected status 'complete', got '%s'", cmd.Status)
	}
	if string(cmd.Result) != string(result) {
		t.Errorf("Result mismatch: expected %s, got %s", result, cmd.Result)
	}
	if cmd.CompletedAt.IsZero() {
		t.Error("CompletedAt should be set")
	}
}

// TestCorrelationIDExpiration verifies command expires after timeout
func TestCorrelationIDExpiration(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	correlationID := "test_expired_67890"
	query := queries.PendingQuery{
		Type:          "execute",
		Params:        json.RawMessage(`{"script":"test"}`),
		CorrelationID: correlationID,
	}

	capture.Queries().CreatePendingQueryWithTimeout(query, 5*time.Second, "")

	// Command starts as "pending"
	cmd, found := capture.Queries().GetCommandResult(correlationID)
	if !found {
		t.Fatal("Command not found after creation")
	}
	if cmd.Status != "pending" {
		t.Errorf("Expected status 'pending', got '%s'", cmd.Status)
	}

	// Correlation tracking owns the lifecycle projection, not timer scheduling;
	// drive the canonical expiration transition explicitly. Dedicated query
	// expiration tests cover deadline detection and cleanup signaling.
	capture.Queries().ExpireCommand(correlationID)

	// Command should be "expired" and moved to failedCommands
	cmd, found = capture.Queries().GetCommandResult(correlationID)
	if !found {
		t.Fatal("Expired command should still be retrievable from failedCommands")
	}
	if cmd.Status != "expired" {
		t.Errorf("Expected status 'expired', got '%s'", cmd.Status)
	}
	if cmd.Error == "" {
		t.Error("Expired command should have error message")
	}
}

// TestCorrelationIDListCommands verifies listing commands by status
func TestCorrelationIDListCommands(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Create 3 pending commands
	for i := 0; i < 3; i++ {
		query := queries.PendingQuery{
			Type:          "execute",
			Params:        json.RawMessage(`{"script":"test"}`),
			CorrelationID: "pending_" + string(rune('a'+i)),
		}
		capture.Queries().CreatePendingQueryWithTimeout(query, 10*time.Second, "")
	}

	// Complete 2 commands
	query1 := queries.PendingQuery{
		Type:          "execute",
		Params:        json.RawMessage(`{"script":"test"}`),
		CorrelationID: "completed_1",
	}
	id1, _ := capture.Queries().CreatePendingQueryWithTimeout(query1, 10*time.Second, "")
	capture.Queries().SetQueryResult(id1, json.RawMessage(`{"ok":true}`))

	query2 := queries.PendingQuery{
		Type:          "execute",
		Params:        json.RawMessage(`{"script":"test"}`),
		CorrelationID: "completed_2",
	}
	id2, _ := capture.Queries().CreatePendingQueryWithTimeout(query2, 10*time.Second, "")
	capture.Queries().SetQueryResult(id2, json.RawMessage(`{"ok":true}`))

	// Create 1 expired command
	expiredQuery := queries.PendingQuery{
		Type:          "execute",
		Params:        json.RawMessage(`{"script":"test"}`),
		CorrelationID: "expired_1",
	}
	capture.Queries().CreatePendingQueryWithTimeout(expiredQuery, 5*time.Second, "")
	capture.Queries().ExpireCommand("expired_1")

	// Check counts
	pending := capture.Queries().GetPendingCommands()
	if len(pending) != 3 {
		t.Errorf("Expected 3 pending commands, got %d", len(pending))
	}

	completed := capture.Queries().GetCompletedCommands()
	if len(completed) != 2 {
		t.Errorf("Expected 2 completed commands, got %d", len(completed))
	}

	failed := capture.Queries().GetFailedCommands()
	if len(failed) != 1 {
		t.Fatalf("Expected 1 failed command, got %d", len(failed))
	}

	// Verify failed command details
	if failed[0].CorrelationID != "expired_1" {
		t.Errorf("Expected failed command correlation_id 'expired_1', got '%s'", failed[0].CorrelationID)
	}
	if failed[0].Status != "expired" {
		t.Errorf("Expected failed command status 'expired', got '%s'", failed[0].Status)
	}
}

// TestCorrelationIDNoTracking verifies commands without correlation ID are not tracked
func TestCorrelationIDNoTracking(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Create command without correlation ID (synchronous query)
	query := queries.PendingQuery{
		Type:   "dom_query",
		Params: json.RawMessage(`{"selector":"#test"}`),
		// No CorrelationID
	}

	capture.Queries().CreatePendingQueryWithTimeout(query, 2*time.Second, "")

	// Should have no tracked commands
	pending := capture.Queries().GetPendingCommands()
	if len(pending) != 0 {
		t.Errorf("Expected 0 tracked commands (no correlation ID), got %d", len(pending))
	}
}

// TestCorrelationIDMultiClient verifies client isolation doesn't affect tracking
func TestCorrelationIDMultiClient(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Client A creates command
	queryA := queries.PendingQuery{
		Type:          "execute",
		Params:        json.RawMessage(`{"script":"test"}`),
		CorrelationID: "client_a_cmd",
	}
	idA, _ := capture.Queries().CreatePendingQueryWithTimeout(queryA, 10*time.Second, "client_a")

	// Client B creates command
	queryB := queries.PendingQuery{
		Type:          "execute",
		Params:        json.RawMessage(`{"script":"test"}`),
		CorrelationID: "client_b_cmd",
	}
	idB, _ := capture.Queries().CreatePendingQueryWithTimeout(queryB, 10*time.Second, "client_b")

	// Both should be pending
	pending := capture.Queries().GetPendingCommands()
	if len(pending) != 2 {
		t.Errorf("Expected 2 pending commands, got %d", len(pending))
	}

	// Client A completes their command
	capture.Queries().SetQueryResultWithClient(idA, json.RawMessage(`{"ok":true}`), "client_a")

	// Check command status (correlation tracking is NOT client-isolated)
	cmdA, found := capture.Queries().GetCommandResult("client_a_cmd")
	if !found {
		t.Fatal("Client A command not found")
	}
	if cmdA.Status != "complete" {
		t.Errorf("Expected client A command to be complete, got '%s'", cmdA.Status)
	}

	// Client B command still pending
	cmdB, found := capture.Queries().GetCommandResult("client_b_cmd")
	if !found {
		t.Fatal("Client B command not found")
	}
	if cmdB.Status != "pending" {
		t.Errorf("Expected client B command to be pending, got '%s'", cmdB.Status)
	}

	// Client B completes their command
	capture.Queries().SetQueryResultWithClient(idB, json.RawMessage(`{"ok":true}`), "client_b")

	// Both should be complete
	completed := capture.Queries().GetCompletedCommands()
	if len(completed) != 2 {
		t.Errorf("Expected 2 completed commands, got %d", len(completed))
	}
}
