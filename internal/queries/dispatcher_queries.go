// Purpose: Creates pending queries and notifies extension pollers of newly queued work.
// Docs: docs/features/feature/query-service/index.md

package queries

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

// ErrQueueFull is returned when a new command is rejected because the queue is at capacity.
// Callers should return an immediate error to the LLM so it knows the command was not accepted.
var ErrQueueFull = errors.New("queue_full")

var queryDispatcherSequence atomic.Uint64

// Constants for query management.
const (
	QueryResultTTL    = 5 * time.Minute // How long to keep query results before cleanup
	MaxPendingQueries = 15              // Max pending queries in queue
)

func newQueryIDPrefix() string {
	return fmt.Sprintf(
		"q-%x-%x-%x",
		time.Now().UnixNano(),
		os.Getpid(),
		queryDispatcherSequence.Add(1),
	)
}

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
//   - Signaling is edge-triggered by closing queryNotify then replacing it under
//     mu (same pattern as commandNotify) so every blocked waiter wakes, not just one.
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
		id := fmt.Sprintf("%s-%d", qd.queryIDPrefix, qd.queryIDCounter)

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

// ============================================
// Queue polling: snapshot, deliver, acknowledge
// ============================================

// cleanExpiredQueries removes past-deadline queue entries in-place.
func (qd *QueryDispatcher) cleanExpiredQueries() {
	now := time.Now()
	remaining := qd.pendingQueries[:0]

	for _, pending := range qd.pendingQueries {
		if pending.Expires.After(now) {
			remaining = append(remaining, pending)
		}
	}
	qd.pendingQueries = remaining
}

// ============================================
// Query Retrieval (Extension Polling)
// ============================================

type pendingQuerySnapshot struct {
	result             []PendingQueryResponse
	sentCorrelationIDs []string
}

func (qd *QueryDispatcher) snapshotPendingQueries(clientID string) pendingQuerySnapshot {
	qd.mu.Lock()
	defer qd.mu.Unlock()

	qd.cleanExpiredQueries()

	result := make([]PendingQueryResponse, 0, len(qd.pendingQueries))
	sentCorrelationIDs := make([]string, 0, len(qd.pendingQueries))
	for _, pending := range qd.pendingQueries {
		if clientID != "" && pending.ClientID != clientID {
			continue
		}
		result = append(result, pending.Query)
		if pending.Query.CorrelationID != "" {
			sentCorrelationIDs = append(sentCorrelationIDs, pending.Query.CorrelationID)
		}
	}
	return pendingQuerySnapshot{
		result:             result,
		sentCorrelationIDs: sentCorrelationIDs,
	}
}

// GetPendingQueries snapshots all currently deliverable queued commands.
func (qd *QueryDispatcher) GetPendingQueries() []PendingQueryResponse {
	snapshot := qd.snapshotPendingQueries("")

	for _, correlationID := range snapshot.sentCorrelationIDs {
		qd.recordTraceEvent(correlationID, traceStageSent, "sync", "pending", "", time.Now())
	}
	return snapshot.result
}

// GetPendingQueriesForClient snapshots queued commands scoped to one client.
func (qd *QueryDispatcher) GetPendingQueriesForClient(clientID string) []PendingQueryResponse {
	snapshot := qd.snapshotPendingQueries(clientID)

	for _, correlationID := range snapshot.sentCorrelationIDs {
		qd.recordTraceEvent(correlationID, traceStageSent, "sync", "pending", "", time.Now())
	}
	return snapshot.result
}

// AcknowledgePendingQuery advances queue head through queryID (inclusive).
func (qd *QueryDispatcher) AcknowledgePendingQuery(queryID string) {
	if queryID == "" {
		return
	}

	type acknowledgePlan struct {
		acknowledged          bool
		startedCorrelationIDs []string
	}
	plan := func() acknowledgePlan {
		qd.mu.Lock()
		defer qd.mu.Unlock()

		ackIndex := -1
		for i, pending := range qd.pendingQueries {
			if pending.Query.ID == queryID {
				ackIndex = i
				break
			}
		}
		if ackIndex < 0 {
			return acknowledgePlan{}
		}

		startedCorrelationIDs := make([]string, 0, ackIndex+1)
		for _, pending := range qd.pendingQueries[:ackIndex+1] {
			if pending.Query.CorrelationID != "" {
				startedCorrelationIDs = append(startedCorrelationIDs, pending.Query.CorrelationID)
			}
		}

		remaining := make([]PendingQueryEntry, 0, len(qd.pendingQueries)-ackIndex-1)
		remaining = append(remaining, qd.pendingQueries[ackIndex+1:]...)
		qd.pendingQueries = remaining
		return acknowledgePlan{
			acknowledged:          true,
			startedCorrelationIDs: startedCorrelationIDs,
		}
	}()
	if !plan.acknowledged {
		return
	}
	for _, correlationID := range plan.startedCorrelationIDs {
		qd.recordTraceEvent(correlationID, traceStageStarted, "sync", "pending", "", time.Now())
	}
}

// ============================================
// Bulk expiry of the whole queue
// ============================================

// ExpireAllPendingQueries fails every queued command with a shared reason.
func (qd *QueryDispatcher) ExpireAllPendingQueries(reason string) {
	correlationIDs := func() []string {
		qd.mu.Lock()
		defer qd.mu.Unlock()

		var ids []string
		for _, pending := range qd.pendingQueries {
			if pending.Query.CorrelationID != "" {
				ids = append(ids, pending.Query.CorrelationID)
			}
		}
		qd.pendingQueries = qd.pendingQueries[:0]
		return ids
	}()

	if len(correlationIDs) == 0 {
		return
	}

	ch := func() chan struct{} {
		qd.resultsMu.Lock()
		defer qd.resultsMu.Unlock()
		for _, correlationID := range correlationIDs {
			cmd, exists := qd.activeCommands[correlationID]
			if !exists {
				continue
			}
			if cmd.Status != "pending" {
				continue
			}
			cmd.Status = "expired"
			cmd.Error = reason
			now := time.Now()
			qd.appendTraceEventLocked(cmd, traceStageTimedOut, "timeout", "expired", reason, now)
			cmd.CompletedAt = now
			delete(qd.activeCommands, correlationID)
			qd.appendTerminalCommandLocked(cmd)
		}

		ch := qd.commandNotify
		qd.commandNotify = make(chan struct{})
		return ch
	}()
	close(ch)
}
