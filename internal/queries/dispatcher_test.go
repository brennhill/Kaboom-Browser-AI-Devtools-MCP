// Purpose: Tests for query dispatcher routing and timeout enforcement.
// Docs: docs/features/feature/query-service/index.md

// dispatcher_test.go — Tests for QueryDispatcher init, pending queries, results, and waiting.
package queries

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

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
	if qd.queryTimeout != DefaultQueryTimeout {
		t.Errorf("queryTimeout = %v, want %v", qd.queryTimeout, DefaultQueryTimeout)
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

func TestNewQueryDispatcher_QueryIDsAreUniqueAcrossDaemonLifetimes(t *testing.T) {
	t.Parallel()

	first := NewQueryDispatcher()
	defer first.Close()
	second := NewQueryDispatcher()
	defer second.Close()

	firstID, err := first.CreatePendingQuery(PendingQuery{Type: "screenshot"})
	if err != nil {
		t.Fatal(err)
	}
	secondID, err := second.CreatePendingQuery(PendingQuery{Type: "screenshot"})
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
	if snap.QueryTimeout != DefaultQueryTimeout {
		t.Errorf("QueryTimeout = %v, want %v", snap.QueryTimeout, DefaultQueryTimeout)
	}
}

func TestNewQueryDispatcher_GetSnapshot_WithPending(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.CreatePendingQuery(PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})
	qd.CreatePendingQuery(PendingQuery{Type: "a11y", Params: json.RawMessage(`{}`)})

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

	id, _ := qd.CreatePendingQuery(PendingQuery{
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

func TestNewQueryDispatcher_CreatePendingQueryWithClient(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	id, _ := qd.CreatePendingQueryWithClient(PendingQuery{
		Type:   "a11y",
		Params: json.RawMessage(`{}`),
	}, "client-1")

	if id == "" {
		t.Fatal("CreatePendingQueryWithClient returned empty id")
	}

	clientPending := qd.GetPendingQueriesForClient("client-1")
	if len(clientPending) != 1 {
		t.Fatalf("client pending len = %d, want 1", len(clientPending))
	}

	otherClient := qd.GetPendingQueriesForClient("client-2")
	if len(otherClient) != 0 {
		t.Fatalf("other client pending len = %d, want 0", len(otherClient))
	}
}

func TestNewQueryDispatcher_CreatePendingQueryWithTimeout(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	id, _ := qd.CreatePendingQueryWithTimeout(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{}`),
		TabID:  42,
	}, 5*time.Second, "client-x")

	if id == "" {
		t.Fatal("CreatePendingQueryWithTimeout returned empty id")
	}

	clientPending := qd.GetPendingQueriesForClient("client-x")
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

	qd.CreatePendingQuery(PendingQuery{
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
	for i := 0; i < MaxPendingQueries; i++ {
		_, err := qd.CreatePendingQuery(PendingQuery{
			Type:   "dom",
			Params: json.RawMessage(`{}`),
		})
		if err != nil {
			t.Fatalf("command %d rejected unexpectedly: %v", i, err)
		}
	}

	// Next command should be rejected with ErrQueueFull
	_, err := qd.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{}`),
	})
	if err != ErrQueueFull {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}

	// Queue should still be at max (no drops)
	pending := qd.GetPendingQueries()
	if len(pending) != MaxPendingQueries {
		t.Fatalf("pending len = %d, want %d (max)", len(pending), MaxPendingQueries)
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
			id, err := qd.CreatePendingQuery(PendingQuery{
				Type:   "execute",
				Params: json.RawMessage(fmt.Sprintf(`{"index":%d}`, index)),
			})
			outcomes <- outcome{id: id, err: err}
		}(i)
	}
	ready.Wait()
	close(start)

	accepted := make(map[string]struct{}, MaxPendingQueries)
	rejected := 0
	for i := 0; i < attempts; i++ {
		result := <-outcomes
		switch result.err {
		case nil:
			if result.id == "" {
				t.Fatal("accepted concurrent query returned an empty ID")
			}
			accepted[result.id] = struct{}{}
		case ErrQueueFull:
			rejected++
		default:
			t.Fatalf("CreatePendingQuery error = %v, want nil or ErrQueueFull", result.err)
		}
	}

	if len(accepted) != MaxPendingQueries {
		t.Fatalf("accepted unique IDs = %d, want queue capacity %d", len(accepted), MaxPendingQueries)
	}
	if rejected != attempts-MaxPendingQueries {
		t.Fatalf("rejected = %d, want %d", rejected, attempts-MaxPendingQueries)
	}
	if pending := qd.GetPendingQueries(); len(pending) != MaxPendingQueries {
		t.Fatalf("pending len = %d, want bounded capacity %d", len(pending), MaxPendingQueries)
	}
}

func TestNewQueryDispatcher_CreatePendingQuery_QueueFull_FailsCorrelatedCommand(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	// Fill queue to max with correlated commands
	for i := 0; i < MaxPendingQueries; i++ {
		_, err := qd.CreatePendingQuery(PendingQuery{
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
	_, err := qd.CreatePendingQuery(PendingQuery{
		Type:          "dom",
		Params:        json.RawMessage(`{}`),
		CorrelationID: "corr-rejected",
	})
	if err != ErrQueueFull {
		t.Fatalf("expected ErrQueueFull, got %v", err)
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
		id, _ := qd.CreatePendingQuery(PendingQuery{
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

	id, _ := qd.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"body"}`),
	})

	resultData := json.RawMessage(`{"html":"<body>test</body>"}`)
	qd.SetQueryResult(id, resultData)

	got, found := qd.TakeQueryResult(id)
	if !found {
		t.Fatal("TakeQueryResult returned false, want true")
	}
	if string(got) != string(resultData) {
		t.Errorf("result = %s, want %s", string(got), string(resultData))
	}

	// Second get should return not found (one-time use)
	_, found2 := qd.TakeQueryResult(id)
	if found2 {
		t.Error("second TakeQueryResult should return false (one-time use)")
	}
}

func TestNewQueryDispatcher_SetResultRemovesFromPending(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	id, _ := qd.CreatePendingQuery(PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})

	qd.SetQueryResult(id, json.RawMessage(`{"ok":true}`))

	pending := qd.GetPendingQueries()
	if len(pending) != 0 {
		t.Errorf("pending queries after SetQueryResult = %d, want 0", len(pending))
	}
}

func TestNewQueryDispatcher_GetResultForClient_Isolation(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	id, _ := qd.CreatePendingQueryWithClient(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{}`),
	}, "client-A")
	qd.SetQueryResultWithClient(id, json.RawMessage(`{"found":true}`), "client-A")

	// Client B should NOT get Client A's result
	_, foundB := qd.TakeQueryResultForClient(id, "client-B")
	if foundB {
		t.Error("client-B should not be able to access client-A's result")
	}

	// Client A should get it
	_, foundA := qd.TakeQueryResultForClient(id, "client-A")
	if !foundA {
		t.Error("client-A should be able to access its own result")
	}
}

func TestNewQueryDispatcher_GetResult_NotFound(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	_, found := qd.TakeQueryResult("nonexistent")
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

	id, _ := qd.CreatePendingQuery(PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})
	qd.SetQueryResult(id, json.RawMessage(`{"immediate":true}`))

	result, err := qd.WaitForResult(id, 1*time.Second)
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

	id, _ := qd.CreatePendingQuery(PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})

	_, err := qd.WaitForResult(id, 50*time.Millisecond)
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
	id, err := qd.CreatePendingQuery(PendingQuery{Type: "environment_transaction_snapshot"})
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

	id, _ := qd.CreatePendingQuery(PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})

	go qd.SetQueryResult(id, json.RawMessage(`{"async":true}`))

	result, err := qd.WaitForResult(id, 2*time.Second)
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

	if _, err := qd.CreatePendingQuery(PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)}); err != nil {
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

	if _, err := qd.CreatePendingQuery(PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatalf("CreatePendingQuery error = %v", err)
	}

	start := time.Now()
	qd.WaitForPendingQueries(5 * time.Second)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("WaitForPendingQueries blocked %v with a non-empty queue", elapsed)
	}
}

// ============================================
// SetQueryTimeout / GetQueryTimeout Tests
// ============================================

func TestNewQueryDispatcher_SetGetQueryTimeout(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	if got := qd.GetQueryTimeout(); got != DefaultQueryTimeout {
		t.Errorf("default timeout = %v, want %v", got, DefaultQueryTimeout)
	}

	qd.SetQueryTimeout(10 * time.Second)
	if got := qd.GetQueryTimeout(); got != 10*time.Second {
		t.Errorf("timeout after set = %v, want 10s", got)
	}
}

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
	if QueryResultTTL != 5*time.Minute {
		t.Fatalf("QueryResultTTL = %v, want 5m", QueryResultTTL)
	}
}
