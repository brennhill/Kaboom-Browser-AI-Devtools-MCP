// Purpose: Thin adapter bridging generatehandler annotation handlers to ToolHandler.
// Why: Keeps ToolHandler methods as thin wrappers while implementation lives in the generatehandler sub-package.
// Docs: docs/features/feature/annotated-screenshots/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/generatehandler"
)

func (h *ToolHandler) toolGenerateVisualTest(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return generatehandler.HandleVisualTest(h.generateDeps(), req, args)
}

func (h *ToolHandler) toolGenerateAnnotationReport(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return generatehandler.HandleAnnotationReport(h.generateDeps(), req, args)
}

func (h *ToolHandler) toolGenerateAnnotationIssues(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return generatehandler.HandleAnnotationIssues(h.generateDeps(), req, args)
}
