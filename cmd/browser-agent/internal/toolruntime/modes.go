// modes.go — Names every mode the composed runtime can dispatch, per tool.
// Why: the schema-parity tests and the MCP response-shape gate both need the
// set of modes the LIVE dispatchers accept, not a hand-kept list. Reading it off
// the dispatchers means a mode added to any of the five tools joins both sweeps
// without anyone remembering to add it.
// Docs: docs/features/feature/quality-gates/index.md

package toolruntime

import (
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate"
)

// DispatchableModes maps each of the five tool names to the modes its dispatcher
// currently accepts. Generate has no per-instance registry — its handler table
// is package-level — so its modes come from the same list its validator uses.
func (h *ToolHandler) DispatchableModes() map[string][]string {
	return map[string][]string{
		"observe":   h.observeDispatcher.ValidModes(),
		"analyze":   h.analyzeDispatcher.ValidModes(),
		"generate":  strings.Split(toolgenerate.ValidFormats(), ", "),
		"configure": h.configureDispatcher.Actions(),
		"interact":  h.interactDispatcher.ActionNames(),
	}
}
