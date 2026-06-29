// Purpose: Builds diagnostic hint context for structured error responses.
// Why: Centralizes extension/pilot/tab/csp state rendering used by multiple gate checks.

package main

import "fmt"

// diagnosticHintString returns a plain-text snapshot of system state.
// Used by both structured errors and JSON error responses.
func (h *ToolHandler) DiagnosticHintString() string {
	extConnected := h.capture.IsExtensionConnected()
	pilotEnabled := h.capture.IsPilotEnabled()
	pilotState := ""
	if status, ok := h.capture.GetPilotStatus().(map[string]any); ok {
		if state, ok := status["state"].(string); ok {
			pilotState = state
		}
		if effective, ok := status["enabled"].(bool); ok {
			pilotEnabled = effective
		}
	}
	enabled, tabID, tabURL := h.capture.GetTrackingStatus()

	var parts []string
	if extConnected {
		parts = append(parts, "extension=connected")
	} else {
		parts = append(parts, "extension=DISCONNECTED")
	}
	pilotStatus := "pilot=DISABLED"
	switch pilotState {
	case "assumed_enabled":
		pilotStatus = "pilot=ASSUMED_ENABLED(startup)"
	case "explicitly_disabled":
		pilotStatus = "pilot=DISABLED(explicit)"
	case "enabled":
		pilotStatus = "pilot=enabled"
	default:
		if pilotEnabled {
			pilotStatus = "pilot=enabled"
		}
	}
	parts = append(parts, pilotStatus)
	if enabled && tabURL != "" {
		parts = append(parts, fmt.Sprintf("tracked_tab=%q (id=%d)", tabURL, tabID))
	} else {
		parts = append(parts, "tracked_tab=NONE")
	}

	cspRestricted, cspLevel := h.capture.GetCSPStatus()
	if cspRestricted {
		parts = append(parts, fmt.Sprintf("csp=RESTRICTED(%s)", cspLevel))
	} else {
		parts = append(parts, "csp=clear")
	}

	hint := "Current state: " + parts[0]
	for _, p := range parts[1:] {
		hint += ", " + p
	}
	return hint
}

// diagnosticHint returns a snapshot of system state for inclusion in structured errors.
func (h *ToolHandler) diagnosticHint() func(*StructuredError) {
	return withHint(h.DiagnosticHintString())
}
