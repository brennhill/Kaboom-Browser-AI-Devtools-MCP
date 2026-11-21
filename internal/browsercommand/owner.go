// owner.go — Transport-independent owner of the browser-command lifecycle.
// Why: /sync, /query-result and MCP waits previously each sequenced queue, ack,
// result, expiry and disconnect transitions themselves; this owner is the one
// place those transitions happen, built around the internal dispatcher state.
// Docs: docs/features/feature/query-service/index.md

// Package browsercommand owns the lifecycle of commands the daemon sends to the
// browser extension: queueing, delivery acknowledgment, results, expiry,
// disconnect handling and missing-progress reconciliation.
//
// It is transport independent: it must not import net/http, internal/mcp,
// internal/capture/** or cmd/** (enforced by scripts/contracts/layering).
// Connection identity, liveness and generation stay with the daemon connection
// owner (internal/capture/syncruntime.Runtime); generation-sensitive work
// enters through ApplySyncExchange with a GenerationGate supplied by that owner.
package browsercommand

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/browsercommand/internal/dispatch"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// Owner is the single owner of browser-command lifecycle transitions.
//
// Invariants:
//   - missingSince only tracks commands that are still pending in the queue.
//   - Every result-bearing mutation goes through SubmitResult or
//     ApplySyncExchange, so each result source is validated by exactly one
//     policy (see ResultSource).
type Owner struct {
	queue *dispatch.QueryDispatcher

	mu           sync.Mutex
	missingSince map[string]time.Time // first heartbeat that omitted a started command
}

// NewOwner creates the lifecycle owner together with the dispatcher that
// stores queue and command state. The dispatcher lives in an internal package,
// so no other package can mutate command state around the owner.
func NewOwner() *Owner {
	return &Owner{queue: dispatch.NewQueryDispatcher(), missingSince: make(map[string]time.Time)}
}

// Close stops the dispatcher's background cleanup. It is idempotent.
func (o *Owner) Close() { o.queue.Close() }

// Enqueue queues one command for extension delivery and returns its query ID.
// A non-empty CorrelationID registers an async command record.
func (o *Owner) Enqueue(query queries.PendingQuery, timeout time.Duration, clientID string) (string, error) {
	return o.queue.CreatePendingQueryWithTimeout(query, timeout, clientID)
}

// Pending returns a snapshot of commands awaiting delivery acknowledgment.
func (o *Owner) Pending() []queries.PendingQueryResponse {
	return o.queue.GetPendingQueries()
}

// AwaitPending implements the long-poll delivery policy: it returns queued
// commands immediately, or blocks up to timeout for the first one to arrive.
func (o *Owner) AwaitPending(timeout time.Duration) []queries.PendingQueryResponse {
	pending := o.queue.GetPendingQueries()
	if len(pending) > 0 {
		return pending
	}
	o.queue.WaitForPendingQueries(timeout)
	return o.queue.GetPendingQueries()
}

// ExpireAll expires every undelivered command and pending async command with
// reason (for example "extension_disconnected").
func (o *Owner) ExpireAll(reason string) {
	o.queue.ExpireAllPendingQueries(reason)
}

// FailDisconnected terminally fails one pending command because the extension
// disconnected while an MCP caller was waiting on it, and returns its record.
func (o *Owner) FailDisconnected(correlationID string) (*queries.CommandResult, bool) {
	o.SubmitResult(Result{Source: SourceDaemon, CorrelationID: correlationID, Status: "error", Error: "extension_disconnected"})
	return o.queue.GetCommandResult(correlationID)
}

// Command returns the current record for an async command.
func (o *Owner) Command(correlationID string) (*queries.CommandResult, bool) {
	return o.queue.GetCommandResult(correlationID)
}

// WaitCommand blocks until the async command leaves pending or timeout elapses.
func (o *Owner) WaitCommand(correlationID string, timeout time.Duration) (*queries.CommandResult, bool) {
	return o.queue.WaitForCommand(correlationID, timeout)
}

// WaitQueryResult blocks for the one-time result of a non-async query.
func (o *Owner) WaitQueryResult(ctx context.Context, queryID string, timeout time.Duration) (json.RawMessage, error) {
	return o.queue.WaitForResultContext(ctx, queryID, timeout)
}

// QueueDepth reports how many commands await delivery.
func (o *Owner) QueueDepth() int { return o.queue.QueueDepth() }

// QueuePosition reports a command's position in the delivery queue, or -1.
func (o *Owner) QueuePosition(correlationID string) int {
	return o.queue.QueuePosition(correlationID)
}
