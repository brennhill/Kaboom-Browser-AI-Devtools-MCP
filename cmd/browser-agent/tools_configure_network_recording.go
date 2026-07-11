// Purpose: Delegates network recording handler to the configurehandler sub-package.
// Why: Keeps the handler wiring in main thin while logic lives in the sub-package.
// Docs: docs/features/feature/backend-log-streaming/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/configurehandler"
)

// toolConfigureNetworkRecording delegates to the sub-package handler.
func (h *ToolHandler) toolConfigureNetworkRecording(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return configurehandler.HandleNetworkRecording(h.capture, h.networkRecording, req, args)
}
