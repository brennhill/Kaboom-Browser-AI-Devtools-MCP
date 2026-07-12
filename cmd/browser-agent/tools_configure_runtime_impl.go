// tools_configure_runtime_impl.go — Entry points for configure runtime controls.
// Thin delegators: restart + test-boundary logic lives in configurehandler.
// Why: Isolates runtime/environment mutations from the configure router.

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/configurehandler"
)

// toolConfigureRestart handles configure(what="restart") that reaches the daemon.
func (h *ToolHandler) toolConfigureRestart(req JSONRPCRequest) JSONRPCResponse {
	return configurehandler.HandleRestart(req)
}

// toolConfigureTestBoundaryStart handles configure(what="test_boundary_start").
func (h *ToolHandler) toolConfigureTestBoundaryStart(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return configurehandler.HandleTestBoundaryStart(h, req, args)
}

// toolConfigureTestBoundaryEnd handles configure(what="test_boundary_end").
func (h *ToolHandler) toolConfigureTestBoundaryEnd(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return configurehandler.HandleTestBoundaryEnd(h, req, args)
}
