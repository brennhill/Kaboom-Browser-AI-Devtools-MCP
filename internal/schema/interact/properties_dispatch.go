// Purpose: Defines canonical dispatch properties for the interact tool.
// Why: Separates dispatch/routing properties from action-specific and targeting properties.
package interact

func dispatchProperties() map[string]any {
	return map[string]any{
		"what": map[string]any{
			"type":        "string",
			"description": "Browser action to perform",
			"enum":        actionEnum,
		},
		"telemetry_mode": map[string]any{
			"type":        "string",
			"description": "Telemetry metadata mode for this call: off, auto, full",
			"enum":        []string{"off", "auto", "full"},
		},
		"background": map[string]any{
			"type":        "boolean",
			"description": "Return immediately with correlation_id instead of waiting for result (default: false).",
		},
		"reason": map[string]any{
			"type":        "string",
			"description": "Action reason (shown as toast)",
		},
		"correlation_id": map[string]any{
			"type":        "string",
			"description": "Link to error/investigation",
		},
	}
}
