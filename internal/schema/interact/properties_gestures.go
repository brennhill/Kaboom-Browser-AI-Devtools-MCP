// Purpose: Defines the pointer-gesture and clipped-capture properties for the interact tool.
// Why: drag, right/double/triple click, hover_at, scroll_at and zoom_region take arguments no
// other interact action takes — a route, a modifier list, a wheel delta, a capture rectangle.
// Keeping them in their own group stops the targeting and core groups from becoming a catch-all.
package interact

func gestureProperties() map[string]any {
	return map[string]any{
		"drag_path": map[string]any{
			"type":        "array",
			"description": "Drag route as viewport points, in order (drag). At least 2. This is the path to follow, not just the endpoints — HTML5 drag-and-drop and canvas apps both begin their drag on the first intermediate move, so add waypoints to trace a curve. Named drag_path because 'path' is the cookie path.",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"x": map[string]any{"type": "number"},
					"y": map[string]any{"type": "number"},
				},
				"required": []string{"x", "y"},
			},
		},
		"modifiers": map[string]any{
			"type":        "array",
			"description": "Modifier keys held during the action: ctrl, shift, alt, cmd (meta). Combinable. Applies to click, type and the pointer gestures — ctrl+click opens a link in a background tab, shift+click extends a selection, and type with ctrl held sends the shortcut instead of the character (ctrl+a selects the field rather than typing an 'a').",
			"items": map[string]any{
				"type": "string",
				"enum": []string{"ctrl", "control", "shift", "alt", "cmd", "meta", "command"},
			},
		},
		"delta_x": map[string]any{
			"type":        "number",
			"description": "Horizontal wheel delta in pixels (scroll_at). Positive scrolls right.",
		},
		"delta_y": map[string]any{
			"type":        "number",
			"description": "Vertical wheel delta in pixels (scroll_at). Positive scrolls down.",
		},
		"width": map[string]any{
			"type":        "number",
			"description": "Width in pixels of the region to capture (zoom_region)",
		},
		"height": map[string]any{
			"type":        "number",
			"description": "Height in pixels of the region to capture (zoom_region)",
		},
		"scale": map[string]any{
			"type":        "number",
			"description": "Supersampling factor for the captured region, 0-4 (zoom_region, default 1). Use 2 to read small text that is illegible in a full-page screenshot.",
		},
	}
}
