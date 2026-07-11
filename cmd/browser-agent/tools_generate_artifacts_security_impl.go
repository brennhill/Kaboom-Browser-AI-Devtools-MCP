// Purpose: Thin adapter for generate(csp) and generate(sri) — delegates to generatehandler sub-package.

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/generatehandler"
)

func (h *ToolHandler) toolGenerateCSP(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return generatehandler.HandleGenerateCSP(h.generateDeps(), req, args)
}

func (h *ToolHandler) toolGenerateSRI(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return generatehandler.HandleGenerateSRI(h.generateDeps(), req, args)
}
