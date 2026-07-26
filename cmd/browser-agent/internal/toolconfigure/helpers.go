// helpers.go — Package-local aliases for the shared tool response vocabulary.
// Why: Keeps configure handler call sites concise. The implementations live in
// internal/mcp; this package no longer carries its own copies.

package toolconfigure

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

var (
	succeed          = mcp.Succeed
	fail             = mcp.Fail
	lenientUnmarshal = mcp.LenientUnmarshal
)
