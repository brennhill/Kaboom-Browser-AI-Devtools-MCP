// Purpose: Stores, retrieves, waits on, and expires query result payloads for sync/async tool paths.
// Why: The result lifecycle is one unit of state guarded by QueryDispatcher.mu — store, wait and
// cleanup all mutate queryResults/pendingQueries under it, so they read and review together.
// Docs: docs/features/feature/query-service/index.md
//
// Sections: configuration accessors, result store, blocking wait, TTL cleanup.

package queries

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// SetQueryTimeout sets the default timeout for queries.
func (qd *QueryDispatcher) SetQueryTimeout(timeout time.Duration) {
	qd.mu.Lock()
	defer qd.mu.Unlock()
	qd.queryTimeout = timeout
}

// GetQueryTimeout returns the current query timeout.
func (qd *QueryDispatcher) GetQueryTimeout() time.Duration {
	qd.mu.Lock()
	defer qd.mu.Unlock()
	return qd.queryTimeout
}

// QueuePosition returns the 0-based position of a command in the pending queue
// by correlation ID, or -1 if not found. Used for progress reporting.
func (qd *QueryDispatcher) QueuePosition(correlationID string) int {
	qd.mu.Lock()
	defer qd.mu.Unlock()

	for i, pq := range qd.pendingQueries {
		if pq.Query.CorrelationID == correlationID {
			return i
		}
	}
	return -1
}

// QueueDepth returns the current number of pending queries.
func (qd *QueryDispatcher) QueueDepth() int {
	qd.mu.Lock()
	defer qd.mu.Unlock()
	return len(qd.pendingQueries)
}

// ============================================
// Result store: insert from extension callbacks, one-time consume
// ============================================

// SetQueryResult stores the result for a pending query.
// Called when extension posts result back to server.
func (qd *QueryDispatcher) SetQueryResult(id string, result json.RawMessage) {
	qd.SetQueryResultWithClient(id, result, "")
}

// SetQueryResultWithClient stores a result and marks correlated command complete.
//
// Flow:
// 1. Store result in queryResults map
// 2. Remove from pendingQueries
// 3. Broadcast to wake up any WaitForResult callers
func (qd *QueryDispatcher) SetQueryResultWithClient(id string, result json.RawMessage, clientID string) {
	qd.setQueryResultWithClient(id, result, clientID, true)
}

// SetQueryResultWithClientNoCommandComplete stores result without lifecycle transition.
//
// Invariants:
// - Used when extension reports command status through ApplyCommandResult separately.
//
// Failure semantics:
// - Prevents accidental status downgrade/upgrade caused by out-of-order query/result transport.
func (qd *QueryDispatcher) SetQueryResultWithClientNoCommandComplete(id string, result json.RawMessage, clientID string) {
	qd.setQueryResultWithClient(id, result, clientID, false)
}

// setQueryResultWithClient performs result insertion + pending queue cleanup.
//
// Invariants:
// - queryCond.Broadcast is emitted after map/slice mutation so waiters observe committed state.
//
// Failure semantics:
// - Missing pending entry is tolerated; result is still stored for one-time retrieval.
// - markComplete=false leaves command lifecycle unchanged by design.
func (qd *QueryDispatcher) setQueryResultWithClient(id string, result json.RawMessage, clientID string, markComplete bool) {
	correlationID := func() string {
		qd.mu.Lock()
		defer qd.mu.Unlock()

		// Store result.
		qd.queryResults[id] = QueryResultEntry{
			Result:    cloneJSON(result),
			ClientID:  clientID,
			CreatedAt: time.Now(),
		}

		// Find correlation ID before removing from pending.
		var corr string
		for _, pq := range qd.pendingQueries {
			if pq.Query.ID == id {
				corr = pq.Query.CorrelationID
				break
			}
		}

		// Remove from pending.
		remaining := qd.pendingQueries[:0]
		for _, pq := range qd.pendingQueries {
			if pq.Query.ID != id {
				remaining = append(remaining, pq)
			}
		}
		qd.pendingQueries = remaining
		qd.queryCond.Broadcast()
		return corr
	}()

	// Mark command as complete if it has a correlation ID
	if markComplete && correlationID != "" {
		qd.ApplyCommandResult(correlationID, "complete", result, "")
	}
}

// TakeQueryResult retrieves and deletes a query result.
// Returns (result, found).
func (qd *QueryDispatcher) TakeQueryResult(id string) (json.RawMessage, bool) {
	return qd.TakeQueryResultForClient(id, "")
}

// TakeQueryResultForClient retrieves and consumes a result scoped to one client.
//
// Invariants:
// - Successful reads are destructive (single-consumer semantics).
//
// Failure semantics:
// - Client mismatch returns (nil,false) without consuming the stored result.
func (qd *QueryDispatcher) TakeQueryResultForClient(id string, clientID string) (json.RawMessage, bool) {
	qd.mu.Lock()
	defer qd.mu.Unlock()

	entry, found := qd.queryResults[id]
	if !found {
		return nil, false
	}

	// Check client isolation
	if clientID != "" && entry.ClientID != clientID {
		return nil, false
	}

	// Delete after retrieval (one-time use)
	delete(qd.queryResults, id)
	return entry.Result, true
}

// ============================================
// Blocking wait for a result (synchronous tool handlers)
// ============================================

// WaitForResult blocks until result is available or timeout.
// Used by synchronous tool handlers that need immediate results.
func (qd *QueryDispatcher) WaitForResult(id string, timeout time.Duration) (json.RawMessage, error) {
	return qd.waitForResultContext(context.Background(), id, timeout, "")
}

// WaitForResultContext waits for one result and wakes promptly when the caller
// is canceled. The result remains one-time consumption, matching WaitForResult.
func (qd *QueryDispatcher) WaitForResultContext(ctx context.Context, id string, timeout time.Duration) (json.RawMessage, error) {
	return qd.waitForResultContext(ctx, id, timeout, "")
}

// WaitForResultWithClient waits for one result under client-isolated view.
// Uses event-driven result, cancellation, and deadline notifications.
//
// Flow:
// 1. Check if result already exists
// 2. If not, wait on condition variable
// 3. Recheck after a result, cancellation, or deadline notification
// 4. Return result or timeout error
//
// Invariants:
// - qd.queryCond.Wait is called only with qd.mu held.
//
// Failure semantics:
// - Timeout returns deterministic error; caller decides retry/abort policy.
// - Missing result after wakeups is expected (spurious or unrelated broadcasts).
func (qd *QueryDispatcher) WaitForResultWithClient(id string, timeout time.Duration, clientID string) (json.RawMessage, error) {
	return qd.waitForResultContext(context.Background(), id, timeout, clientID)
}

func (qd *QueryDispatcher) waitForResultContext(ctx context.Context, id string, timeout time.Duration, clientID string) (json.RawMessage, error) {
	deadline := time.Now().Add(timeout)
	wake := func() {
		qd.mu.Lock()
		qd.queryCond.Broadcast()
		qd.mu.Unlock()
	}
	timeoutTimer := time.AfterFunc(timeout, wake)
	cancelWake := context.AfterFunc(ctx, wake)
	defer timeoutTimer.Stop()
	defer cancelWake()

	qd.mu.Lock()
	defer qd.mu.Unlock()

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// Check if result exists
		if entry, found := qd.queryResults[id]; found {
			// Check client isolation
			if clientID == "" || entry.ClientID == clientID {
				delete(qd.queryResults, id)
				return entry.Result, nil
			}
		}

		// Check timeout
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for result %s", id)
		}

		qd.queryCond.Wait()
	}
}

// ============================================
// Periodic TTL cleanup and orphaned-command reconciliation
// ============================================

// startResultCleanup starts periodic TTL/lifecycle reconciliation.
//
// Invariants:
// - Returned stop func is single-use and owned by QueryDispatcher.Close.
//
// Failure semantics:
// - Cleanup failures are contained (no panic path); next ticker cycle retries.
func (qd *QueryDispatcher) startResultCleanup() func() {
	stop := make(chan struct{})
	util.SafeGo(func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				qd.cleanExpiredResults()
			case <-stop:
				return
			}
		}
	})
	return func() { close(stop) }
}

// cleanExpiredResults prunes stale query results and reconciles orphaned commands.
//
// Invariants:
// - Runs in three lock phases: mu-only collection, out-of-lock expiration, resultsMu cleanup.
//
// Failure semantics:
// - Orphaned correlations are expired rather than silently dropped to preserve LLM-visible outcomes.
func (qd *QueryDispatcher) cleanExpiredResults() {
	now := time.Now()

	// 1. Clean expired query results and collect orphaned pending correlations (under mu).
	orphanedCorrelationIDs := func() []string {
		qd.mu.Lock()
		defer qd.mu.Unlock()

		for id, entry := range qd.queryResults {
			if now.Sub(entry.CreatedAt) > QueryResultTTL {
				delete(qd.queryResults, id)
			}
		}

		var orphaned []string
		gracePeriod := AsyncCommandTimeout + 10*time.Second
		remaining := qd.pendingQueries[:0]
		for _, pq := range qd.pendingQueries {
			if now.Sub(pq.Expires) > gracePeriod {
				if pq.Query.CorrelationID != "" {
					orphaned = append(orphaned, pq.Query.CorrelationID)
				}
				// Drop from pending.
				continue
			}
			remaining = append(remaining, pq)
		}
		qd.pendingQueries = remaining
		return orphaned
	}()

	// 3. Expire orphaned pending commands (outside mu to respect lock ordering)
	for _, correlationID := range orphanedCorrelationIDs {
		qd.ExpireCommand(correlationID)
	}

	// 4. Clean stale active command entries (under resultsMu).
	func() {
		qd.resultsMu.Lock()
		defer qd.resultsMu.Unlock()
		for id, cmd := range qd.activeCommands {
			if now.Sub(cmd.CreatedAt) > QueryResultTTL {
				delete(qd.activeCommands, id)
			}
		}
	}()
}
