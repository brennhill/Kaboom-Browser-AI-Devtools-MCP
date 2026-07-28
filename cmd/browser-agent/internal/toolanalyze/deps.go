// deps.go — Explicit dependencies for analyze-local handlers.
// Why: Keeps analyze handlers independent without forcing the composition root
// to expose forwarding methods for analyze-owned data.

package toolanalyze

import (
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// Deps groups the exact callbacks used by analyze-local handlers.
type Deps struct {
	EnqueuePendingQuery     func(mcp.JSONRPCRequest, queries.PendingQuery, time.Duration) (mcp.JSONRPCResponse, bool)
	MaybeWaitForCommand     func(mcp.JSONRPCRequest, string, json.RawMessage, string) mcp.JSONRPCResponse
	GetTrackingStatus       func() (bool, int, string)
	NetworkBodies           func() []types.NetworkBody
	NetworkWaterfallEntries func() []types.NetworkWaterfallEntry
	ConsoleSecurityEntries  func() []types.LogEntry
	SecurityScanner         func() SecurityScannerInterface
	LogEntries              func() []types.LogEntry
	ExecuteA11yQuery        func(string, []string, any, bool) (json.RawMessage, error)
}

// SecurityScannerInterface is the narrow interface for security scanning.
type SecurityScannerInterface interface {
	HandleSecurityAudit(args json.RawMessage, bodies []types.NetworkBody, console []types.LogEntry, pageURLs []string, waterfall []types.NetworkWaterfallEntry) (any, error)
}
