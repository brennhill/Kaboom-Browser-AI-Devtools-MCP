// Purpose: Declares what the observe handlers need from the host server, and the bounds every mode applies.
// Why: Deps is the single seam between this package and *ToolHandler in cmd/browser-agent; the limit clamp
// sits with it because every mode that accepts a limit runs it through clampLimit before touching a buffer.
// Docs: docs/features/feature/observe/index.md

package observe

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"

// Deps provides all dependencies the observe handlers need.
// *ToolHandler in cmd/browser-agent/ satisfies this interface.
type Deps interface {
	mcp.DiagnosticProvider
	mcp.CaptureProvider
	mcp.LogBufferReader
	mcp.A11yQueryExecutor
	mcp.NoiseFilterer
}

// MaxObserveLimit caps the limit parameter to prevent oversized responses.
const MaxObserveLimit = 1000

// clampLimit applies default and max bounds to a limit parameter.
func clampLimit(limit, defaultVal int) int {
	if limit <= 0 {
		return defaultVal
	}
	if limit > MaxObserveLimit {
		return MaxObserveLimit
	}
	return limit
}
