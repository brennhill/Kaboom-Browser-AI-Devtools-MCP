// Purpose: Dispatches diff_sessions MCP tool calls to capture, compare, list, and delete actions.
// Docs: docs/features/feature/request-session-correlation/index.md

// tool-handler.go — Tool handler for diff_sessions MCP tool.
// HandleTool function dispatches capture, compare, list, delete actions.
package session

import (
	"encoding/json"
	"fmt"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
)

// diffSessionsParams defines the MCP tool input schema.
type diffSessionsParams struct {
	Action             string             `json:"action"`
	Name               string             `json:"name,omitempty"`
	CompareA           string             `json:"compare_a,omitempty"`
	CompareB           string             `json:"compare_b,omitempty"`
	URLFilter          string             `json:"url,omitempty"`
	PerformanceBudgets map[string]float64 `json:"performance_budgets,omitempty"`
}

// handleCapture handles the "capture" action for diff_sessions.
func (sm *SessionManager) handleCapture(p diffSessionsParams) (any, error) {
	if p.Name == "" {
		return nil, fmt.Errorf("'name' is required for capture action")
	}
	snap, err := sm.Capture(p.Name, p.URLFilter)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"action": "captured",
		"name":   snap.Name,
		"snapshot": map[string]any{
			"captured_at":      snap.CapturedAt,
			"console_errors":   len(snap.ConsoleErrors),
			"console_warnings": len(snap.ConsoleWarnings),
			"network_requests": len(snap.NetworkRequests),
			"page_url":         snap.PageURL,
		},
	}, nil
}

// handleCompare handles the "compare" action for diff_sessions.
func (sm *SessionManager) handleCompare(p diffSessionsParams) (any, error) {
	if p.CompareA == "" || p.CompareB == "" {
		return nil, fmt.Errorf("'compare_a' and 'compare_b' are required for compare action")
	}
	for name, allowed := range p.PerformanceBudgets {
		if !performance.IsBudgetMetric(name) {
			return nil, fmt.Errorf("unknown performance budget %q", name)
		}
		if allowed < 0 {
			return nil, fmt.Errorf("performance budget %q must be non-negative", name)
		}
	}
	diff, err := sm.CompareWithBudgets(p.CompareA, p.CompareB, p.PerformanceBudgets)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"action": "compared", "a": diff.A, "b": diff.B,
		"diff": diff, "summary": diff.Summary,
	}, nil
}

// handleDelete handles the "delete" action for diff_sessions.
func (sm *SessionManager) handleDelete(p diffSessionsParams) (any, error) {
	if p.Name == "" {
		return nil, fmt.Errorf("'name' is required for delete action")
	}
	if err := sm.Delete(p.Name); err != nil {
		return nil, err
	}
	return map[string]any{"action": "deleted", "name": p.Name}, nil
}

// HandleTool dispatches the diff_sessions MCP tool call.
func (sm *SessionManager) HandleTool(params json.RawMessage) (any, error) {
	var p diffSessionsParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	switch p.Action {
	case "capture":
		return sm.handleCapture(p)
	case "compare":
		return sm.handleCompare(p)
	case "list":
		return map[string]any{"action": "listed", "snapshots": sm.List()}, nil
	case "delete":
		return sm.handleDelete(p)
	default:
		if p.Action == "" {
			return nil, fmt.Errorf("'action' is required (capture, compare, list, delete)")
		}
		return nil, fmt.Errorf("unknown action %q (valid: capture, compare, list, delete)", p.Action)
	}
}
