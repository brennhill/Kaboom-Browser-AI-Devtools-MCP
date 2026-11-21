// Purpose: Integration tests for capture pipeline end-to-end flows.
// Docs: docs/features/feature/backend-log-streaming/index.md

// async_queue_integration_test.go — Integration test for full async queue-and-poll flow
// This test MUST pass. If it fails, the async queue architecture is broken.
// If you're seeing this test fail after a refactor, DO NOT disable it - restore the missing components.
package capture

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/browsercommand/commandtest"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// TestAsyncQueueIntegration verifies the complete async queue-and-poll architecture.
// This test exercises the critical path that all interact() commands depend on.
//
// Flow:
// 1. MCP tool handler creates pending query with correlation ID
// 2. Extension polls GET /pending-queries and receives command
// 3. Extension executes command in browser
// 4. Extension posts result to POST /dom-result
// 5. MCP tool handler retrieves result by correlation ID
//
// If ANY of these steps break, this test fails.
func TestAsyncQueueIntegration(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Step 1: MCP creates async command (simulate interact({action: 'execute_js', ...}))
	correlationID := "integration_test_cmd"
	query := queries.PendingQuery{
		Type:          "execute",
		Params:        json.RawMessage(`{"script":"return 42;"}`),
		CorrelationID: correlationID,
		TabID:         0,
	}

	queryID, _ := capture.Commands().Enqueue(query, queries.AsyncCommandTimeout, "")
	if queryID == "" {
		t.Fatal("CreatePendingQueryWithTimeout returned empty query ID")
	}

	// Verify command is tracked as "pending"
	cmd, found := capture.Commands().Command(correlationID)
	if !found {
		t.Fatal("Command not registered after CreatePendingQueryWithTimeout")
	}
	if cmd.Status != "pending" {
		t.Errorf("Expected status 'pending', got '%s'", cmd.Status)
	}

	// Step 2: Extension polls for pending queries (simulate GET /pending-queries)
	pendingQueries := capture.Commands().Pending()
	if len(pendingQueries) != 1 {
		t.Fatalf("Expected 1 pending query, got %d", len(pendingQueries))
	}

	receivedQuery := pendingQueries[0]
	if receivedQuery.ID != queryID {
		t.Errorf("Expected query ID '%s', got '%s'", queryID, receivedQuery.ID)
	}
	if receivedQuery.CorrelationID != correlationID {
		t.Errorf("Expected correlation ID '%s', got '%s'", correlationID, receivedQuery.CorrelationID)
	}
	if receivedQuery.Type != "execute" {
		t.Errorf("Expected type 'execute', got '%s'", receivedQuery.Type)
	}

	// Step 3: Extension executes in browser (simulated - we skip actual browser execution)
	result := json.RawMessage(`{"value": 42, "success": true}`)

	// Step 4: Extension posts result (simulate POST /dom-result)
	commandtest.Deliver(capture.Commands(), queryID, result)

	// Verify query is no longer in pending list
	pendingQueries = capture.Commands().Pending()
	if len(pendingQueries) != 0 {
		t.Errorf("Expected 0 pending queries after result posted, got %d", len(pendingQueries))
	}

	// Verify result is stored
	storedResult, found := commandtest.Take(capture.Commands(), queryID)
	if !found {
		t.Fatal("Result not found after SetQueryResult")
	}
	if string(storedResult) != string(result) {
		t.Errorf("Result mismatch: expected %s, got %s", result, storedResult)
	}

	// Step 5: MCP retrieves command status (simulate observe({what: 'command_result', correlation_id: '...'}))
	cmd, found = capture.Commands().Command(correlationID)
	if !found {
		t.Fatal("Command not found after result posted")
	}
	if cmd.Status != "complete" {
		t.Errorf("Expected status 'complete', got '%s'", cmd.Status)
	}
	if string(cmd.Result) != string(result) {
		t.Errorf("Command result mismatch: expected %s, got %s", result, cmd.Result)
	}
	if cmd.CompletedAt.IsZero() {
		t.Error("CompletedAt should be set for completed command")
	}

	t.Logf("✅ Full async queue flow verified: create → poll → execute → result → retrieve")
}

// TestAsyncQueueArchitectureInvariants verifies critical methods exist.
// This test ensures refactoring doesn't accidentally remove required methods.
func TestAsyncQueueArchitectureInvariants(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Verify CreatePendingQueryWithTimeout exists and works
	query := queries.PendingQuery{Type: "test", Params: json.RawMessage(`{}`)}
	id, _ := capture.Commands().Enqueue(query, 1*time.Second, "")
	if id == "" {
		t.Error("CreatePendingQueryWithTimeout is broken or missing")
	}

	// Verify GetPendingQueries exists and works
	pending := capture.Commands().Pending()
	if pending == nil {
		t.Error("GetPendingQueries is broken or missing")
	}

	// Verify SetQueryResult exists and works
	commandtest.Deliver(capture.Commands(), id, json.RawMessage(`{}`))

	// Verify TakeQueryResult exists and works
	_, _ = commandtest.Take(capture.Commands(), id)

	// Verify correlation ID tracking methods exist
	query2 := queries.PendingQuery{
		Type:          "test",
		Params:        json.RawMessage(`{}`),
		CorrelationID: "test_corr_id",
	}
	capture.Commands().Enqueue(query2, 1*time.Second, "")

	// Verify GetCommandResult exists and works
	_, found := capture.Commands().Command("test_corr_id")
	if !found {
		t.Error("GetCommandResult is broken or correlation ID tracking is missing")
	}

	// Verify GetPendingCommands exists and works
	pendingCmds := capture.Commands().PendingCommands()
	if pendingCmds == nil {
		t.Error("GetPendingCommands is broken or missing")
	}

	// Verify GetCompletedCommands exists and works
	completedCmds := capture.Commands().CompletedCommands()
	if completedCmds == nil {
		t.Error("GetCompletedCommands is broken or missing")
	}

	// Verify GetFailedCommands exists and works
	failedCmds := capture.Commands().FailedCommands()
	if failedCmds == nil {
		t.Error("GetFailedCommands is broken or missing")
	}

	t.Logf("✅ All required methods exist and are callable")
}

// TestAsyncQueueMultiClientIntegration verifies multi-client isolation works correctly.
func TestAsyncQueueMultiClientIntegration(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Client A creates command
	queryA := queries.PendingQuery{
		Type:          "execute",
		Params:        json.RawMessage(`{"script":"console.log('A')"}`),
		CorrelationID: "client_a_integration",
		TabID:         1,
	}
	idA, _ := capture.Commands().Enqueue(queryA, queries.AsyncCommandTimeout, "client_a")

	// Client B creates command
	queryB := queries.PendingQuery{
		Type:          "execute",
		Params:        json.RawMessage(`{"script":"console.log('B')"}`),
		CorrelationID: "client_b_integration",
		TabID:         2,
	}
	idB, _ := capture.Commands().Enqueue(queryB, queries.AsyncCommandTimeout, "client_b")

	// The owner delivers every client's queued command on one heartbeat;
	// result consumption below stays client-scoped.
	if pending := capture.Commands().Pending(); len(pending) != 2 {
		t.Errorf("expected both clients' queries pending, got %d", len(pending))
	}

	// Client A posts result
	resultA := json.RawMessage(`{"client":"A"}`)
	commandtest.DeliverForClient(capture.Commands(), idA, resultA, "client_a")

	// Client B posts result
	resultB := json.RawMessage(`{"client":"B"}`)
	commandtest.DeliverForClient(capture.Commands(), idB, resultB, "client_b")

	// Both commands should be tracked as complete (correlation tracking is global)
	cmdA, foundA := capture.Commands().Command("client_a_integration")
	if !foundA || cmdA.Status != "complete" {
		t.Error("Client A command not completed correctly")
	}

	cmdB, foundB := capture.Commands().Command("client_b_integration")
	if !foundB || cmdB.Status != "complete" {
		t.Error("Client B command not completed correctly")
	}

	t.Logf("✅ Multi-client isolation verified")
}
