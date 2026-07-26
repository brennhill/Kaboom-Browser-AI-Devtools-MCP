// Purpose: Defines MCP tool input types and validation for session verification actions.
// Why: Separates tool parameter handling from verification computation and snapshot logic.
package verify

import (
	"encoding/json"
	"fmt"
)

// verifyFixParams defines the MCP tool input schema
type verifyFixParams struct {
	Action         string `json:"action"`
	VerifSessionID string `json:"verif_session_id,omitempty"`
	Label          string `json:"label,omitempty"`
	URLFilter      string `json:"url,omitempty"`
}

// requireVerifSessionID returns an error if session_id is empty for the given action.
func requireVerifSessionID(sessionID, action string) error {
	if sessionID == "" {
		return fmt.Errorf("'verif_session_id' is required for %s action", action)
	}
	return nil
}

// handleVerifyWatch handles the "watch" action.
func (vm *VerificationManager) handleVerifyWatch(p verifyFixParams) (any, error) {
	if err := requireVerifSessionID(p.VerifSessionID, "watch"); err != nil {
		return nil, err
	}
	result, err := vm.Watch(p.VerifSessionID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"verif_session_id": result.VerifSessionID, "status": result.Status, "message": result.Message,
	}, nil
}

// handleVerifyCompare handles the "compare" action.
func (vm *VerificationManager) handleVerifyCompare(p verifyFixParams) (any, error) {
	if err := requireVerifSessionID(p.VerifSessionID, "compare"); err != nil {
		return nil, err
	}
	result, err := vm.Compare(p.VerifSessionID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"verif_session_id": result.VerifSessionID, "status": result.Status, "label": result.Label,
		"result": map[string]any{
			"verdict": result.Result.Verdict, "before": result.Result.Before,
			"after": result.Result.After, "changes": result.Result.Changes,
			"new_issues": result.Result.NewIssues, "performance_diff": result.Result.PerformanceDiff,
		},
	}, nil
}

// handleVerifyStatus handles the "status" action.
func (vm *VerificationManager) handleVerifyStatus(p verifyFixParams) (any, error) {
	if err := requireVerifSessionID(p.VerifSessionID, "status"); err != nil {
		return nil, err
	}
	result, err := vm.Status(p.VerifSessionID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"verif_session_id": result.VerifSessionID, "status": result.Status,
		"label": result.Label, "created_at": result.CreatedAt,
	}, nil
}

// handleVerifyCancel handles the "cancel" action.
func (vm *VerificationManager) handleVerifyCancel(p verifyFixParams) (any, error) {
	if err := requireVerifSessionID(p.VerifSessionID, "cancel"); err != nil {
		return nil, err
	}
	result, err := vm.Cancel(p.VerifSessionID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"verif_session_id": result.VerifSessionID, "status": result.Status}, nil
}

// HandleTool dispatches the verify_fix MCP tool call
func (vm *VerificationManager) HandleTool(params json.RawMessage) (any, error) {
	var p verifyFixParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	switch p.Action {
	case "start":
		result, err := vm.Start(p.Label, p.URLFilter)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"verif_session_id": result.VerifSessionID, "status": result.Status,
			"label": result.Label, "baseline": result.Baseline,
		}, nil
	case "watch":
		return vm.handleVerifyWatch(p)
	case "compare":
		return vm.handleVerifyCompare(p)
	case "status":
		return vm.handleVerifyStatus(p)
	case "cancel":
		return vm.handleVerifyCancel(p)
	default:
		if p.Action == "" {
			return nil, fmt.Errorf("'action' is required (start, watch, compare, status, cancel)")
		}
		return nil, fmt.Errorf("unknown action %q (valid: start, watch, compare, status, cancel)", p.Action)
	}
}
