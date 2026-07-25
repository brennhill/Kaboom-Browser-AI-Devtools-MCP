// Purpose: Integration tests for capture pipeline end-to-end flows.
// Docs: docs/features/feature/backend-log-streaming/index.md

// async_queue_integration_test.go — Integration test for full async queue-and-poll flow
// This test MUST pass. If it fails, the async queue architecture is broken.
// If you're seeing this test fail after a refactor, DO NOT disable it - restore the missing components.
package capture

import (
	"encoding/json"
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

	queryID, _ := capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, "")
	if queryID == "" {
		t.Fatal("CreatePendingQueryWithTimeout returned empty query ID")
	}

	// Verify command is tracked as "pending"
	cmd, found := capture.GetCommandResult(correlationID)
	if !found {
		t.Fatal("Command not registered after CreatePendingQueryWithTimeout")
	}
	if cmd.Status != "pending" {
		t.Errorf("Expected status 'pending', got '%s'", cmd.Status)
	}

	// Step 2: Extension polls for pending queries (simulate GET /pending-queries)
	pendingQueries := capture.GetPendingQueries()
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
	capture.SetQueryResult(queryID, result)

	// Verify query is no longer in pending list
	pendingQueries = capture.GetPendingQueries()
	if len(pendingQueries) != 0 {
		t.Errorf("Expected 0 pending queries after result posted, got %d", len(pendingQueries))
	}

	// Verify result is stored
	storedResult, found := capture.TakeQueryResult(queryID)
	if !found {
		t.Fatal("Result not found after SetQueryResult")
	}
	if string(storedResult) != string(result) {
		t.Errorf("Result mismatch: expected %s, got %s", result, storedResult)
	}

	// Step 5: MCP retrieves command status (simulate observe({what: 'command_result', correlation_id: '...'}))
	cmd, found = capture.GetCommandResult(correlationID)
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
	id, _ := capture.CreatePendingQueryWithTimeout(query, 1*time.Second, "")
	if id == "" {
		t.Error("CreatePendingQueryWithTimeout is broken or missing")
	}

	// Verify GetPendingQueries exists and works
	pending := capture.GetPendingQueries()
	if pending == nil {
		t.Error("GetPendingQueries is broken or missing")
	}

	// Verify SetQueryResult exists and works
	capture.SetQueryResult(id, json.RawMessage(`{}`))

	// Verify TakeQueryResult exists and works
	_, _ = capture.TakeQueryResult(id)

	// Verify correlation ID tracking methods exist
	query2 := queries.PendingQuery{
		Type:          "test",
		Params:        json.RawMessage(`{}`),
		CorrelationID: "test_corr_id",
	}
	capture.CreatePendingQueryWithTimeout(query2, 1*time.Second, "")

	// Verify GetCommandResult exists and works
	_, found := capture.GetCommandResult("test_corr_id")
	if !found {
		t.Error("GetCommandResult is broken or correlation ID tracking is missing")
	}

	// Verify GetPendingCommands exists and works
	pendingCmds := capture.GetPendingCommands()
	if pendingCmds == nil {
		t.Error("GetPendingCommands is broken or missing")
	}

	// Verify GetCompletedCommands exists and works
	completedCmds := capture.GetCompletedCommands()
	if completedCmds == nil {
		t.Error("GetCompletedCommands is broken or missing")
	}

	// Verify GetFailedCommands exists and works
	failedCmds := capture.GetFailedCommands()
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
	idA, _ := capture.CreatePendingQueryWithTimeout(queryA, queries.AsyncCommandTimeout, "client_a")

	// Client B creates command
	queryB := queries.PendingQuery{
		Type:          "execute",
		Params:        json.RawMessage(`{"script":"console.log('B')"}`),
		CorrelationID: "client_b_integration",
		TabID:         2,
	}
	idB, _ := capture.CreatePendingQueryWithTimeout(queryB, queries.AsyncCommandTimeout, "client_b")

	// Client A polls - should only see their query
	pendingA := capture.GetPendingQueriesForClient("client_a")
	if len(pendingA) != 1 {
		t.Errorf("Client A: expected 1 pending query, got %d", len(pendingA))
	}
	if len(pendingA) > 0 && pendingA[0].CorrelationID != "client_a_integration" {
		t.Errorf("Client A got wrong query: %s", pendingA[0].CorrelationID)
	}

	// Client B polls - should only see their query
	pendingB := capture.GetPendingQueriesForClient("client_b")
	if len(pendingB) != 1 {
		t.Errorf("Client B: expected 1 pending query, got %d", len(pendingB))
	}
	if len(pendingB) > 0 && pendingB[0].CorrelationID != "client_b_integration" {
		t.Errorf("Client B got wrong query: %s", pendingB[0].CorrelationID)
	}

	// Client A posts result
	resultA := json.RawMessage(`{"client":"A"}`)
	capture.SetQueryResultWithClient(idA, resultA, "client_a")

	// Client B posts result
	resultB := json.RawMessage(`{"client":"B"}`)
	capture.SetQueryResultWithClient(idB, resultB, "client_b")

	// Both commands should be tracked as complete (correlation tracking is global)
	cmdA, foundA := capture.GetCommandResult("client_a_integration")
	if !foundA || cmdA.Status != "complete" {
		t.Error("Client A command not completed correctly")
	}

	cmdB, foundB := capture.GetCommandResult("client_b_integration")
	if !foundB || cmdB.Status != "complete" {
		t.Error("Client B command not completed correctly")
	}

	t.Logf("✅ Multi-client isolation verified")
}

// TestAsyncQueueExpirationIntegration verifies command expiration works end-to-end.
func TestAsyncQueueExpirationIntegration(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Create command with short timeout (will expire)
	correlationID := "expiration_integration_test"
	query := queries.PendingQuery{
		Type:          "execute",
		Params:        json.RawMessage(`{"script":"test"}`),
		CorrelationID: correlationID,
	}

	capture.CreatePendingQueryWithTimeout(query, 120*time.Millisecond, "")

	// Initially pending
	cmd, found := capture.GetCommandResult(correlationID)
	if !found || cmd.Status != "pending" {
		t.Fatal("Command should be pending initially")
	}

	// Wait for expiration with polling (faster and less flaky than fixed long sleeps)
	deadline := time.Now().Add(800 * time.Millisecond)
	for time.Now().Before(deadline) {
		cmd, found = capture.GetCommandResult(correlationID)
		if found && cmd.Status == "expired" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Should be expired
	cmd, found = capture.GetCommandResult(correlationID)
	if !found {
		t.Fatal("Expired command should still be retrievable from failedCommands")
	}
	if cmd.Status != "expired" {
		t.Errorf("Expected status 'expired', got '%s'", cmd.Status)
	}
	if cmd.Error == "" {
		t.Error("Expired command should have error message")
	}

	// Should be in failed commands list
	failedCmds := capture.GetFailedCommands()
	foundInFailed := false
	for _, fc := range failedCmds {
		if fc.CorrelationID == correlationID {
			foundInFailed = true
			break
		}
	}
	if !foundInFailed {
		t.Error("Expired command not found in GetFailedCommands()")
	}

	t.Logf("✅ Command expiration flow verified")
}
