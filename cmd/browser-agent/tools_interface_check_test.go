// Purpose: Tests for tool handler interface compliance.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_interface_check_test.go — Compile-time interface satisfaction assertions.
// If *ToolHandler doesn't satisfy a dep interface, compilation fails immediately.
package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/netrecord"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// Phase 3: Narrow sub-package dependency interfaces
var _ netrecord.NetworkBodyProvider = (*capture.TelemetryStore)(nil)
