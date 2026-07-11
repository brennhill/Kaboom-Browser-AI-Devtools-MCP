// Purpose: Thin adapter for generate(sarif) — delegates to generatehandler sub-package.

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/generatehandler"
)

func (h *ToolHandler) toolExportSARIF(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return generatehandler.HandleExportSARIF(h.generateDeps(), req, args)
}
