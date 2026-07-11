// Purpose: Parses and validates evidence configuration from interact command parameters.
// Why: Centralizes evidence mode resolution so all interact handlers share consistent config logic.

package interacthandler

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func ParseEvidenceMode(args json.RawMessage) (evidenceMode, error) {
	var params struct {
		Evidence string `json:"evidence"`
	}
	lenientUnmarshal(args, &params)
	raw := strings.TrimSpace(params.Evidence)
	if raw == "" {
		return evidenceModeOff, nil
	}

	mode := evidenceMode(strings.ToLower(raw))
	switch mode {
	case evidenceModeOff, evidenceModeOnMutation, evidenceModeAlways:
		return mode, nil
	default:
		return evidenceModeOff, fmt.Errorf("interact_evidence: invalid evidence mode %q. Valid modes: off, on_mutation, always", raw)
	}
}

func evidenceMaxCapturesPerCommand() int {
	return parseBoundedEnvInt(evidenceMaxCapturesEnv, 2, 0, 2)
}

func evidenceRetryCount() int {
	return parseBoundedEnvInt(evidenceRetryEnv, 1, 0, 3)
}

func parseBoundedEnvInt(name string, def, min, max int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func canonicalActionFromInteractArgs(args json.RawMessage) string {
	var params struct {
		What   string `json:"what"`
		Action string `json:"action"`
	}
	lenientUnmarshal(args, &params)
	action := strings.TrimSpace(params.What)
	if action == "" {
		action = strings.TrimSpace(params.Action)
	}
	return strings.ToLower(action)
}

func isMutationAction(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case
		"highlight",
		"execute_js",
		"navigate", "refresh", "back", "forward", "new_tab", "switch_tab", "close_tab", "activate_tab",
		"click", "type", "select", "check", "paste", "key_press",
		"set_attribute", "scroll_to", "focus", "hover",
		"open_composer", "submit_active_composer", "confirm_top_dialog", "dismiss_top_overlay",
		"set_storage", "delete_storage", "clear_storage",
		"set_cookie", "delete_cookie",
		"fill_form", "fill_form_and_submit",
		"upload":
		return true
	default:
		return false
	}
}
