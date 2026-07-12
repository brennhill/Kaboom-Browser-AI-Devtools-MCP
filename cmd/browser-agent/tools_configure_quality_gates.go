// tools_configure_quality_gates.go — Entry point for configure(what="setup_quality_gates").
// Thin delegator: the scaffolding logic lives in configurehandler.HandleSetupQualityGates.
// Docs: docs/features/feature/quality-gates/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/configurehandler"
)

// toolConfigureSetupQualityGates handles configure(what="setup_quality_gates").
func (h *ToolHandler) toolConfigureSetupQualityGates(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return configurehandler.HandleSetupQualityGates(h, req, args)
}
