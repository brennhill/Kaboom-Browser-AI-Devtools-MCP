// modespecs_observe.go — observe tool per-mode parameter specs.
// Docs: docs/features/generation/describe_capabilities.md
package capabilities

var observeModeSpecs = map[string]modeParamSpec{
	"errors": {
		Hint:     "Raw JavaScript console errors. summary=true returns counts by source + top messages",
		Returns:  "A list of captured JavaScript errors with their stacks.",
		Optional: []string{"scope", "limit", "summary"},
	},
	"logs": {
		Hint:     "Console log messages with level/source filtering. summary=true returns counts by level/source",
		Returns:  "A list of console log entries. Noise-filtered — dev-server chatter matching the builtin rules never appears.",
		Optional: []string{"min_level", "source", "include_internal", "include_extension_logs", "extension_limit", "limit", "scope", "summary"},
	},
	"extension_logs": {
		Hint:     "Kaboom extension internal debug logs",
		Returns:  "A list of the extension's own internal debug lines. Diagnostics, not page console output.",
		Optional: []string{"limit"},
	},
	"network_waterfall": {
		Hint:     "HTTP request/response timeline with status and timing. summary=true returns compact {url,ms,type} entries",
		Returns:  "A list of resource requests with timing and status. Timings only — no response bodies.",
		Optional: []string{"url", "method", "status_min", "status_max", "limit", "summary", "after_cursor", "before_cursor", "since_cursor", "restart_on_eviction"},
	},
	"network_bodies": {
		Hint:     "HTTP response bodies with JSON path extraction. summary=true returns status groups + top URLs",
		Returns:  "A list of captured fetch response bodies. Large bodies are truncated with a flag.",
		Optional: []string{"url", "body_path", "method", "status_min", "status_max", "limit", "after_cursor", "before_cursor", "since_cursor", "restart_on_eviction", "summary"},
	},
	"websocket_events": {
		Hint:     "WebSocket message frames (incoming/outgoing). summary=true returns direction/event counts",
		Returns:  "A list of WebSocket frames that were sent and received.",
		Optional: []string{"connection_id", "direction", "limit", "after_cursor", "before_cursor", "since_cursor", "restart_on_eviction", "summary"},
	},
	"websocket_status": {
		Hint:     "Active WebSocket connection states",
		Returns:  "A summary of open and closed WebSocket connections.",
		Optional: []string{"url", "connection_id", "summary"},
	},
	"actions": {
		Hint:     "User interaction log (clicks, inputs, navigation). summary=true returns counts by type + time range",
		Returns:  "A list of user actions the extension recorded on the page.",
		Optional: []string{"limit", "after_cursor", "before_cursor", "since_cursor", "last_n", "restart_on_eviction", "summary"},
	},
	"vitals": {
		Hint:     "Core Web Vitals (LCP, CLS, INP, FCP, TTFB)",
		Returns:  "The page's Core Web Vitals measurements.",
		Optional: []string{"limit"},
	},
	"page": {
		Hint:    "Current page URL, title, and tracked tab info (metadata only; for content use analyze/page_summary or interact/explore_page)",
		Returns: "The tracked tab's current identity and readiness — url, title, whether commands can run.",
	},
	"tabs": {
		Hint:    "All open browser tabs with URLs",
		Returns: "A list of open browser tabs and which one is tracked.",
	},
	"history": {
		Hint:     "Recent page navigation history. summary=true returns counts only (chronological list; for pattern analysis use analyze/navigation_patterns)",
		Returns:  "A list of pages visited in this session.",
		Optional: []string{"limit", "summary"},
	},
	"pilot": {
		Hint:    "AI Web Pilot connection status and availability",
		Returns: "Whether the extension is connected and accepting commands.",
	},
	"timeline": {
		Hint:     "Merged chronological view of actions, errors, network, and WebSocket events. summary=true returns counts by type",
		Returns:  "A merged, time-ordered list of logs, network calls and actions.",
		Optional: []string{"include", "limit", "summary"},
	},
	"error_bundles": {
		Hint:     "Pre-assembled debug context per error (error + network + actions + logs in time window). summary=true returns bundle counts + unique messages",
		Returns:  "Pre-assembled debug bundles, one per error, each with its surrounding context.",
		Optional: []string{"window_seconds", "limit", "scope", "summary"},
	},
	"screenshot": {
		Hint:     "Capture page screenshot (full page or element)",
		Returns:  "A FILE PATH to the saved image on disk, not the image bytes.",
		Optional: []string{"format", "quality", "full_page", "selector", "wait_for_stable", "save_to"},
	},
	"storage": {
		Hint:     "localStorage, sessionStorage, and cookies (with full metadata including httpOnly)",
		Returns:  "The page's localStorage, sessionStorage or cookie entries.",
		Optional: []string{"storage_type", "key", "summary"},
	},
	"indexeddb": {
		Hint:     "IndexedDB database/store contents",
		Returns:  "Records read out of one IndexedDB object store.",
		Optional: []string{"database", "store"},
	},
	"command_result": {
		Hint:     "Poll result of an async command. Requires correlation_id from the original call response",
		Returns:  "The outcome of one previously queued command, looked up by correlation id.",
		Required: []string{"correlation_id"},
	},
	"pending_commands": {
		Hint:    "List in-flight async commands awaiting results",
		Returns: "The queued, completed and failed command lists, each capped at the 50 most recent with true totals alongside.",
	},
	"failed_commands": {
		Hint:    "List recently failed or expired async commands",
		Returns: "A list of commands that failed or expired.",
	},
	"saved_videos": {
		Hint:    "List saved SCREEN-CAPTURE VIDEO files (webm) with their paths and sizes. Not action recordings — see 'recordings' for those.",
		Returns: "A list of saved screen-capture VIDEO files with their paths and sizes. Paths only — the video bytes stay on disk.",
	},
	"recordings": {
		Hint:     "List ACTION recordings (captured user-action sequences for playback and test generation): id, name, created_at, duration, action_count, start_url. Entries omit the actions themselves — use 'recording_actions' with an id for those. Not video files; see 'saved_videos'.",
		Returns:  "A list of ACTION-recording summaries: id, name, when, how long, how many actions. Metadata only — not the actions (use recording_actions) and not video (use saved_videos).",
		Optional: []string{"limit"},
	},
	"recording_actions": {
		Hint:     "Action log from a specific recording",
		Returns:  "The ordered actions inside ONE recording, with selectors and timings.",
		Required: []string{"recording_id"},
		Optional: []string{"limit"},
	},
	"playback_results": {
		Hint:     "Results from replaying a recording",
		Returns:  "The pass/fail outcome of a replayed recording, step by step.",
		Required: []string{"recording_id"},
		Optional: []string{"limit"},
	},
	"log_diff_report": {
		Hint:     "Compare error logs between original and replay to find regressions",
		Returns:  "The differences in console output between an original run and a replay.",
		Required: []string{"original_id", "replay_id"},
	},
	"summarized_logs": {
		Hint:     "Console messages grouped by fingerprint for pattern detection",
		Returns:  "Log entries grouped into patterns, with anomalies called out, instead of the raw stream.",
		Optional: []string{"min_level", "source", "limit", "min_group_size"},
	},
	"page_inventory": {
		Hint:     "Combined page info + interactive elements in one call. For a richer snapshot (readable text, navigation links, screenshot), use interact(what='explore_page') instead.",
		Returns:  "The page identity plus a list of interactive elements. Lean by default; pass verbose=true for geometry and landmark context.",
		Optional: []string{"visible_only", "limit"},
	},
	"transients": {
		Hint:     "Captured transient UI elements (toasts, alerts, snackbars)",
		Returns:  "A list of short-lived UI messages — toasts, banners, alerts — that appeared and vanished.",
		Optional: []string{"limit", "classification", "url", "summary"},
	},
	"inbox": {
		Hint:    "Drain pending push events queued for MCP clients",
		Returns: "A list of push events the daemon received.",
	},
	"site_menus": {
		Hint:     "Discover page menus using 3-layer heuristic: semantic landmarks, axis alignment, border proximity. Returns {main, sidebar, footer, other, ungrouped}",
		Returns:  "The page's navigation menus grouped by region: main, sidebar, footer.",
		Optional: []string{"summary"},
	},
}
