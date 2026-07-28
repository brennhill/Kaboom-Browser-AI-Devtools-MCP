// deps.go — Declares the Deps interface for observe-local handlers.
// Why: Narrow interface decouples observe handlers from the full ToolHandler.

package toolobserve

import (
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// Deps names the state and command operations owned outside observe-local handlers.
type Deps struct {
	Inbox               *push.PushInbox
	EnqueuePendingQuery func(mcp.JSONRPCRequest, queries.PendingQuery, time.Duration) (mcp.JSONRPCResponse, bool)
	MaybeWaitForCommand func(mcp.JSONRPCRequest, string, json.RawMessage, string) mcp.JSONRPCResponse
}
