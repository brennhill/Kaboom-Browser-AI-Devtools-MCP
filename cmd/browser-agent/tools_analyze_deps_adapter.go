// tools_analyze_deps_adapter.go — Adapts ToolHandler to satisfy analyzehandler.Deps interface.
// Why: Provides narrow accessor methods that bridge ToolHandler fields to the analyze sub-package.

package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/analyzehandler"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// NetworkBodies satisfies analyzehandler.Deps (also used by configurehandler.Deps).
// Already defined in tools_configure_deps_adapter.go.

// NetworkWaterfallEntries satisfies analyzehandler.Deps.
func (h *ToolHandler) NetworkWaterfallEntries() []capture.NetworkWaterfallEntry {
	return h.capture.GetNetworkWaterfallEntries()
}

// ConsoleSecurityEntries satisfies analyzehandler.Deps.
func (h *ToolHandler) ConsoleSecurityEntries() []security.LogEntry {
	h.server.logs.mu.RLock()
	entries := make([]security.LogEntry, len(h.server.logs.entries))
	for i, e := range h.server.logs.entries {
		entries[i] = security.LogEntry(e)
	}
	h.server.logs.mu.RUnlock()
	return entries
}

// SecurityScanner satisfies analyzehandler.Deps.
func (h *ToolHandler) SecurityScanner() analyzehandler.SecurityScannerInterface {
	if h.securityScannerImpl == nil {
		return nil
	}
	return h.securityScannerImpl
}

// LogEntries satisfies analyzehandler.Deps (returns entries without timestamps).
func (h *ToolHandler) LogEntries() []types.LogEntry {
	entries, _ := h.GetLogEntries()
	return entries
}

// ExecuteA11yQuery satisfies analyzehandler.Deps.
// Delegates to the existing method (already on ToolHandler via MCPHandler).
// Already defined via MCPHandler embedding.
