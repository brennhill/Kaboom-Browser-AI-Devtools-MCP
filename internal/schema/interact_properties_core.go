// Purpose: Defines core action properties for the interact tool (highlight, state, script, storage).
// Why: Separates action-specific properties from targeting, form, and output properties.
package schema

func interactCoreActionProperties() map[string]any {
	return map[string]any{
		"duration_ms": map[string]any{
			"type":        "number",
			"description": "Highlight duration ms (default 5000)",
		},
		"snapshot_name": map[string]any{
			"type":        "string",
			"description": "State snapshot name",
		},
		"include_url": map[string]any{
			"type":        "boolean",
			"description": "Restore URL with state",
		},
		"script": map[string]any{
			"type":        "string",
			"description": "JS code (execute_js)",
		},
		"timeout_ms": map[string]any{
			"type":        "number",
			"description": "Timeout in milliseconds (default 5000, max 60000). Applies to: click, type, execute_js, wait_for, navigate, auto_dismiss_overlays, wait_for_stable, draw_mode_start.",
		},
		"text": map[string]any{
			"type":        "string",
			"description": "Text for type/subtitle. key_press keys: Enter, Tab, Escape, Backspace, ArrowDown, ArrowUp, Space.",
		},
		"subtitle": map[string]any{
			"type":        "string",
			"description": "Narration text, composable with any action. Empty clears.",
		},
		"value": map[string]any{
			"type":        "string",
			"description": "Value for select/set_attribute. Also accepted by scroll_to as a legacy direction alias.",
		},
		"direction": map[string]any{
			"type":        "string",
			"description": "Scroll direction for scroll_to: top, bottom, up, or down (preferred over value)",
			"enum":        []string{"top", "bottom", "up", "down"},
		},
		"storage_type": map[string]any{
			"type":        "string",
			"description": "Storage target for state mutation actions",
			"enum":        []string{"localStorage", "sessionStorage"},
		},
		"key": map[string]any{
			"type":        "string",
			"description": "Storage key for set_storage/delete_storage",
		},
		"clear": map[string]any{
			"type":        "boolean",
			"description": "Clear before typing",
		},
		"dispatch": map[string]any{
			"type":        "string",
			"description": "Input dispatch strategy for click/type. 'auto' (default) uses CDP hardware events; 'dom' routes through in-page element.click() + native-setter input events, which reliably drives React/Vue/Svelte controlled inputs and delegated onClick.",
			"enum":        []string{"auto", "dom"},
		},
		"checked": map[string]any{
			"type":        "boolean",
			"description": "Check/uncheck (default true)",
		},
		"name": map[string]any{
			"type":        "string",
			"description": "Attribute, recording, or cookie name",
		},
		"domain": map[string]any{
			"type":        "string",
			"description": "Cookie domain (set_cookie/delete_cookie)",
		},
		"path": map[string]any{
			"type":        "string",
			"description": "Cookie path (set_cookie/delete_cookie, default /)",
		},
		"audio": map[string]any{
			"type":        "string",
			"description": "Recording audio: tab, mic, both. Omit for video-only.",
			"enum":        []string{"tab", "mic", "both"},
		},
		"fps": map[string]any{
			"type":        "number",
			"description": "Recording FPS (5-60, default 15)",
		},
		"world": map[string]any{
			"type":        "string",
			"description": "JS world: auto (default), main (page globals), isolated (bypass CSP).",
			"enum":        []string{"auto", "main", "isolated"},
		},
		"url": map[string]any{
			"type":        "string",
			"description": "URL (navigate, new_tab)",
		},
		"include_content": map[string]any{
			"type":        "boolean",
			"description": "Return page content with navigate response (url, title, text_content, vitals)",
		},
		"tab_id": map[string]any{
			"type":        "number",
			"description": "Tab ID (default: active)",
		},
		"tab_index": map[string]any{
			"type":        "number",
			"description": "Tab index in current window ordering (switch_tab)",
		},
		"set_tracked": map[string]any{
			"type":        "boolean",
			"description": "Controls whether switch_tab updates the tracked tab to the newly activated tab (default: true). Set to false to switch focus without changing which tab the server targets for subsequent commands.",
		},
		"new_tab": map[string]any{
			"type":        "boolean",
			"description": "Open navigation URL in a background tab instead of replacing current tab (navigate)",
		},
		"analyze": map[string]any{
			"type":        "boolean",
			"description": "Enable perf profiling (captures perf_diff)",
		},
		"evidence": map[string]any{
			"type":        "string",
			"description": "Optional visual evidence capture mode: off (default), on_mutation, always.",
			"enum":        []string{"off", "on_mutation", "always"},
		},
		"observe_mutations": map[string]any{
			"type":        "boolean",
			"description": "Track element-level DOM mutations during action execution",
		},
		"annot_session": map[string]any{
			"type":        "string",
			"description": "Named session for multi-page annotation review (applies to draw_mode_start). Accumulates annotations across pages under a shared session name.",
		},
		"file_path": map[string]any{
			"type":        "string",
			"description": "Absolute file path for upload action",
		},
		"api_endpoint": map[string]any{
			"type":        "string",
			"description": "API endpoint for direct upload mode",
		},
		"submit": map[string]any{
			"type":        "boolean",
			"description": "Submit form after upload",
		},
		"escalation_timeout_ms": map[string]any{
			"type":        "number",
			"description": "Upload escalation timeout in ms",
		},
	}
}
