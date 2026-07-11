// deps.go — Declares the Deps interface for observe-local handlers.
// Why: Narrow interface decouples observe handlers from the full ToolHandler.

package observehandler

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
)

// Deps provides all dependencies the observe-local handlers need.
// *ToolHandler in cmd/browser-agent/ satisfies this interface.
type Deps interface {
	mcp.PendingQueryEnqueuer
	mcp.AsyncCommandDispatcher

	// PushInbox returns the push inbox for draining events, or nil if unavailable.
	PushInbox() *push.PushInbox

	// IsExtensionConnected reports whether the browser extension is connected.
	IsExtensionConnected() bool
}
