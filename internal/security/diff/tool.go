// tool.go — MCP action dispatch for snapshot/compare/list.
// Purpose: Exposes security snapshot and diff operations via the MCP tool contract.
// Why: Keeps tool-specific request parsing separate from domain comparison logic.
// Docs: docs/features/feature/security-hardening/index.md

package diff

import (
	"encoding/json"
	"fmt"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// HandleDiffSecurity dispatches MCP diff tool actions.
//
// Failure semantics:
// - Invalid JSON/action returns explicit error and performs no mutation.
func (m *Manager) HandleDiffSecurity(params json.RawMessage, bodies []capture.NetworkBody) (any, error) {
	var toolParams struct {
		Action      string `json:"action"`
		Name        string `json:"name"`
		CompareFrom string `json:"compare_from"`
		CompareTo   string `json:"compare_to"`
	}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &toolParams); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}
	}

	switch toolParams.Action {
	case "snapshot":
		return m.TakeSnapshot(toolParams.Name, bodies)
	case "compare":
		return m.Compare(toolParams.CompareFrom, toolParams.CompareTo, bodies)
	case "list":
		return m.ListSnapshots(), nil
	default:
		return nil, fmt.Errorf("unknown action %q; use 'snapshot', 'compare', or 'list'", toolParams.Action)
	}
}
