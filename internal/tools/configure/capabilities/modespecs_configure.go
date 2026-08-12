// modespecs_configure.go — configure tool per-mode parameter specs.
// Docs: docs/features/generation/describe_capabilities.md
package capabilities

var configureModeSpecs = map[string]modeParamSpec{
	"store": {
		Hint:     "Persist/retrieve session key-value data",
		Returns:  "The keys held in the project memory namespace.",
		Optional: []string{"store_action", "namespace", "key", "data"},
	},
	"load": {
		Hint:    "Load stored session data by namespace",
		Returns: "The stored project memory — baselines, error history, session count.",
	},
	"noise_rule": {
		Hint:    "Suppress recurring console noise with pattern rules",
		Returns: "The active noise-filter rules and how much each has suppressed.",
		Optional: []string{
			"noise_action", "rules", "rule_id", "pattern", "category", "classification",
			"message_regex", "source_regex", "url_regex", "method", "status_min", "status_max", "level", "reason",
		},
	},
	"clear": {
		Hint:     "Reset capture buffers (network, logs, actions, all)",
		Returns:  "Which buffers were cleared and how many entries went.",
		Optional: []string{"buffer"},
	},
	"health": {
		Hint:    "Check daemon + extension connection status",
		Returns: "The daemon's health across every subsystem, including whether the extension is connected.",
	},
	"tutorial": {
		Hint:    "Context-aware usage guidance and best practices",
		Returns: "Guidance text for a topic: next steps, snippets and recovery playbooks.",
	},
	"streaming": {
		Hint:     "Enable/disable push notifications for browser events. streaming_action: enable|disable|status (default: status)",
		Returns:  "The streaming configuration and how many notifications are pending.",
		Optional: []string{"streaming_action", "events", "throttle_seconds", "severity_min"},
	},
	"test_boundary_start": {
		Hint:     "Mark start of a test boundary for isolated captures",
		Returns:  "Confirmation that a test boundary opened, with its id.",
		Required: []string{"test_id"},
		Optional: []string{"label"},
	},
	"test_boundary_end": {
		Hint:     "Mark end of a test boundary",
		Returns:  "Confirmation that a test boundary closed.",
		Required: []string{"test_id"},
	},
	"event_recording_start": {
		Hint:     "Start recording browser session (actions + video)",
		Returns:  "The new recording's id — keep it; stopping and reading the recording both need it.",
		Optional: []string{"name", "url", "sensitive_data_enabled"},
	},
	"event_recording_stop": {
		Hint:     "Stop an active browser recording",
		Returns:  "Confirmation the recording stopped, with how many actions it captured.",
		Required: []string{"recording_id"},
	},
	"playback": {
		Hint:     "Replay a saved recording",
		Returns:  "The result of replaying a recording.",
		Required: []string{"recording_id"},
	},
	"log_diff": {
		Hint:     "Compare error logs between original and replay recordings",
		Returns:  "The console differences between an original run and a replay.",
		Required: []string{"original_id", "replay_id"},
	},
	"telemetry": {
		Hint:     "Set telemetry metadata mode (off/auto/full)",
		Returns:  "The current telemetry mode.",
		Optional: []string{"telemetry_mode"},
	},
	"describe_capabilities": {
		Hint:     "List modes and per-mode params; filter by tool and mode",
		Returns:  "What a tool or mode accepts and returns — required and optional params, and this line.",
		Optional: []string{"tool", "mode"},
	},
	"diff_sessions": {
		Hint:     "Compare two session snapshots to find state differences",
		Returns:  "What changed between two captured sessions.",
		Optional: []string{"verif_session_action", "name", "compare_a", "compare_b", "url", "performance_budgets"},
	},
	"audit_log": {
		Hint:     "View tool call audit trail with timing and results",
		Returns:  "A list of recorded tool invocations for compliance review.",
		Optional: []string{"operation", "audit_session_id", "tool_name", "since", "limit"},
	},
	"restart": {
		Hint:    "Force-restart daemon when unresponsive",
		Returns: "Confirmation the daemon is restarting.",
	},
	"save_sequence": {
		Hint:     "Save a named sequence of interact actions for replay",
		Returns:  "Confirmation the sequence was saved, with its step count.",
		Required: []string{"name", "steps"},
		Optional: []string{"description", "tags"},
	},
	"get_sequence": {
		Hint:     "Retrieve a saved action sequence by name",
		Returns:  "The steps of one saved sequence.",
		Required: []string{"name"},
	},
	"list_sequences": {
		Hint:    "List all saved action sequences",
		Returns: "A list of saved sequence NAMES. Names only — fetch one with get_sequence.",
	},
	"delete_sequence": {
		Hint:     "Delete a saved action sequence",
		Returns:  "Confirmation the sequence was deleted.",
		Required: []string{"name"},
	},
	"replay_sequence": {
		Hint:     "Replay a saved action sequence with optional overrides",
		Returns:  "The outcome of replaying a saved sequence.",
		Required: []string{"name"},
		Optional: []string{"override_steps", "step_timeout_ms", "continue_on_error", "stop_after_step"},
	},
	"doctor": {
		Hint:    "System diagnostics: port, state directory, log health",
		Returns: "Diagnostic checks with a ready-for-interaction verdict.",
	},
	"security_mode": {
		Hint:     "Toggle normal/insecure_proxy mode for debug environments",
		Returns:  "The active security mode and whether insecure rewrites are applied.",
		Optional: []string{"mode", "confirm"},
	},
	"network_recording": {
		Hint:     "Passive network traffic recording with start/stop capture",
		Returns:  "Whether network recording is currently on.",
		Optional: []string{"operation", "domain", "method"},
	},
	"action_jitter": {
		Hint:     "Randomized micro-delays before interact actions for human-like timing",
		Returns:  "The current action jitter delay in milliseconds.",
		Optional: []string{"action_jitter_ms"},
	},
	"report_issue": {
		Hint:     "Report an issue to the Kaboom team via GitHub",
		Returns:  "A formatted issue body ready to file. Text only — nothing is submitted for you.",
		Optional: []string{"operation", "template", "title", "user_context"},
	},
	"setup_quality_gates": {
		Hint:     "Scaffold .kaboom.json and code standards file for automated quality gate enforcement",
		Returns:  "Which quality-gate files were written into the target directory.",
		Optional: []string{"target_dir"},
	},
	"qa_fixture": {
		Hint:     "Validate a versioned, declarative browser QA environment before applying it",
		Returns:  "Whether the QA fixture is valid, and its version.",
		Required: []string{"fixture_action", "fixture"},
	},
}
