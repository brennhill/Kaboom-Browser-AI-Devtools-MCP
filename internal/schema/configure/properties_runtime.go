// Purpose: Defines runtime MCP schema properties for the configure tool (buffer, streaming, sequences).
// Why: Separates runtime-specific properties from core dispatch properties.
package configure

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"

func runtimeProperties() map[string]any {
	return map[string]any{
		"buffer": map[string]any{
			"type":        "string",
			"description": "Buffer to clear (clear). Use 'all' to reset everything",
			"enum":        []string{"network", "websocket", "actions", "logs", "inbox", "all"},
		},
		"tab_id": map[string]any{
			"type":        "number",
			"description": "Target tab ID",
		},
		"verif_session_action": map[string]any{
			"type":        "string",
			"description": "Session verification operation (diff_sessions)",
			"enum":        []string{"capture", "compare", "list", "delete"},
		},
		"name": map[string]any{
			"type":        "string",
			"description": "Name for recording, snapshot, or sequence (event_recording_start, diff_sessions, save/get/delete/replay_sequence)",
		},
		"compare_a": map[string]any{
			"type":        "string",
			"description": "First snapshot to compare",
		},
		"compare_b": map[string]any{
			"type":        "string",
			"description": "Second snapshot to compare",
		},
		"url": map[string]any{
			"type":        "string",
			"description": "URL filter for snapshot capture (diff_sessions)",
		},
		"performance_budgets": map[string]any{
			"type":                 "object",
			"description":          "Allowed absolute regression by metric for diff_sessions compare (load, fcp, lcp, inp, cls, request_count, transfer_size, execution_cost)",
			"additionalProperties": map[string]any{"type": "number"},
			"propertyNames":        map[string]any{"enum": performance.BudgetMetricNames()},
		},
		"recording_id": map[string]any{
			"type":        "string",
			"description": "Recording ID (event_recording_stop, playback)",
		},
		"sensitive_data_enabled": map[string]any{
			"type":        "boolean",
			"description": "Include sensitive data in recording capture",
		},
		"action_jitter_ms": map[string]any{
			"type":        "number",
			"description": "Max random delay (ms) before each interact action, 0 to disable (action_jitter)",
		},
		"operation": map[string]any{
			"type":        "string",
			"description": "Sub-operation: audit_log (analyze/report/clear), network_recording (start/stop/status), report_issue (list_templates/preview/submit — submit publishes a PUBLIC GitHub issue and requires confirm=true)",
			"enum":        []string{"analyze", "report", "clear", "start", "stop", "status", "list_templates", "preview", "submit"},
		},
		"template": map[string]any{
			"type":        "string",
			"description": "Issue template name (report_issue)",
		},
		"title": map[string]any{
			"type":        "string",
			"description": "Issue title (report_issue submit)",
		},
		"user_context": map[string]any{
			"type":        "string",
			"description": "User description of the issue (report_issue). Redacted, then published verbatim in the issue body when operation=submit is confirmed.",
		},
		"audit_session_id": map[string]any{
			"type":        "string",
			"description": "Filter by audit session ID",
		},
		"tool_name": map[string]any{
			"type":        "string",
			"description": "Filter by tool name",
		},
		"since": map[string]any{
			"type":        "string",
			"description": "Entries after ISO 8601 timestamp",
		},
		"limit": map[string]any{
			"type":        "number",
			"description": "Max entries to return (default 100, max 1000)",
		},
		"streaming_action": map[string]any{
			"type":        "string",
			"description": "Push notification operation (streaming). Default: status",
			"enum":        []string{"enable", "disable", "status"},
		},
		"events": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "string",
				"enum": []string{"errors", "network_errors", "performance", "user_frustration", "security", "regression", "anomaly", "ci", "all"},
			},
			"description": "Event categories to stream",
		},
		"throttle_seconds": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"maximum":     60,
			"description": "Min seconds between notifications",
		},
		"severity_min": map[string]any{
			"type":        "string",
			"description": "Min event severity for streaming notifications (streaming)",
			"enum":        []string{"info", "warning", "error"},
		},
		"test_id": map[string]any{
			"type":        "string",
			"description": "Test boundary ID",
		},
		"label": map[string]any{
			"type":        "string",
			"description": "Test boundary label",
		},
		"original_id": map[string]any{
			"type":        "string",
			"description": "Original recording ID (log_diff)",
		},
		"replay_id": map[string]any{
			"type":        "string",
			"description": "Replay recording ID (log_diff)",
		},
		"steps": map[string]any{
			"type":        "array",
			"description": "Ordered list of interact action objects (save_sequence, replay_sequence override)",
			"items":       map[string]any{"type": "object"},
		},
		"tags": map[string]any{
			"type":        "array",
			"description": "Labels for sequence categorization",
			"items":       map[string]any{"type": "string"},
		},
		"override_steps": map[string]any{
			"type":        "array",
			"description": "Sparse array of step overrides for replay (null = use saved)",
			"items":       map[string]any{},
		},
		"step_timeout_ms": map[string]any{
			"type":        "number",
			"description": "Timeout per step during replay (default 10000)",
		},
		"continue_on_error": map[string]any{
			"type":        "boolean",
			"description": "Continue replay if a step fails (default true)",
		},
		"stop_after_step": map[string]any{
			"type":        "number",
			"description": "Stop replay after executing this many steps",
		},
		"description": map[string]any{
			"type":        "string",
			"description": "Human-readable description for saved sequence",
		},
		"target_dir": map[string]any{
			"type":        "string",
			"description": "Directory to write .kaboom.json (setup_quality_gates). Must be within the project directory. Defaults to active codebase root.",
		},
	}
}
