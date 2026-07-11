// Purpose: Test alias for DOM params helper that moved to internal/interacthandler.

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/interacthandler"
)

func parseDOMPrimitiveParams(args json.RawMessage) (interacthandler.DOMPrimitiveParams, error) {
	return interacthandler.ParseDOMPrimitiveParams(args)
}

func validateDOMActionParams(req JSONRPCRequest, action, text, value, name string) (JSONRPCResponse, bool) {
	return interacthandler.ValidateDOMActionParams(req, action, text, value, name)
}
