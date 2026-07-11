// tools_generate_adapter.go — Bridges the generatehandler package to the main ToolHandler.
// Purpose: Provides the generateDeps accessor that satisfies generatehandler.Deps.
// Why: Keeps the generatehandler package decoupled from the main package's god object.

package main

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/generatehandler"

// generateDeps returns the ToolHandler as a generatehandler.Deps.
// *ToolHandler satisfies the generatehandler.Deps interface directly.
func (h *ToolHandler) generateDeps() generatehandler.Deps {
	return h
}
