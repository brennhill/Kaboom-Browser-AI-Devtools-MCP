// deps.go — Declares the Deps interface for analyze-local handlers.
// Why: Narrow interface decouples analyze handlers from the full ToolHandler.

package toolanalyze

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/scan"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// Deps provides all dependencies the analyze-local handlers need.
// *ToolHandler in cmd/browser-agent/ satisfies this interface.
type Deps interface {
	mcp.PendingQueryEnqueuer
	mcp.AsyncCommandDispatcher

	// GetTrackingStatus returns (enabled, tabID, tabURL) for the tracked tab.
	GetTrackingStatus() (bool, int, string)

	// NetworkBodies returns captured network bodies.
	NetworkBodies() []capture.NetworkBody

	// NetworkWaterfallEntries returns captured network waterfall entries.
	NetworkWaterfallEntries() []capture.NetworkWaterfallEntry

	// ConsoleSecurityEntries returns console entries as scan.LogEntry for security scanning.
	ConsoleSecurityEntries() []scan.LogEntry

	// SecurityScanner returns the security scanner, or nil if not initialized.
	SecurityScanner() SecurityScannerInterface

	// LogEntries returns a snapshot of console log entries.
	LogEntries() []types.LogEntry

	// ExecuteA11yQuery runs an accessibility audit via the extension.
	ExecuteA11yQuery(scope string, tags []string, frame any, forceRefresh bool) (json.RawMessage, error)
}

// SecurityScannerInterface is the narrow interface for security scanning.
type SecurityScannerInterface interface {
	HandleSecurityAudit(args json.RawMessage, bodies []capture.NetworkBody, console []scan.LogEntry, pageURLs []string, waterfall []capture.NetworkWaterfallEntry) (any, error)
}
