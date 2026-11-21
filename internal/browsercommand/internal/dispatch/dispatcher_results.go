// Purpose: Stores, retrieves, waits on, and expires query result payloads for sync/async tool paths.
// Why: The result lifecycle is one unit of state guarded by QueryDispatcher.mu — store, wait and
// cleanup all mutate queryResults/pendingQueries under it, so they read and review together.
// Docs: docs/features/feature/query-service/index.md
//
// Sections: configuration accessors, result store, blocking wait, TTL cleanup.

package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

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
			Result:    bytes.Clone(result),
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

// ============================================
// Blocking wait for a result (synchronous tool handlers)
// ============================================

// WaitForResultContext waits for one result and wakes promptly when the caller
// is canceled or the timeout elapses. Results are one-time consumption.
func (qd *QueryDispatcher) WaitForResultContext(ctx context.Context, id string, timeout time.Duration) (json.RawMessage, error) {
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
			delete(qd.queryResults, id)
			return entry.Result, nil
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
//   - cleanupDone closes only after the worker has stopped, making Close a
//     deterministic completion barrier.
//
// Failure semantics:
// - Cleanup failures are contained (no panic path); next ticker cycle retries.
func (qd *QueryDispatcher) startResultCleanup() {
	util.SafeGo(func() {
		defer close(qd.cleanupDone)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				qd.cleanExpiredResults()
			case <-qd.cleanupStop:
				return
			}
		}
	})
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
			if now.Sub(entry.CreatedAt) > queries.QueryResultTTL {
				delete(qd.queryResults, id)
			}
		}

		var orphaned []string
		gracePeriod := queries.AsyncCommandTimeout + 10*time.Second
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
			if now.Sub(cmd.CreatedAt) > queries.QueryResultTTL {
				delete(qd.activeCommands, id)
			}
		}
	}()
}
