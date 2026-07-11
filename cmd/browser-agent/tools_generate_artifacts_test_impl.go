// Purpose: Thin adapter for generate(test) — delegates to generatehandler sub-package.

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/generatehandler"
)

func (h *ToolHandler) toolGenerateTest(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return generatehandler.HandleGenerateTest(h.generateDeps(), req, args)
}
