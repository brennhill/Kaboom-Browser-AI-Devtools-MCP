// result.go — Result sources and the validation policy each one must pass.
// Why: results reach the daemon over three paths with different trust and
// ordering guarantees; one policy per source keeps them from drifting.
// Docs: docs/features/feature/query-service/index.md

package browsercommand

import "encoding/json"

// ResultSource names the path a command result arrived on.
//
// Validation policy:
//   - SourceSync: carried by a /sync heartbeat. Accepted only inside
//     ApplySyncExchange while the connection owner confirms the exchange's
//     generation is current; a per-result generation must be 0 (inherits the
//     exchange) or equal the exchange generation.
//   - SourceQueryResult: the out-of-band /query-result channel for large
//     payloads. Its wire contract carries no generation, so it is validated by
//     identity only: it needs a query ID or correlation ID, query results are
//     scoped to ClientID, and command transitions stay monotonic (a terminal
//     command ignores later results).
//   - SourceDaemon: a daemon-originated terminal failure (disconnect, lost
//     command). It requires a correlation ID and an error.
type ResultSource string

// Result sources; see ResultSource for each one's validation policy.
const (
	SourceSync        ResultSource = "sync"
	SourceQueryResult ResultSource = "query_result"
	SourceDaemon      ResultSource = "daemon"
)

// Result is one lifecycle update for a command.
type Result struct {
	Source        ResultSource
	QueryID       string
	CorrelationID string
	Status        string
	Payload       json.RawMessage
	Error         string
	ClientID      string
	// Generation is the connection generation the extension stamped on a
	// SourceSync result; 0 means it inherits the enclosing exchange.
	Generation uint64
}

// Outcome reports what SubmitResult did with a result.
type Outcome string

// Result outcomes.
const (
	OutcomeApplied  Outcome = "applied"
	OutcomeRejected Outcome = "rejected"
)

// SubmitResult applies a result that does not travel inside a sync exchange.
// SourceSync results are rejected here: they must go through ApplySyncExchange
// so the connection owner can gate them on the current generation.
func (o *Owner) SubmitResult(result Result) Outcome {
	switch result.Source {
	case SourceQueryResult:
		if result.QueryID == "" && result.CorrelationID == "" {
			return OutcomeRejected
		}
	case SourceDaemon:
		if result.CorrelationID == "" || result.Error == "" {
			return OutcomeRejected
		}
	default:
		return OutcomeRejected
	}
	o.apply(result)
	return OutcomeApplied
}

// apply performs the store transitions shared by every accepted source.
// When a correlation ID is present the explicit status drives the command
// transition, so the query-result store must not also mark it complete.
func (o *Owner) apply(result Result) {
	if result.QueryID != "" {
		if result.CorrelationID != "" {
			o.queue.SetQueryResultWithClientNoCommandComplete(result.QueryID, result.Payload, result.ClientID)
		} else {
			o.queue.SetQueryResultWithClient(result.QueryID, result.Payload, result.ClientID)
		}
	}
	if result.CorrelationID != "" {
		o.queue.ApplyCommandResult(result.CorrelationID, result.Status, result.Payload, result.Error)
	}
}
