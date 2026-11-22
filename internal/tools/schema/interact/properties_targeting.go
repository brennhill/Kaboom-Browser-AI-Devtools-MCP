// Purpose: Defines element targeting properties for the interact tool (selector, scope, index, ref, nth).
// Why: Separates targeting properties from action-specific and output properties.
package interact

// targetingProperties is the union of the two groups below. It is split because a single
// map literal covering both had outgrown the function-length budget, and because "how do I
// name one element" and "which elements should the listing return" are different questions.
func targetingProperties() map[string]any {
	properties := elementAddressProperties()
	for name, schema := range listingFilterProperties() {
		properties[name] = schema
	}
	return properties
}

// elementAddressProperties name a single element to act on.
func elementAddressProperties() map[string]any {
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
		"ref": map[string]any{
			"type":        "string",
			"description": "Accessibility ref from find results (e.g. 'ax_412'). Resolves to the element find ranked, so it reaches controls no CSS selector names. Quote index_generation with it: a ref from an earlier snapshot is refused rather than resolved against a re-rendered page.",
		},
		"index_generation": map[string]any{
			"type":        "string",
			"description": "Generation token from list_interactive or find, so index/ref resolve against the same element snapshot they came from",
		},
		"nth": map[string]any{
			"type":        "integer",
			"description": "Select the Nth matching element when a selector matches multiple. 0 = first visible match, 1 = second, etc. Negative values count from end (-1 = last). Prefers visible elements when available.",
		},
		"x": map[string]any{
			"type":        "number",
			"description": "X in viewport CSS pixels from the left edge of the visible area — the space a screenshot's coordinate_frame maps image pixels into. An alternative to selector/ref for click, right_click, double_click, triple_click, hover_at and scroll_at; also the region origin for zoom_region. Send it with y, and with no other target.",
		},
		"y": map[string]any{
			"type":        "number",
			"description": "Y in viewport CSS pixels from the top edge of the visible area — the space a screenshot's coordinate_frame maps image pixels into. An alternative to selector/ref for click, right_click, double_click, triple_click, hover_at and scroll_at; also the region origin for zoom_region. Send it with x, and with no other target.",
		},
	}
}

// listingFilterProperties shape what list_interactive, explore_page and query return.
func listingFilterProperties() map[string]any {
	return map[string]any{
		"visible_only": map[string]any{
			"type":        "boolean",
			"description": "Only return visible elements (list_interactive)",
		},
		"limit": map[string]any{
			"type":        "number",
			"description": "Max elements to return (list_interactive, default all)",
		},
		"verbose": map[string]any{
			"type":        "boolean",
			"description": "Return every element field including bbox, tag, landmark and overlay context (list_interactive, explore_page). Default false returns element_id, label, element_type, selector and index — enough to choose and act on an element at a fraction of the tokens. Ask for verbose only when you need geometry or landmark grouping.",
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
