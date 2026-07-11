// Purpose: Thin adapter for generate(pr_summary) — delegates to generatehandler sub-package.

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/generatehandler"
)

func (h *ToolHandler) toolGeneratePRSummary(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return generatehandler.HandlePRSummary(h.generateDeps(), req, args)
}
