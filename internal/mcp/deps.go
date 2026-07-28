// Purpose: Declares provider interfaces (DiagnosticProvider, CaptureProvider, etc.) that tools require from the host server.
// Docs: docs/features/feature/query-service/index.md

package mcp

import (
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
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

// CaptureProvider gives access to the capture instance for buffer reads.
// Used by all 5 tools.
type CaptureProvider interface {
	GetCapture() *capture.Capture
}

// LogBufferReader provides read-only access to server log entries.
// Used by observe, generate (testgen), configure, and analyze (security audit).
type LogBufferReader interface {
	GetLogEntries() ([]LogEntry, []time.Time)
	GetLogTotalAdded() int64
}

// A11yQueryExecutor runs accessibility queries via the browser extension.
// Used by observe (accessibility), generate (SARIF), and analyze (accessibility).
type A11yQueryExecutor interface {
	ExecuteA11yQuery(scope string, tags []string, frame any, forceRefresh bool) (json.RawMessage, error)
}

// NoiseFilterer checks whether log/network entries match noise suppression rules.
// Used by observe to filter out repetitive/irrelevant entries.
type NoiseFilterer interface {
	IsConsoleNoise(entry LogEntry) bool
}
