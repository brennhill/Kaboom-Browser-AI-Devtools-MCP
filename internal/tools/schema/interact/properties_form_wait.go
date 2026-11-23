// Purpose: Defines form-filling and wait-condition properties for the interact tool.
// Why: Separates form and wait properties from targeting, output, and core action properties.
package interact

func formAndWaitProperties() map[string]any {
	return map[string]any{
		"fields": map[string]any{
			"type":        "array",
			"description": "Form fields to fill (fill_form, fill_form_and_submit)",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"selector": map[string]any{"type": "string", "description": "CSS selector for the field"},
					"value":    map[string]any{"type": "string", "description": "Value to type into the field"},
					"index":    map[string]any{"type": "number", "description": "Element index from list_interactive (alternative to selector)"},
				},
				"required": []string{"value"},
			},
		},
		"submit_selector": map[string]any{
			"type":        "string",
			"description": "Submit button selector (fill_form_and_submit)",
		},
		"submit_index": map[string]any{
			"type":        "number",
			"description": "Submit button index from list_interactive (fill_form_and_submit)",
		},
		"wait_for": map[string]any{
			"type":        "string",
			"description": "CSS selector to wait for after navigation (navigate_and_wait_for)",
		},
		"url_contains": map[string]any{
			"type":        "string",
			"description": "Wait for URL to contain this substring (wait_for)",
		},
		"absent": map[string]any{
			"type":        "boolean",
			"description": "Wait for element to disappear (wait_for)",
		},
		"structured": map[string]any{
			"type":        "boolean",
			"description": "Return nested/hierarchical text extraction (get_text)",
		},
		"save_to": map[string]any{
			"type":        "string",
			"description": "File path to save output (run_a11y_and_export_sarif)",
		},
	}
}
