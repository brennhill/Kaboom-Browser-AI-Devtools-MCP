// Purpose: Declares the query dispatch package and its transport/lifecycle contracts.
// Docs: docs/features/feature/query-service/index.md

// Package queries owns extension-bound request queues and command results.
package queries

import (
	"bytes"
	"encoding/json"
	"time"
)

func cloneJSON(value json.RawMessage) json.RawMessage {
	return bytes.Clone(value)
}

// ============================================
// Query Types
// ============================================

// PendingQuery represents one command request queued for extension execution.
//
// Invariants:
// - CorrelationID is optional but required for async lifecycle tracking.
// - Params must remain JSON-serializable through extension transport.
type PendingQuery struct {
	Type          string          `json:"type"`
	Params        json.RawMessage `json:"params"`
	TabID         int             `json:"tab_id,omitempty"`         // Target tab ID (0 = active tab)
	CorrelationID string          `json:"correlation_id,omitempty"` // LLM-facing tracking ID for async commands
	TraceID       string          `json:"trace_id,omitempty"`       // End-to-end trace ID for async command lifecycle
}

// PendingQueryResponse is the transport envelope delivered to the extension.
//
// Invariants:
// - ID is daemon-generated and unique for this process lifetime.
// - TraceID should remain stable across queue, extension, and observe surfaces.
type PendingQueryResponse struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Params        json.RawMessage `json:"params"`
	TabID         int             `json:"tab_id,omitempty"`         // Target tab ID (0 = active tab)
	CorrelationID string          `json:"correlation_id,omitempty"` // LLM-facing tracking ID for async commands
	TraceID       string          `json:"trace_id,omitempty"`       // End-to-end trace ID for async command lifecycle
}

// CommandTraceEvent records one lifecycle transition for async command diagnostics.
//
// Invariants:
// - Stage values are canonical (queued, sent, started, resolved, timed_out, errored).
// - At timestamps are append-ordered per command, producing a stable TraceTimeline.
type CommandTraceEvent struct {
	Stage   string    `json:"stage"` // queued, sent, started, resolved, timed_out, errored
	At      time.Time `json:"at"`
	Source  string    `json:"source,omitempty"` // queue, sync, extension, timeout
	Status  string    `json:"status,omitempty"`
	Message string    `json:"message,omitempty"`
}

// CommandResult is the authoritative lifecycle record for one correlation ID.
//
// Invariants:
// - Status transitions are monotonic: pending -> terminal (complete/error/timeout/expired/cancelled).
// - CreatedAt is immutable; UpdatedAt tracks latest transition mutation.
// - ExpiresAt is a hard deadline used to reconcile lost extension callbacks.
//
// Failure semantics:
// - Error may be set even when a result payload exists; consumers should trust Status first.
// - Terminal failures may move from active storage into failed-history ring.
type CommandResult struct {
	CorrelationID string              `json:"correlation_id"`
	TraceID       string              `json:"trace_id,omitempty"`
	QueryID       string              `json:"query_id,omitempty"`
	Status        string              `json:"status"` // "pending", "complete", "error", "timeout", "expired", "cancelled"
	Result        json.RawMessage     `json:"result,omitempty"`
	Error         string              `json:"error,omitempty"`
	CompletedAt   time.Time           `json:"completed_at,omitempty"`
	CreatedAt     time.Time           `json:"created_at"`
	ExpiresAt     time.Time           `json:"expires_at,omitempty"` // hard deadline to avoid stuck-pending commands
	UpdatedAt     time.Time           `json:"updated_at,omitempty"`
	TraceEvents   []CommandTraceEvent `json:"trace_events,omitempty"`
	TraceTimeline string              `json:"trace_timeline,omitempty"`
}

// ElapsedMs returns lifecycle elapsed time.
//
// Failure semantics:
// - For pending commands, uses time.Now() to provide a moving duration snapshot.
func (cr *CommandResult) ElapsedMs() int64 {
	end := cr.CompletedAt
	if end.IsZero() {
		end = time.Now()
	}
	return end.Sub(cr.CreatedAt).Milliseconds()
}

// ============================================
// Constants
// ============================================

const (
	// DefaultQueryTimeout is the default timeout for extension queries.
	// Extension polls every 1-2s, fast timeout prevents MCP hangs.
	DefaultQueryTimeout = 2 * time.Second

	// AsyncCommandTimeout is the timeout for async commands (execute_js, browser actions).
	// Commands are queued and polled by the extension, then completed asynchronously.
	// This timeout must be >= extension execution timeout to avoid premature expiration.
	AsyncCommandTimeout = 60 * time.Second
)
