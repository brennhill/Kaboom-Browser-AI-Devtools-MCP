// Purpose: Tests for query dispatcher routing and timeout enforcement.
// Docs: docs/features/feature/query-service/index.md

// dispatcher_test.go — Tests for QueryDispatcher init, pending queries, results, and waiting.
package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/browsercommand/query"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestQueryDispatcherArchitectureBoundaries(t *testing.T) {
	t.Parallel()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) > 10 {
		t.Fatalf("dispatch package has %d Go files; maximum is 10", len(paths))
	}
	var source strings.Builder
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		source.Write(contents)
	}
	for _, forbidden := range []string{"CompleteCommand(", "GetLastPendingQuery("} {
		if strings.Contains(source.String(), forbidden) {
			t.Errorf("dispatch retains compatibility surface %q", forbidden)
		}
	}
}

// ============================================
// NewQueryDispatcher Tests
// ============================================

func TestNewNewQueryDispatcher_Initialization(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	if qd.pendingQueries == nil {
		t.Fatal("pendingQueries should be initialized")
	}
	if len(qd.pendingQueries) != 0 {
		t.Errorf("pendingQueries len = %d, want 0", len(qd.pendingQueries))
	}
	if qd.queryResults == nil {
		t.Fatal("queryResults should be initialized")
	}
	if len(qd.queryResults) != 0 {
		t.Errorf("queryResults len = %d, want 0", len(qd.queryResults))
	}
	if qd.queryTimeout != queries.DefaultQueryTimeout {
		t.Errorf("queryTimeout = %v, want %v", qd.queryTimeout, queries.DefaultQueryTimeout)
	}
	if qd.activeCommands == nil {
		t.Fatal("activeCommands should be initialized")
	}
	if qd.terminalHistory == nil {
		t.Fatal("terminalHistory should be initialized")
	}
	if qd.commandNotify == nil {
		t.Fatal("commandNotify channel should be initialized")
	}
	if qd.queryCond == nil {
		t.Fatal("queryCond should be initialized")
	}
	if qd.queryIDCounter != 0 {
		t.Errorf("queryIDCounter = %d, want 0", qd.queryIDCounter)
	}
}

func TestQueuePositionAndDepthPreserveInsertionOrder(t *testing.T) {
	t.Parallel()
	dispatcher := NewQueryDispatcher()
	defer dispatcher.Close()
	for _, correlationID := range []string{"position-0", "position-1", "position-2"} {
		if _, err := createPending(dispatcher, queries.PendingQuery{
			Type: "dom", Params: json.RawMessage(`{}`), CorrelationID: correlationID,
		}); err != nil {
			t.Fatalf("queue %s: %v", correlationID, err)
		}
	}
	if depth := dispatcher.QueueDepth(); depth != 3 {
		t.Fatalf("queue depth = %d, want 3", depth)
	}
	for index, correlationID := range []string{"position-0", "position-1", "position-2"} {
		if position := dispatcher.QueuePosition(correlationID); position != index {
			t.Fatalf("queue position %s = %d, want %d", correlationID, position, index)
		}
	}
	if position := dispatcher.QueuePosition("missing"); position != -1 {
		t.Fatalf("missing queue position = %d, want -1", position)
	}
}

func TestNewQueryDispatcher_QueryIDsAreUniqueAcrossDaemonLifetimes(t *testing.T) {
	t.Parallel()

	first := NewQueryDispatcher()
	defer first.Close()
	second := NewQueryDispatcher()
	defer second.Close()

	firstID, err := createPending(first, queries.PendingQuery{Type: "screenshot"})
	if err != nil {
		t.Fatal(err)
	}
	secondID, err := createPending(second, queries.PendingQuery{Type: "screenshot"})
	if err != nil {
		t.Fatal(err)
	}
	if firstID == secondID {
		t.Fatalf("fresh dispatchers reused query ID %q", firstID)
	}
}

func TestNewQueryDispatcher_Close(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	qd.Close()
	// Should be safe to call multiple times
	qd.Close()
}

// ============================================
// GetSnapshot Tests
// ============================================

func TestNewQueryDispatcher_GetSnapshot_Empty(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	snap := qd.GetSnapshot()
	if snap.PendingQueryCount != 0 {
		t.Errorf("PendingQueryCount = %d, want 0", snap.PendingQueryCount)
	}
	if snap.QueryResultCount != 0 {
		t.Errorf("QueryResultCount = %d, want 0", snap.QueryResultCount)
	}
	if snap.QueryTimeout != queries.DefaultQueryTimeout {
		t.Errorf("QueryTimeout = %v, want %v", snap.QueryTimeout, queries.DefaultQueryTimeout)
	}
}

func TestNewQueryDispatcher_GetSnapshot_WithPending(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	createPending(qd, queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})
	createPending(qd, queries.PendingQuery{Type: "a11y", Params: json.RawMessage(`{}`)})

	snap := qd.GetSnapshot()
	if snap.PendingQueryCount != 2 {
		t.Errorf("PendingQueryCount = %d, want 2", snap.PendingQueryCount)
	}
}

// ============================================
// CreatePendingQuery Tests
// ============================================

func TestNewQueryDispatcher_CreatePendingQuery(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	id, _ := createPending(qd, queries.PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"body"}`),
	})

	if id == "" {
		t.Fatal("CreatePendingQuery returned empty id")
	}
	if !strings.HasPrefix(id, "q-") {
		t.Errorf("id = %q, want prefix q-", id)
	}

	pending := qd.GetPendingQueries()
	if len(pending) != 1 {
		t.Fatalf("pending len = %d, want 1", len(pending))
	}
	if pending[0].ID != id {
		t.Errorf("pending[0].ID = %q, want %q", pending[0].ID, id)
	}
	if pending[0].Type != "dom" {
		t.Errorf("pending[0].Type = %q, want dom", pending[0].Type)
	}
	if string(pending[0].Params) != `{"selector":"body"}` {
		t.Errorf("pending[0].Params = %s, want {\"selector\":\"body\"}", pending[0].Params)
	}
}

func TestNewQueryDispatcher_CreatePendingQueryWithTimeout(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	id, _ := qd.CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{}`),
		TabID:  42,
	}, 5*time.Second, "client-x")

	if id == "" {
		t.Fatal("CreatePendingQueryWithTimeout returned empty id")
	}

	clientPending := qd.GetPendingQueries()
	if len(clientPending) != 1 {
		t.Fatalf("client pending len = %d, want 1", len(clientPending))
	}
	if clientPending[0].TabID != 42 {
		t.Errorf("TabID = %d, want 42", clientPending[0].TabID)
	}
}

func TestNewQueryDispatcher_CreatePendingQuery_WithCorrelationID(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	createPending(qd, queries.PendingQuery{
		Type:          "execute_js",
		Params:        json.RawMessage(`{"script":"document.title"}`),
		CorrelationID: "corr-123",
	})

	cmd, found := qd.GetCommandResult("corr-123")
	if !found {
		t.Fatal("command not registered for correlation ID")
	}
	if cmd.Status != "pending" {
		t.Errorf("command status = %q, want pending", cmd.Status)
	}
	if cmd.CorrelationID != "corr-123" {
		t.Errorf("CorrelationID = %q, want corr-123", cmd.CorrelationID)
	}
}

func TestNewQueryDispatcher_CreatePendingQuery_QueueFull_RejectsNew(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	// Fill queue to max
	for i := 0; i < queries.MaxPendingQueries; i++ {
		_, err := createPending(qd, queries.PendingQuery{
			Type:   "dom",
			Params: json.RawMessage(`{}`),
		})
		if err != nil {
			t.Fatalf("command %d rejected unexpectedly: %v", i, err)
		}
	}

	// Next command should be rejected with queries.ErrQueueFull
	_, err := createPending(qd, queries.PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{}`),
	})
	if err != queries.ErrQueueFull {
		t.Fatalf("expected queries.ErrQueueFull, got %v", err)
	}

	// Queue should still be at max (no drops)
	pending := qd.GetPendingQueries()
	if len(pending) != queries.MaxPendingQueries {
		t.Fatalf("pending len = %d, want %d (max)", len(pending), queries.MaxPendingQueries)
	}
}

func TestNewQueryDispatcher_CreatePendingQuery_ConcurrentCapacityIsBounded(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	const attempts = 64
	start := make(chan struct{})
	type outcome struct {
		id  string
		err error
	}
	outcomes := make(chan outcome, attempts)
	var ready sync.WaitGroup
	ready.Add(attempts)

	for i := 0; i < attempts; i++ {
		go func(index int) {
			ready.Done()
			<-start
			id, err := createPending(qd, queries.PendingQuery{
				Type:   "execute",
				Params: json.RawMessage(fmt.Sprintf(`{"index":%d}`, index)),
			})
			outcomes <- outcome{id: id, err: err}
		}(i)
	}
	ready.Wait()
	close(start)

	accepted := make(map[string]struct{}, queries.MaxPendingQueries)
	rejected := 0
	for i := 0; i < attempts; i++ {
		result := <-outcomes
		switch result.err {
		case nil:
			if result.id == "" {
				t.Fatal("accepted concurrent query returned an empty ID")
			}
			accepted[result.id] = struct{}{}
		case queries.ErrQueueFull:
			rejected++
		default:
			t.Fatalf("CreatePendingQuery error = %v, want nil or queries.ErrQueueFull", result.err)
		}
	}

	if len(accepted) != queries.MaxPendingQueries {
		t.Fatalf("accepted unique IDs = %d, want queue capacity %d", len(accepted), queries.MaxPendingQueries)
	}
	if rejected != attempts-queries.MaxPendingQueries {
		t.Fatalf("rejected = %d, want %d", rejected, attempts-queries.MaxPendingQueries)
	}
	if pending := qd.GetPendingQueries(); len(pending) != queries.MaxPendingQueries {
		t.Fatalf("pending len = %d, want bounded capacity %d", len(pending), queries.MaxPendingQueries)
	}
}

func TestNewQueryDispatcher_CreatePendingQuery_QueueFull_FailsCorrelatedCommand(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	// Fill queue to max with correlated commands
	for i := 0; i < queries.MaxPendingQueries; i++ {
		_, err := createPending(qd, queries.PendingQuery{
			Type:          "dom",
			Params:        json.RawMessage(`{}`),
			CorrelationID: fmt.Sprintf("corr-%d", i),
		})
		if err != nil {
			t.Fatalf("command %d rejected unexpectedly: %v", i, err)
		}
	}

	// All original commands should still be pending
	cmd, found := qd.GetCommandResult("corr-0")
	if !found {
		t.Fatal("first command not found")
	}
	if cmd.Status != "pending" {
		t.Fatalf("first command status = %q, want pending", cmd.Status)
	}

	// Add one more with correlation ID — should be rejected and immediately failed
	_, err := createPending(qd, queries.PendingQuery{
		Type:          "dom",
		Params:        json.RawMessage(`{}`),
		CorrelationID: "corr-rejected",
	})
	if err != queries.ErrQueueFull {
		t.Fatalf("expected queries.ErrQueueFull, got %v", err)
	}

	// The rejected command should be registered and immediately failed
	cmd, found = qd.GetCommandResult("corr-rejected")
	if !found {
		t.Fatal("rejected command result not found")
	}
	if cmd.Status != "error" {
		t.Fatalf("rejected command status = %q, want error", cmd.Status)
	}
	if !strings.Contains(cmd.Error, "Queue full") {
		t.Errorf("rejected command error = %q, want substring 'Queue full'", cmd.Error)
	}

	// Original commands should NOT be dropped
	cmd, found = qd.GetCommandResult("corr-0")
	if !found {
		t.Fatal("first command should still exist after rejection")
	}
	if cmd.Status != "pending" {
		t.Fatalf("first command status = %q, want pending (not dropped)", cmd.Status)
	}
}

func TestNewQueryDispatcher_UniqueIDs(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	ids := make(map[string]bool)
	for i := 0; i < 10; i++ {
		id, _ := createPending(qd, queries.PendingQuery{
			Type:   "dom",
			Params: json.RawMessage(`{}`),
		})
		if ids[id] {
			t.Fatalf("duplicate query ID: %q", id)
		}
		ids[id] = true
	}
}

// ============================================
// SetQueryResult / TakeQueryResult Tests
// ============================================

func TestNewQueryDispatcher_SetAndGetResult(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	id, _ := createPending(qd, queries.PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"body"}`),
	})

	resultData := json.RawMessage(`{"html":"<body>test</body>"}`)
	qd.SetQueryResultWithClient(id, resultData, "")

	got, found := takeResult(qd, id)
	if !found {
		t.Fatal("TakeQueryResult returned false, want true")
	}
	if string(got) != string(resultData) {
		t.Errorf("result = %s, want %s", string(got), string(resultData))
	}

	// Second get should return not found (one-time use)
	_, found2 := takeResult(qd, id)
	if found2 {
		t.Error("second TakeQueryResult should return false (one-time use)")
	}
}

func TestNewQueryDispatcher_SetResultRemovesFromPending(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	id, _ := createPending(qd, queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})

	qd.SetQueryResultWithClient(id, json.RawMessage(`{"ok":true}`), "")

	pending := qd.GetPendingQueries()
	if len(pending) != 0 {
		t.Errorf("pending queries after SetQueryResult = %d, want 0", len(pending))
	}
}

func TestNewQueryDispatcher_GetResult_NotFound(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	_, found := takeResult(qd, "nonexistent")
	if found {
		t.Error("TakeQueryResult for nonexistent id should return false")
	}
}

// ============================================
// WaitForResult Tests
// ============================================

func TestNewQueryDispatcher_WaitForResult_Immediate(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	id, _ := createPending(qd, queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})
	qd.SetQueryResultWithClient(id, json.RawMessage(`{"immediate":true}`), "")

	result, err := qd.WaitForResultContext(context.Background(), id, 1*time.Second)
	if err != nil {
		t.Fatalf("WaitForResult error = %v", err)
	}
	if string(result) != `{"immediate":true}` {
		t.Errorf("result = %s, want {\"immediate\":true}", string(result))
	}
}

func TestNewQueryDispatcher_WaitForResult_Timeout(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	id, _ := createPending(qd, queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})

	_, err := qd.WaitForResultContext(context.Background(), id, 50*time.Millisecond)
	if err == nil {
		t.Fatal("WaitForResult should timeout")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("error = %q, want timeout message", err.Error())
	}
}

func TestQueryDispatcherWaitForResultContextStopsOnCancellation(t *testing.T) {
	qd := NewQueryDispatcher()
	defer qd.Close()
	id, err := createPending(qd, queries.PendingQuery{Type: "environment_transaction_snapshot"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	_, err = qd.WaitForResultContext(ctx, id, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("cancellation took %s", elapsed)
	}
}

func TestNewQueryDispatcher_WaitForResult_Async(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	id, _ := createPending(qd, queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})

	go qd.SetQueryResultWithClient(id, json.RawMessage(`{"async":true}`), "")

	result, err := qd.WaitForResultContext(context.Background(), id, 2*time.Second)
	if err != nil {
		t.Fatalf("WaitForResult error = %v", err)
	}
	if string(result) != `{"async":true}` {
		t.Errorf("result = %s, want {\"async\":true}", string(result))
	}
}

// ============================================
// WaitForPendingQueries Tests
// ============================================

// TestNewQueryDispatcher_WaitForPendingQueries_WakesAllWaiters is the
// regression test for the single buffered queryNotify token: with N pollers
// blocked in WaitForPendingQueries, one enqueue woke only one of them and the
// rest slept until their full timeout. The close-and-rotate pattern (mirroring
// commandNotify) must wake every waiter.
func TestNewQueryDispatcher_WaitForPendingQueries_WakesAllWaiters(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	const waiters = 3
	done := make(chan struct{}, waiters)
	for i := 0; i < waiters; i++ {
		go func() {
			qd.WaitForPendingQueries(10 * time.Second)
			done <- struct{}{}
		}()
	}

	if _, err := createPending(qd, queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatalf("CreatePendingQuery error = %v", err)
	}

	for i := 0; i < waiters; i++ {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d of %d waiters woke within 2s after enqueue", i, waiters)
		}
	}
}

func TestNewQueryDispatcher_WaitForPendingQueries_ImmediateWhenNonEmpty(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	if _, err := createPending(qd, queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatalf("CreatePendingQuery error = %v", err)
	}

	start := time.Now()
	qd.WaitForPendingQueries(5 * time.Second)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("WaitForPendingQueries blocked %v with a non-empty queue", elapsed)
	}
}

// ============================================
// Lifecycle Tests
// ============================================

func TestQueryDispatcherCloseWaitsForCleanupAndIsConcurrentSafe(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	const closers = 8
	start := make(chan struct{})
	var closed sync.WaitGroup
	closed.Add(closers)
	for i := 0; i < closers; i++ {
		go func() {
			defer closed.Done()
			<-start
			qd.Close()
		}()
	}
	close(start)
	closed.Wait()

	select {
	case <-qd.cleanupDone:
		// Close is a completion barrier, not a best-effort stop request.
	default:
		t.Fatal("Close returned before the cleanup worker exited")
	}
}

func TestQueryResultTTLSupportsMultiStepAgents(t *testing.T) {
	t.Parallel()
	if queries.QueryResultTTL != 5*time.Minute {
		t.Fatalf("queries.QueryResultTTL = %v, want 5m", queries.QueryResultTTL)
	}
}

// createPending queues a query with the default timeout and no client scope.
func createPending(qd *QueryDispatcher, query queries.PendingQuery) (string, error) {
	return qd.CreatePendingQueryWithTimeout(query, queries.DefaultQueryTimeout, "")
}

// takeResult consumes a stored result without waiting.
func takeResult(qd *QueryDispatcher, id string) (json.RawMessage, bool) {
	result, err := qd.WaitForResultContext(context.Background(), id, 0)
	return result, err == nil
}
