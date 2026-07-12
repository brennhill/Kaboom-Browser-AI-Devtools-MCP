// tools_configure_audit_log.go — Entry point for configure(what="audit_log").
// Thin delegator: report/analyze/clear logic lives in configurehandler.HandleAuditLog.
// Docs: docs/features/feature/noise-filtering/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/configurehandler"
)

func (h *ToolHandler) toolGetAuditLog(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return configurehandler.HandleAuditLog(h, req, args)
}
