// command.go — Sends bounded QA fixture commands through the extension query queue.

package transport

import (
	"context"
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// Execute sends one fixture command and waits for its correlated extension result.
func Execute(ctx context.Context, captureStore *capture.Capture, command string, params json.RawMessage, timeout time.Duration) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if captureStore == nil || !captureStore.Extension().IsExtensionConnected() {
		return nil, context.Canceled
	}
	queryID, err := captureStore.Queries().CreatePendingQueryWithTimeout(queries.PendingQuery{Type: command, Params: params}, timeout, "")
	if err != nil {
		return nil, err
	}
	return captureStore.Queries().WaitForResultContext(ctx, queryID, timeout)
}
