// Purpose: Creates pending queries and notifies extension pollers of newly queued work.
// Docs: docs/features/feature/query-service/index.md

package queries

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// ErrQueueFull is returned when a new command is rejected because the queue is at capacity.
// Callers should return an immediate error to the LLM so it knows the command was not accepted.
var ErrQueueFull = errors.New("queue_full")

// Constants for query management.
const (
	QueryResultTTL    = 5 * time.Minute // How long to keep query results before cleanup
	MaxPendingQueries = 15              // Max pending queries in queue
)

// ============================================
// Pending Query Creation
// ============================================

// CreatePendingQuery creates a pending query with default timeout and no client ID.
// Returns the query ID that extension will use to post the result, or ErrQueueFull.
func (qd *QueryDispatcher) CreatePendingQuery(query PendingQuery) (string, error) {
	return qd.CreatePendingQueryWithTimeout(query, qd.queryTimeout, "")
}

// CreatePendingQueryWithClient creates a pending query for a specific client.
// Used in multi-client mode to isolate queries between different MCP clients.
func (qd *QueryDispatcher) CreatePendingQueryWithClient(query PendingQuery, clientID string) (string, error) {
	return qd.CreatePendingQueryWithTimeout(query, qd.queryTimeout, clientID)
}

// CreatePendingQueryWithTimeout enqueues one command for extension pickup.
//
// Invariants:
// - Signaling is edge-triggered by closing queryNotify then replacing it under
//   mu (same pattern as commandNotify) so every blocked waiter wakes, not just one.
func (qd *QueryDispatcher) CreatePendingQueryWithTimeout(query PendingQuery, timeout time.Duration, clientID string) (string, error) {
	type pendingQueryPlan struct {
		id            string
		correlationID string
		queueFull     bool
		notify        chan struct{}
	}
	plan := func() pendingQueryPlan {
		qd.mu.Lock()
		defer qd.mu.Unlock()

		if len(qd.pendingQueries) >= MaxPendingQueries {
			return pendingQueryPlan{
				correlationID: query.CorrelationID,
				queueFull:     true,
			}
		}

		qd.queryIDCounter++
		id := fmt.Sprintf("q-%d", qd.queryIDCounter)

		entry := PendingQueryEntry{
			Query: PendingQueryResponse{
				ID:            id,
				Type:          query.Type,
				Params:        query.Params,
				TabID:         query.TabID,
				CorrelationID: query.CorrelationID,
				TraceID:       deriveTraceID(query.TraceID, query.CorrelationID, id),
			},
			Expires:  time.Now().Add(timeout),
			ClientID: clientID,
		}

		qd.pendingQueries = append(qd.pendingQueries, entry)
		notify := qd.queryNotify
		qd.queryNotify = make(chan struct{})
		return pendingQueryPlan{
			id:            id,
			correlationID: query.CorrelationID,
			notify:        notify,
		}
	}()
	if plan.queueFull {
		fmt.Fprintf(os.Stderr, "[Kaboom] Queue full (%d/%d): rejecting command type=%s correlation_id=%s\n",
			MaxPendingQueries, MaxPendingQueries, query.Type, plan.correlationID)

		if plan.correlationID != "" {
			qd.RegisterCommand(plan.correlationID, "", timeout)
			qd.ApplyCommandResult(plan.correlationID, "error", nil,
				fmt.Sprintf("Queue full: %d commands pending. Wait for in-flight commands to complete.", MaxPendingQueries))
		}
		return "", ErrQueueFull
	}

	close(plan.notify)

	if plan.correlationID != "" {
		qd.RegisterCommand(plan.correlationID, plan.id, timeout)
	}

	return plan.id, nil
}

// WaitForPendingQueries blocks until queue is non-empty or timeout elapses.
func (qd *QueryDispatcher) WaitForPendingQueries(timeout time.Duration) {
	notify := func() chan struct{} {
		qd.mu.Lock()
		defer qd.mu.Unlock()
		if len(qd.pendingQueries) > 0 {
			return nil
		}
		return qd.queryNotify
	}()
	if notify == nil {
		return
	}

	select {
	case <-notify:
	case <-time.After(timeout):
	}
}
