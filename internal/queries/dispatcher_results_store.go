// Purpose: Stores query results from extension callbacks and broadcasts to waiting callers.
// Why: Separates result write-path from cleanup and wait-for-result blocking logic.
package queries

import (
	"encoding/json"
	"time"
)

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
			Result:    result,
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
		return corr
	}()

	// Wake up waiters
	qd.queryCond.Broadcast()

	// Mark command as complete if it has a correlation ID
	if markComplete && correlationID != "" {
		qd.CompleteCommand(correlationID, result, "")
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
