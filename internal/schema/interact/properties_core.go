// Purpose: Defines core action properties for the interact tool (highlight, state, script, storage).
// Why: Separates action-specific properties from targeting, form, and output properties.
package interact

func coreActionProperties() map[string]any {
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
			"description": "Value for select or set_attribute.",
		},
		"direction": map[string]any{
			"type":        "string",
			"description": "Scroll direction for scroll_to: top, bottom, up, or down.",
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

// findProperties are the parameters of the accessibility-tree `find` action. Kept as their
// own group so the core property map stays inside its length budget.
func findProperties() map[string]any {
	return map[string]any{
		"query": map[string]any{
			"type":        "string",
			"description": "For find: a natural-language description of the element, e.g. 'add to cart button' or 'search bar'. Matched against the accessibility tree (role + accessible name), so it reaches controls no CSS selector can name.",
		},
	}
}

// environmentPinProperties are the parameters of the `pin_environment` action. One object
// property rather than nine flat ones: the interact schema shares a single property map
// across every action, so nine loose names would collide with targeting and geometry params.
func environmentPinProperties() map[string]any {
	return map[string]any{
		"environment": map[string]any{
			"type":        "object",
			"description": "For pin_environment: the environment knobs to hold still. Everything named here is reported in the generated reproduction artifact, because a test that silently depends on a pinned clock passes only on the machine that recorded it.",
			"properties": map[string]any{
				"clock_epoch_ms": map[string]any{"type": "number", "description": "Fix the clock's origin to this epoch (ms)"},
				"timezone_id":    map[string]any{"type": "string", "description": "IANA timezone, e.g. 'UTC' or 'America/New_York'"},
				"virtual_time_policy": map[string]any{
					"type":        "string",
					"enum":        []string{"advance", "pause", "pauseIfNetworkFetchesPending"},
					"description": "Default 'advance' — fixes the clock's origin and lets it run. 'pause' stops time outright, which freezes the page you are recording; use it for replay only.",
				},
				"latitude":            map[string]any{"type": "number", "description": "Geolocation latitude"},
				"longitude":           map[string]any{"type": "number", "description": "Geolocation longitude"},
				"accuracy_m":          map[string]any{"type": "number", "description": "Geolocation accuracy in metres (default 1)"},
				"viewport_width":      map[string]any{"type": "number", "description": "Device metrics width in CSS pixels"},
				"viewport_height":     map[string]any{"type": "number", "description": "Device metrics height in CSS pixels"},
				"device_scale_factor": map[string]any{"type": "number", "description": "Device pixel ratio (default 1)"},
				"mobile":              map[string]any{"type": "boolean", "description": "Emulate a mobile device"},
				"random_seed":         map[string]any{"type": "string", "description": "Seed Math.random and crypto.getRandomValues so a replay draws the same values"},
			},
		},
	}
}
