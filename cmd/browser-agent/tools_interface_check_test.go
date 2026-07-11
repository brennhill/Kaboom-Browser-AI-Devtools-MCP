// Purpose: Tests for tool handler interface compliance.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_interface_check_test.go — Compile-time interface satisfaction assertions.
// If *ToolHandler doesn't satisfy a dep interface, compilation fails immediately.
package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/analyzehandler"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/configurehandler"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/observehandler"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"
)

// Phase 1: Shared dependency interfaces
var _ mcp.DiagnosticProvider = (*ToolHandler)(nil)
var _ mcp.AsyncCommandDispatcher = (*ToolHandler)(nil)
var _ mcp.PendingQueryEnqueuer = (*ToolHandler)(nil)

// Phase 2: Tool-specific dependency interfaces
var _ observe.Deps = (*ToolHandler)(nil)
var _ observehandler.Deps = (*ToolHandler)(nil)
var _ configurehandler.Deps = (*ToolHandler)(nil)
var _ analyzehandler.Deps = (*ToolHandler)(nil)
