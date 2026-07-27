// tools_generate_artifacts.go — Main-package adapters for generated artifacts.
// Why: HAR, SARIF, CSP/SRI, PR summaries, and tests share one generate boundary.
// Docs: docs/features/feature/har-export/index.md
// Docs: docs/features/feature/sarif-export/index.md
// Docs: docs/features/feature/security-hardening/index.md
// Docs: docs/features/feature/test-generation/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate"
)

func (h *ToolHandler) toolExportHAR(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return toolgenerate.HandleExportHAR(h.generateDeps(), req, args)
}

func (h *ToolHandler) toolGeneratePRSummary(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return toolgenerate.HandlePRSummary(h.generateDeps(), req, args)
}

func (h *ToolHandler) toolExportSARIF(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return toolgenerate.HandleExportSARIF(h.generateDeps(), req, args)
}

func (h *ToolHandler) toolGenerateCSP(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return toolgenerate.HandleGenerateCSP(h.generateDeps(), req, args)
}

func (h *ToolHandler) toolGenerateSRI(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return toolgenerate.HandleGenerateSRI(h.generateDeps(), req, args)
}

func (h *ToolHandler) toolGenerateTest(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return toolgenerate.HandleGenerateTest(h.generateDeps(), req, args)
}
