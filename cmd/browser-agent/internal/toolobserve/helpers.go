// helpers.go — Package-local aliases for the shared tool response vocabulary.
// Why: Keeps observe handler call sites concise. The implementations live in
// internal/mcp (response builders) and internal/toolresp (correlation IDs);
// this package no longer carries its own copies.

package toolobserve

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

var (
	succeed          = mcp.Succeed
	fail             = mcp.Fail
	newCorrelationID = toolresp.NewCorrelationID
)
