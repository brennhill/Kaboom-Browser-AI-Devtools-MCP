// tools_analyze_deps_adapter.go — Adapts ToolHandler to satisfy toolanalyze.Deps interface.
// Why: Provides narrow accessor methods that bridge ToolHandler fields to the analyze sub-package.

package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/scan"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// NetworkBodies satisfies toolanalyze.Deps (also used by toolconfigure.Deps).
// Already defined in tools_configure_deps_adapter.go.

// NetworkWaterfallEntries satisfies toolanalyze.Deps.
func (h *ToolHandler) NetworkWaterfallEntries() []capture.NetworkWaterfallEntry {
	return h.capture.GetNetworkWaterfallEntries()
}

// ConsoleSecurityEntries satisfies toolanalyze.Deps.
func (h *ToolHandler) ConsoleSecurityEntries() []scan.LogEntry {
	snapshot := h.server.logs.Entries()
	entries := make([]scan.LogEntry, len(snapshot))
	for i, e := range snapshot {
		entries[i] = scan.LogEntry(e)
	}
	return entries
}

// SecurityScanner satisfies toolanalyze.Deps.
func (h *ToolHandler) SecurityScanner() toolanalyze.SecurityScannerInterface {
	if h.securityScannerImpl == nil {
		return nil
	}
	return h.securityScannerImpl
}

// LogEntries satisfies toolanalyze.Deps (returns entries without timestamps).
func (h *ToolHandler) LogEntries() []types.LogEntry {
	entries, _ := h.GetLogEntries()
	return entries
}

// ExecuteA11yQuery satisfies toolanalyze.Deps.
// Delegates to the existing method (already on ToolHandler via MCPHandler).
// Already defined via MCPHandler embedding.
