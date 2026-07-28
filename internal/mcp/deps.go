// Purpose: Declares the asynchronous command and diagnostic contracts used at MCP boundaries.
// Docs: docs/features/feature/query-service/index.md

package mcp

import (
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// DiagnosticProvider supplies system state snapshots for error messages.
// Used by all tools to attach "Current state: extension=connected, pilot=enabled, ..."
// hints to structured errors.
type DiagnosticProvider interface {
	DiagnosticHintString() string
}

// AsyncCommandDispatcher manages synchronous-by-default command execution.
// Used by analyze, generate, and interact tools that dispatch commands to
// the browser extension and wait for results.
type AsyncCommandDispatcher interface {
	MaybeWaitForCommand(req JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) JSONRPCResponse
}

// PendingQueryEnqueuer submits commands for browser extension pickup.
// Used by observe, analyze, interact, and generate tools that dispatch
// async queries to the extension.
type PendingQueryEnqueuer interface {
	EnqueuePendingQuery(req JSONRPCRequest, query queries.PendingQuery, timeout time.Duration) (JSONRPCResponse, bool)
}
