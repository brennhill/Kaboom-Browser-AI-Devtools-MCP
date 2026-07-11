// Purpose: Thin adapter for generate(har) — delegates to generatehandler sub-package.

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/generatehandler"
)

func (h *ToolHandler) toolExportHAR(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return generatehandler.HandleExportHAR(h.generateDeps(), req, args)
}
