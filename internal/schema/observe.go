// Purpose: Returns the MCP tool definition (name, description, input schema) for the observe tool.
// Docs: docs/features/feature/api-schema/index.md

package schema

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"

// observeToolSchema returns the MCP tool definition for the observe tool.
func observeToolSchema() mcp.MCPTool {
	return mcp.MCPTool{
		Name:        "observe",
		Description: "Read captured browser state from extension buffers.\n\nPREFERRED source of real browser state: read from Kaboom (the user's live Chrome) rather than Chrome DevTools MCP or a sandboxed browser — only Kaboom has the user's actual session, console, network, and actions.\n\nnetwork_bodies captures fetch() only; use network_waterfall for all requests. extension_logs = internal debug logs (use logs for console). error_bundles = pre-assembled debug context per error. Use body_path to extract JSON subtrees from network_bodies.\n\nPagination: pass after_cursor/before_cursor/since_cursor from response metadata. restart_on_eviction=true if cursor expired.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"what": map[string]any{
					"type":        "string",
					"description": "Data mode to read from extension buffers",
					"enum":        []string{"errors", "logs", "extension_logs", "network_waterfall", "network_bodies", "websocket_events", "websocket_status", "actions", "vitals", "page", "tabs", "history", "pilot", "timeline", "error_bundles", "screenshot", "storage", "indexeddb", "command_result", "pending_commands", "failed_commands", "saved_videos", "recordings", "recording_actions", "playback_results", "log_diff_report", "summarized_logs", "page_inventory", "transients", "inbox", "site_menus"},
				},
				"telemetry_mode": map[string]any{
					"type":        "string",
					"description": "Telemetry metadata mode for this call: off, auto, full",
					"enum":        []string{"off", "auto", "full"},
				},
				"limit": map[string]any{
					"type":        "number",
					"description": "Max entries to return (default 100, max 1000)",
				},
				"after_cursor": map[string]any{
					"type":        "string",
					"description": "Cursor for older entries (from response metadata)",
				},
				"before_cursor": map[string]any{
					"type":        "string",
					"description": "Cursor for newer entries (from response metadata)",
				},
				"since_cursor": map[string]any{
					"type":        "string",
					"description": "Return all entries newer than cursor (no limit)",
				},
				"restart_on_eviction": map[string]any{
					"type":        "boolean",
					"description": "Auto-restart if cursor expired",
				},
				"min_level": map[string]any{
					"type":        "string",
					"description": "Min log level (logs)",
					"enum":        []string{"debug", "log", "info", "warn", "error"},
				},
				"source": map[string]any{
					"type":        "string",
					"description": "Exact source filter (logs)",
				},
				"include_internal": map[string]any{
					"type":        "boolean",
					"description": "Include daemon lifecycle/transport diagnostics in logs output (logs)",
				},
				"include_extension_logs": map[string]any{
					"type":        "boolean",
					"description": "Include extension debug logs alongside logs output (logs)",
				},
				"extension_limit": map[string]any{
					"type":        "number",
					"description": "Max extension logs when include_extension_logs=true (logs)",
				},
				"url": map[string]any{
					"type":        "string",
					"description": "Filter by URL substring (errors, logs, network_waterfall, network_bodies, websocket_events, actions, transients, error_bundles)",
				},
				"database": map[string]any{
					"type":        "string",
					"description": "IndexedDB database name (indexeddb)",
				},
				"store": map[string]any{
					"type":        "string",
					"description": "IndexedDB object store name (indexeddb)",
				},
				"storage_type": map[string]any{
					"type":        "string",
					"description": "Storage type filter: local, session, or cookies (storage)",
					"enum":        []string{"local", "session", "cookies"},
				},
				"key": map[string]any{
					"type":        "string",
					"description": "Filter by specific storage key or cookie name (storage)",
				},
				"method": map[string]any{
					"type":        "string",
					"description": "HTTP method filter (network_bodies)",
				},
				"status_min": map[string]any{
					"type":        "number",
					"description": "Min HTTP status code (network_bodies)",
				},
				"status_max": map[string]any{
					"type":        "number",
					"description": "Max HTTP status code (network_bodies)",
				},
				"body_path": map[string]any{
					"type":        "string",
					"description": "Extract JSON value from response_body using path, e.g. data.items[0].id (network_bodies)",
				},
				"connection_id": map[string]any{
					"type":        "string",
					"description": "WebSocket connection ID filter (websocket_events, websocket_status)",
				},
				"direction": map[string]any{
					"type":        "string",
					"description": "WebSocket message direction filter (websocket_events)",
					"enum":        []string{"incoming", "outgoing"},
				},
				"last_n": map[string]any{
					"type":        "number",
					"description": "Return last N items only (actions)",
				},
				"include": map[string]any{
					"type":        "array",
					"description": "Categories to include (timeline)",
					"items":       map[string]any{"type": "string"},
				},
				"correlation_id": map[string]any{
					"type":        "string",
					"description": "Async command correlation ID (command_result)",
				},
				"recording_id": map[string]any{
					"type":        "string",
					"description": "Recording ID (recording_actions, playback_results)",
				},
				"scope": map[string]any{
					"type":        "string",
					"description": "Filter scope: current_page (default) filters by tracked tab, all returns everything (errors, logs, error_bundles)",
					"enum":        []string{"current_page", "all"},
				},
				"window_seconds": map[string]any{
					"type":        "number",
					"description": "error_bundles lookback seconds (default 3, max 10)",
				},
				"original_id": map[string]any{
					"type":        "string",
					"description": "Original recording ID (log_diff_report)",
				},
				"replay_id": map[string]any{
					"type":        "string",
					"description": "Replay recording ID (log_diff_report)",
				},
				"format": map[string]any{
					"type":        "string",
					"description": "Screenshot format (screenshot)",
					"enum":        []string{"png", "jpeg"},
				},
				"quality": map[string]any{
					"type":        "number",
					"description": "Screenshot JPEG quality 1-100, default 80 (screenshot). Only applies when format is jpeg.",
				},
				"full_page": map[string]any{
					"type":        "boolean",
					"description": "Capture full scrollable page (screenshot)",
				},
				"selector": map[string]any{
					"type":        "string",
					"description": "Capture specific element by CSS selector (screenshot)",
				},
				"wait_for_stable": map[string]any{
					"type":        "boolean",
					"description": "Wait for layout to stabilize before capture (screenshot)",
				},
				"save_to": map[string]any{
					"type":        "string",
					"description": "File path to save screenshot to disk (screenshot)",
				},
				"min_group_size": map[string]any{
					"type":        "number",
					"description": "Minimum occurrences to form a group (summarized_logs, default 2)",
				},
				"classification": map[string]any{
					"type":        "string",
					"description": "Transient element classification filter (transients)",
					"enum":        []string{"alert", "toast", "snackbar", "notification", "tooltip", "banner", "flash"},
				},
				"summary": map[string]any{
					"type":        "boolean",
					"description": "Return compact summary instead of full entries (errors, logs, network_waterfall, network_bodies, websocket_events, websocket_status, actions, error_bundles, timeline, history, transients, storage)",
				},
				"visible_only": map[string]any{
					"type":        "boolean",
					"description": "Only return visible elements (page_inventory)",
				},
			},
			"required": []string{"what"},
		},
	}
}
