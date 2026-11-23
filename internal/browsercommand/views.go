// views.go — Daemon-local commands and read-only views over browser-command state.
// Why: tool adapters, health and diagnostics need command history and queue
// state; serving them from the owner keeps *queries.QueryDispatcher private to
// the lifecycle owner so no caller can mutate command state around it.
// Docs: docs/features/feature/query-service/index.md

package browsercommand

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/browsercommand/query"
)

// RegisterLocal creates a pending command that the daemon itself completes
// (for example an annotation wait); it is never delivered to the extension.
// Completion goes through SubmitResult with SourceDaemon.
func (o *Owner) RegisterLocal(correlationID string, ttl time.Duration) {
	o.queue.RegisterCommand(correlationID, "", ttl)
}

// Snapshot reports queue and result-store counters.
func (o *Owner) Snapshot() queries.QuerySnapshot { return o.queue.GetSnapshot() }

// PendingCommands returns async commands that have not reached a terminal state.
func (o *Owner) PendingCommands() []*queries.CommandResult { return o.queue.GetPendingCommands() }

// CompletedCommands returns recently completed async commands.
func (o *Owner) CompletedCommands() []*queries.CommandResult { return o.queue.GetCompletedCommands() }

// FailedCommands returns recently failed, expired or timed-out async commands.
func (o *Owner) FailedCommands() []*queries.CommandResult { return o.queue.GetFailedCommands() }

// RecentTraces returns up to limit recent command lifecycle traces.
func (o *Owner) RecentTraces(limit int) []*queries.CommandResult {
	return o.queue.GetRecentCommandTraces(limit)
}
