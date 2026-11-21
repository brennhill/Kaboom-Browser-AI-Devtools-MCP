// commandtest.go — Test support that simulates the extension through browsercommand.Owner's public API.
// Why: the dispatcher is internal to browsercommand, so tests can no longer write command state
// directly. These helpers drive the same result-source policies production uses (query_result
// uploads, gated sync acknowledgments) so tests exercise real lifecycle transitions.
// Docs: docs/features/feature/query-service/index.md

// Package commandtest provides test helpers for code that depends on browsercommand.Owner.
// Production code must not import it.
package commandtest

import (
	"context"
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/browsercommand"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// Enqueue queues a query with the default query timeout and no client scope.
func Enqueue(o *browsercommand.Owner, query queries.PendingQuery) (string, error) {
	return o.Enqueue(query, queries.DefaultQueryTimeout, "")
}

// EnqueueForClient queues a query with the default timeout for one client.
func EnqueueForClient(o *browsercommand.Owner, query queries.PendingQuery, clientID string) (string, error) {
	return o.Enqueue(query, queries.DefaultQueryTimeout, clientID)
}

// Register creates a pending async command record without extension delivery.
func Register(o *browsercommand.Owner, correlationID string, ttl time.Duration) {
	o.RegisterLocal(correlationID, ttl)
}

// Complete reports an extension result for a correlated command, as the
// /query-result channel would.
func Complete(o *browsercommand.Owner, correlationID, status string, payload json.RawMessage, errMessage string) browsercommand.Outcome {
	return o.SubmitResult(browsercommand.Result{
		Source: browsercommand.SourceQueryResult, CorrelationID: correlationID,
		Status: status, Payload: payload, Error: errMessage,
	})
}

// Expire marks a correlated command expired, as an extension timeout report would.
func Expire(o *browsercommand.Owner, correlationID string) browsercommand.Outcome {
	return Complete(o, correlationID, "expired", nil, "Command expired before extension could execute it")
}

// Deliver reports the one-time result of a query with no client scope.
func Deliver(o *browsercommand.Owner, queryID string, payload json.RawMessage) browsercommand.Outcome {
	return DeliverForClient(o, queryID, payload, "")
}

// DeliverForClient reports the one-time result of a query scoped to clientID.
func DeliverForClient(o *browsercommand.Owner, queryID string, payload json.RawMessage, clientID string) browsercommand.Outcome {
	return o.SubmitResult(browsercommand.Result{
		Source: browsercommand.SourceQueryResult, QueryID: queryID, Payload: payload, ClientID: clientID,
	})
}

// Take consumes a stored query result without waiting; false when absent.
func Take(o *browsercommand.Owner, queryID string) (json.RawMessage, bool) {
	result, err := o.WaitQueryResult(context.Background(), queryID, 0)
	return result, err == nil
}

// Acknowledge acknowledges delivery through queryID inside an always-current
// sync exchange, as a /sync heartbeat's last_command_ack would.
func Acknowledge(o *browsercommand.Owner, queryID string) {
	o.ApplySyncExchange(func(apply func()) (uint64, bool) {
		apply()
		return 0, true
	}, browsercommand.SyncExchange{LastAck: queryID, Now: time.Now()})
}
