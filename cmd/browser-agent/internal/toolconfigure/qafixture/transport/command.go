// command.go — Sends bounded QA fixture commands through the browser-command owner.

package transport

import (
	"context"
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// Execute sends one fixture command and waits for its correlated extension result.
// A nil commands owner or a disconnected extension reports context.Canceled.
func Execute(ctx context.Context, commands queries.QueryRunner, connected func() bool, command string, params json.RawMessage, timeout time.Duration) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if commands == nil || connected == nil || !connected() {
		return nil, context.Canceled
	}
	queryID, err := commands.Enqueue(queries.PendingQuery{Type: command, Params: params}, timeout, "")
	if err != nil {
		return nil, err
	}
	return commands.WaitQueryResult(ctx, queryID, timeout)
}
