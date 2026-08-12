// modespecs_observe.go — observe tool per-mode parameter specs.
// Docs: docs/features/generation/describe_capabilities.md
package capabilities

var observeModeSpecs = map[string]modeParamSpec{
	"errors": {
		Hint:     "Raw JavaScript console errors. summary=true returns counts by source + top messages",
		Returns:  "errors[]: message, stack, source, timestamp for uncaught errors and console.error entries.",
		Optional: []string{"scope", "limit", "summary"},
	},
	"logs": {
		Hint:     "Console log messages with level/source filtering. summary=true returns counts by level/source",
		Returns:  "logs[]: level, message, source, timestamp. Noise-filtered — dev-server chatter matching the builtin rules is suppressed and will not appear.",
		Optional: []string{"min_level", "source", "include_internal", "include_extension_logs", "extension_limit", "limit", "scope", "summary"},
	},
	"extension_logs": {
		Hint:     "Kaboom extension internal debug logs",
		Optional: []string{"limit"},
	},
	"network_waterfall": {
		Hint:     "HTTP request/response timeline with status and timing. summary=true returns compact {url,ms,type} entries",
		Returns:  "entries[]: one per resource request with url, status, timing breakdown (dns/connect/ttfb/download), sizes and initiator. Deduplicated per page load.",
		Optional: []string{"url", "method", "status_min", "status_max", "limit", "summary", "after_cursor", "before_cursor", "since_cursor", "restart_on_eviction"},
	},
	"network_bodies": {
		Hint:     "HTTP response bodies with JSON path extraction. summary=true returns status groups + top URLs",
		Optional: []string{"url", "body_path", "method", "status_min", "status_max", "limit", "after_cursor", "before_cursor", "since_cursor", "restart_on_eviction", "summary"},
	},
	"websocket_events": {
		Hint:     "WebSocket message frames (incoming/outgoing). summary=true returns direction/event counts",
		Optional: []string{"connection_id", "direction", "limit", "after_cursor", "before_cursor", "since_cursor", "restart_on_eviction", "summary"},
	},
	"websocket_status": {
		Hint:     "Active WebSocket connection states",
		Optional: []string{"url", "connection_id", "summary"},
	},
	"actions": {
		Hint:     "User interaction log (clicks, inputs, navigation). summary=true returns counts by type + time range",
		Optional: []string{"limit", "after_cursor", "before_cursor", "since_cursor", "last_n", "restart_on_eviction", "summary"},
	},
	"vitals": {
		Hint:     "Core Web Vitals (LCP, CLS, INP, FCP, TTFB)",
		Optional: []string{"limit"},
	},
	"page": {
		Hint: "Current page URL, title, and tracked tab info (metadata only; for content use analyze/page_summary or interact/explore_page)",
	},
	"tabs": {
		Hint: "All open browser tabs with URLs",
	},
	"history": {
		Hint:     "Recent page navigation history. summary=true returns counts only (chronological list; for pattern analysis use analyze/navigation_patterns)",
		Optional: []string{"limit", "summary"},
	},
	"pilot": {
		Hint: "AI Web Pilot connection status and availability",
	},
	"timeline": {
		Hint:     "Merged chronological view of actions, errors, network, and WebSocket events. summary=true returns counts by type",
		Optional: []string{"include", "limit", "summary"},
	},
	"error_bundles": {
		Hint:     "Pre-assembled debug context per error (error + network + actions + logs in time window). summary=true returns bundle counts + unique messages",
		Optional: []string{"window_seconds", "limit", "scope", "summary"},
	},
	"screenshot": {
		Hint:     "Capture page screenshot (full page or element)",
		Optional: []string{"format", "quality", "full_page", "selector", "wait_for_stable", "save_to"},
	},
	"storage": {
		Hint:     "localStorage, sessionStorage, and cookies (with full metadata including httpOnly)",
		Optional: []string{"storage_type", "key", "summary"},
	},
	"indexeddb": {
		Hint:     "IndexedDB database/store contents",
		Optional: []string{"database", "store"},
	},
	"command_result": {
		Hint:     "Poll result of an async command. Requires correlation_id from the original call response",
		Required: []string{"correlation_id"},
	},
	"pending_commands": {
		Hint:    "List in-flight async commands awaiting results",
		Returns: "pending[], completed[], failed[] (each capped at the 50 most recent, with pending_total/completed_total/failed_total for the true counts and truncated when any were withheld), plus extension_in_progress[] and its count.",
	},
	"failed_commands": {
		Hint: "List recently failed or expired async commands",
	},
	"saved_videos": {
		Hint:    "List saved SCREEN-CAPTURE VIDEO files (webm) with their paths and sizes. Not action recordings — see 'recordings' for those.",
		Returns: "videos[]: file path, size and creation time for each saved screen-capture file. No action data.",
	},
	"recordings": {
		Hint:     "List ACTION recordings (captured user-action sequences for playback and test generation): id, name, created_at, duration, action_count, start_url. Entries omit the actions themselves — use 'recording_actions' with an id for those. Not video files; see 'saved_videos'.",
		Returns:  "recordings[]: id, name, created_at, start_url, duration_ms, action_count, viewport. Entries OMIT the actions themselves — call recording_actions with an id for those. Also active_recording_id (empty when nothing is recording), count, limit.",
		Optional: []string{"limit"},
	},
	"recording_actions": {
		Hint:     "Action log from a specific recording",
		Returns:  "The ordered actions of ONE recording: type, selector, value and timing per action, plus the recording metadata.",
		Required: []string{"recording_id"},
		Optional: []string{"limit"},
	},
	"playback_results": {
		Hint:     "Results from replaying a recording",
		Required: []string{"recording_id"},
		Optional: []string{"limit"},
	},
	"log_diff_report": {
		Hint:     "Compare error logs between original and replay to find regressions",
		Required: []string{"original_id", "replay_id"},
	},
	"summarized_logs": {
		Hint:     "Console messages grouped by fingerprint for pattern detection",
		Optional: []string{"min_level", "source", "limit", "min_group_size"},
	},
	"page_inventory": {
		Hint:     "Combined page info + interactive elements in one call. For a richer snapshot (readable text, navigation links, screenshot), use interact(what='explore_page') instead.",
		Returns:  "Page identity (url, title, favicon, tab_status) plus interactive_elements[]: element_id, element_type, label, selector, index. Pass verbose=true for bbox, tag, landmark and overlay context.",
		Optional: []string{"visible_only", "limit"},
	},
	"transients": {
		Hint:     "Captured transient UI elements (toasts, alerts, snackbars)",
		Optional: []string{"limit", "classification", "url", "summary"},
	},
	"inbox": {
		Hint: "Drain pending push events queued for MCP clients",
	},
	"site_menus": {
		Hint:     "Discover page menus using 3-layer heuristic: semantic landmarks, axis alignment, border proximity. Returns {main, sidebar, footer, other, ungrouped}",
		Optional: []string{"summary"},
	},
}
