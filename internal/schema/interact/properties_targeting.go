// Purpose: Defines element targeting properties for the interact tool (selector, scope, index, nth).
// Why: Separates targeting properties from action-specific and output properties.
package interact

func targetingProperties() map[string]any {
	return map[string]any{
		"selector": map[string]any{
			"type":        "string",
			"description": "CSS or semantic selector for target element",
		},
		"scope_selector": map[string]any{
			"type":        "string",
			"description": "Optional container selector to constrain DOM actions to a specific region",
		},
		"scope_rect": map[string]any{
			"type":        "object",
			"description": "Optional viewport rectangle to constrain DOM actions by geometry (x/y/width/height)",
			"properties": map[string]any{
				"x":      map[string]any{"type": "number"},
				"y":      map[string]any{"type": "number"},
				"width":  map[string]any{"type": "number"},
				"height": map[string]any{"type": "number"},
			},
		},
		"element_id": map[string]any{
			"type":        "string",
			"description": "Stable element handle from list_interactive (preferred for deterministic follow-up actions)",
		},
		"index": map[string]any{
			"type":        "number",
			"description": "Element index from list_interactive results (legacy alternative to selector/element_id)",
		},
		"index_generation": map[string]any{
			"type":        "string",
			"description": "Generation token from list_interactive to ensure index resolves against the same element snapshot",
		},
		"nth": map[string]any{
			"type":        "integer",
			"description": "Select the Nth matching element when a selector matches multiple. 0 = first visible match, 1 = second, etc. Negative values count from end (-1 = last). Prefers visible elements when available.",
		},
		"x": map[string]any{
			"type":        "number",
			"description": "X coordinate in pixels from left edge (click, hardware_click)",
		},
		"y": map[string]any{
			"type":        "number",
			"description": "Y coordinate in pixels from top edge (click, hardware_click)",
		},
		"visible_only": map[string]any{
			"type":        "boolean",
			"description": "Only return visible elements (list_interactive)",
		},
		"limit": map[string]any{
			"type":        "number",
			"description": "Max elements to return (list_interactive, default all)",
		},
		"text_contains": map[string]any{
			"type":        "string",
			"description": "Filter list_interactive elements whose label contains this substring (case-insensitive)",
		},
		"role": map[string]any{
			"type":        "string",
			"description": "Filter list_interactive elements by element type or ARIA role (e.g., 'button', 'link', 'input', 'tab')",
		},
		"exclude_nav": map[string]any{
			"type":        "boolean",
			"description": "Exclude elements inside navigation containers — nav, header, or role=navigation (list_interactive)",
		},
		"query_type": map[string]any{
			"type":        "string",
			"description": "Query operation type for interact(what='query'): exists, count, text, text_all, attributes",
			"enum":        []string{"exists", "count", "text", "text_all", "attributes"},
		},
		"attribute_names": map[string]any{
			"type":        "array",
			"description": "Attribute names to read for query_type='attributes' (e.g., ['href', 'data-id'])",
			"items":       map[string]any{"type": "string"},
		},
		"frame": map[string]any{
			"description": "Target iframe: CSS selector, 0-based index, or \"all\"",
			"type":        "string",
		},
	}
}
