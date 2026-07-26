// Purpose: Defines core MCP schema properties for the configure tool (what, action, mode, tool).
// Why: Separates core dispatch properties from runtime configuration properties.
package configure

func coreProperties() map[string]any {
	return map[string]any{
		"what": map[string]any{
			"type":        "string",
			"description": "Setting or utility to configure",
			"enum":        []string{"store", "load", "noise_rule", "clear", "health", "tutorial", "examples", "streaming", "test_boundary_start", "test_boundary_end", "event_recording_start", "event_recording_stop", "playback", "log_diff", "telemetry", "describe_capabilities", "diff_sessions", "audit_log", "restart", "save_sequence", "get_sequence", "list_sequences", "delete_sequence", "replay_sequence", "doctor", "security_mode", "network_recording", "action_jitter", "report_issue", "setup_quality_gates"},
		},
		"action": map[string]any{
			"type":        "string",
			"description": "Deprecated alias for 'what'. Prefer 'what'.",
		},
		"mode": map[string]any{
			"type":        "string",
			"description": "For security_mode: 'normal' or 'insecure_proxy'. For describe_capabilities: tool mode name to filter (e.g. 'errors', 'click').",
		},
		"tool": map[string]any{
			"type":        "string",
			"description": "Filter describe_capabilities to a single tool by name (e.g. 'observe', 'interact')",
		},
		"confirm": map[string]any{
			"type":        "boolean",
			"description": "Required true when enabling insecure_proxy mode.",
		},
		"telemetry_mode": map[string]any{
			"type":        "string",
			"description": "Telemetry metadata mode: off, auto, full. configure(what='telemetry') sets global default. Any tools/call may override per request with telemetry_mode.",
			"enum":        []string{"off", "auto", "full"},
		},
		"store_action": map[string]any{
			"type":        "string",
			"description": "Store operation (default: list)",
			"enum":        []string{"save", "load", "list", "delete", "stats"},
			"default":     "list",
		},
		"namespace": map[string]any{
			"type":        "string",
			"description": "Store grouping (default: session)",
			"default":     "session",
		},
		"key": map[string]any{
			"type":        "string",
			"description": "Storage key",
		},
		"data": map[string]any{
			"type":        "object",
			"description": "JSON data to persist",
		},
		"value": map[string]any{
			"type":        "string",
			"description": "Flat value alias for save action; treated as data when provided",
		},
		"noise_action": map[string]any{
			"type":        "string",
			"description": "Noise operation (default: list)",
			"enum":        []string{"add", "remove", "list", "reset", "auto_detect"},
			"default":     "list",
		},
		"rules": map[string]any{
			"type":        "array",
			"description": "Noise rules to add",
			"items":       map[string]any{"type": "object"},
		},
		"classification": map[string]any{
			"type":        "string",
			"description": "Single-rule flattening helper for noise_action=add",
		},
		"message_regex": map[string]any{
			"type":        "string",
			"description": "Single-rule flattening helper for noise_action=add",
		},
		"source_regex": map[string]any{
			"type":        "string",
			"description": "Single-rule flattening helper for noise_action=add",
		},
		"url_regex": map[string]any{
			"type":        "string",
			"description": "Single-rule flattening helper for noise_action=add",
		},
		"method": map[string]any{
			"type":        "string",
			"description": "HTTP method filter (noise_action=add, network_recording)",
		},
		"domain": map[string]any{
			"type":        "string",
			"description": "Domain filter for network_recording",
		},
		"status_min": map[string]any{
			"type":        "integer",
			"description": "Single-rule flattening helper for noise_action=add",
		},
		"status_max": map[string]any{
			"type":        "integer",
			"description": "Single-rule flattening helper for noise_action=add",
		},
		"level": map[string]any{
			"type":        "string",
			"description": "Single-rule flattening helper for noise_action=add",
		},
		"rule_id": map[string]any{
			"type":        "string",
			"description": "Rule ID to remove",
		},
		"pattern": map[string]any{
			"type":        "string",
			"description": "Regex pattern (single-rule flattening helper for noise_action=add)",
		},
		"category": map[string]any{
			"type":        "string",
			"description": "Noise category (default: console for flattened add)",
			"enum":        []string{"console", "network", "websocket"},
			"default":     "console",
		},
		"reason": map[string]any{
			"type":        "string",
			"description": "Why this is noise",
		},
	}
}
