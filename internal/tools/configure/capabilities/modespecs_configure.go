// modespecs_configure.go — configure tool per-mode parameter specs.
// Docs: docs/features/describe_capabilities.md
package capabilities

var configureModeSpecs = map[string]modeParamSpec{
	"store": {
		Hint:     "Persist/retrieve session key-value data",
		Optional: []string{"store_action", "namespace", "key", "data"},
	},
	"load": {
		Hint: "Load stored session data by namespace",
	},
	"noise_rule": {
		Hint: "Suppress recurring console noise with pattern rules",
		Optional: []string{
			"noise_action", "rules", "rule_id", "pattern", "category", "classification",
			"message_regex", "source_regex", "url_regex", "method", "status_min", "status_max", "level", "reason",
		},
	},
	"clear": {
		Hint:     "Reset capture buffers (network, logs, actions, all)",
		Optional: []string{"buffer"},
	},
	"health": {
		Hint: "Check daemon + extension connection status",
	},
	"tutorial": {
		Hint: "Context-aware usage guidance and best practices",
	},
	"streaming": {
		Hint:     "Enable/disable push notifications for browser events. streaming_action: enable|disable|status (default: status)",
		Optional: []string{"streaming_action", "events", "throttle_seconds", "severity_min"},
	},
	"test_boundary_start": {
		Hint:     "Mark start of a test boundary for isolated captures",
		Required: []string{"test_id"},
		Optional: []string{"label"},
	},
	"test_boundary_end": {
		Hint:     "Mark end of a test boundary",
		Required: []string{"test_id"},
	},
	"event_recording_start": {
		Hint:     "Start recording browser session (actions + video)",
		Optional: []string{"name", "url", "sensitive_data_enabled"},
	},
	"event_recording_stop": {
		Hint:     "Stop an active browser recording",
		Required: []string{"recording_id"},
	},
	"playback": {
		Hint:     "Replay a saved recording",
		Required: []string{"recording_id"},
	},
	"log_diff": {
		Hint:     "Compare error logs between original and replay recordings",
		Required: []string{"original_id", "replay_id"},
	},
	"telemetry": {
		Hint:     "Set telemetry metadata mode (off/auto/full)",
		Optional: []string{"telemetry_mode"},
	},
	"describe_capabilities": {
		Hint:     "List modes and per-mode params; filter by tool and mode",
		Optional: []string{"tool", "mode"},
	},
	"diff_sessions": {
		Hint:     "Compare two session snapshots to find state differences",
		Optional: []string{"verif_session_action", "name", "compare_a", "compare_b", "url"},
	},
	"audit_log": {
		Hint:     "View tool call audit trail with timing and results",
		Optional: []string{"operation", "audit_session_id", "tool_name", "since", "limit"},
	},
	"restart": {
		Hint: "Force-restart daemon when unresponsive",
	},
	"save_sequence": {
		Hint:     "Save a named sequence of interact actions for replay",
		Required: []string{"name", "steps"},
		Optional: []string{"description", "tags"},
	},
	"get_sequence": {
		Hint:     "Retrieve a saved action sequence by name",
		Required: []string{"name"},
	},
	"list_sequences": {
		Hint: "List all saved action sequences",
	},
	"delete_sequence": {
		Hint:     "Delete a saved action sequence",
		Required: []string{"name"},
	},
	"replay_sequence": {
		Hint:     "Replay a saved action sequence with optional overrides",
		Required: []string{"name"},
		Optional: []string{"override_steps", "step_timeout_ms", "continue_on_error", "stop_after_step"},
	},
	"doctor": {
		Hint: "System diagnostics: port, state directory, log health",
	},
	"security_mode": {
		Hint:     "Toggle normal/insecure_proxy mode for debug environments",
		Optional: []string{"mode", "confirm"},
	},
	"network_recording": {
		Hint:     "Passive network traffic recording with start/stop capture",
		Optional: []string{"operation", "domain", "method"},
	},
	"action_jitter": {
		Hint:     "Randomized micro-delays before interact actions for human-like timing",
		Optional: []string{"action_jitter_ms"},
	},
	"report_issue": {
		Hint:     "Report an issue to the Kaboom team via GitHub",
		Optional: []string{"operation", "template", "title", "user_context"},
	},
	"setup_quality_gates": {
		Hint:     "Scaffold .kaboom.json and code standards file for automated quality gate enforcement",
		Optional: []string{"target_dir"},
	},
	"qa_fixture": {
		Hint:     "Validate a versioned, declarative browser QA environment before applying it",
		Required: []string{"fixture_action", "fixture"},
	},
}
