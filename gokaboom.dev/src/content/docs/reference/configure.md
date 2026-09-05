---
title: "Configure — Customize the Session"
description: "Complete reference for the configure tool. 30 modes for noise filtering, persistent storage, recording, playback, streaming, QA fixtures, session diffs, diagnostics, and more."
last_verified_version: 0.9.0
last_verified_date: 2026-08-03
normalized_tags: ['reference', 'configure']
---

The `configure` tool manages your KaBOOM session — filter noise, store data, manage recordings, compare error states, diff sessions, control streaming, and view audit logs.

Need one runnable call + response shape + failure fix for every mode? See [Configure Executable Examples](/reference/examples/configure-examples/).

## Quick Reference

```js
configure({what:"noise_rule", noise_action: "auto_detect"})       // Auto-filter noise
configure({what:"store", store_action: "save", key: "baseline", data: {...}})  // Save data
configure({what:"clear", buffer: "all"})                          // Clear all buffers
configure({what:"health"})                                        // Server health
configure({what:"event_recording_start"})                         // Start recording
configure({what:"event_recording_stop", recording_id: "rec-123"}) // Stop recording
configure({what:"playback", recording_id: "rec-123"})            // Replay recording
configure({what:"log_diff", original_id: "rec-1", replay_id: "rec-2"})  // Compare error states
configure({what:"diff_sessions", verif_session_action: "capture", name: "v1"})  // Session snapshot
configure({what:"audit_log", tool_name: "observe"})               // View tool usage history
configure({what:"qa_fixture", fixture_action: "validate", fixture: {version: 1}}) // Validate local QA state
```

## Common Parameters

These parameters are shared across many `configure` modes:

| Parameter | Type | Description |
|-----------|------|-------------|
| `what` | string (required) | Mode to execute (`noise_rule`, `store`, `diff_sessions`, etc.) |
| `telemetry_mode` | string | Telemetry metadata mode: `off`, `auto`, `full` |
| `tab_id` | number | Optional target tab ID for tab-aware modes |
| `operation` | string | Secondary operation selector used by mode-specific flows |
| `limit` | number | Max entries returned for list/report modes |

---

## noise_rule — Filter Irrelevant Errors

Manages noise rules that suppress irrelevant errors — browser extension noise, analytics failures, framework internals.

### Auto-detect noise

```js
configure({what:"noise_rule", noise_action: "auto_detect"})
```

Scans current errors and identifies patterns that are likely noise (extension errors, analytics, third-party scripts). Creates rules automatically.

### Add a manual rule

```js
configure({what:"noise_rule",
           noise_action: "add",
           pattern: "analytics\\.google",
           category: "console",
           reason: "Google Analytics noise"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `noise_action` | string | `add`, `remove`, `list`, `reset`, `auto_detect` |
| `pattern` | string | Regex pattern to match against error messages |
| `category` | string | Buffer: `console`, `network`, or `websocket` |
| `reason` | string | Human-readable explanation of why this is noise |
| `rules` | array | Batch-add multiple rules at once |
| `rule_id` | string | ID of rule to remove (for `remove` action) |

### List current rules

```js
configure({what:"noise_rule", noise_action: "list"})
```

### Remove a rule

```js
configure({what:"noise_rule", noise_action: "remove", rule_id: "rule-123"})
```

### Reset all rules

```js
configure({what:"noise_rule", noise_action: "reset"})
```

---

## store — Persistent Key-Value Storage

Save and load JSON data that persists across sessions. Useful for storing baselines, configuration, or any data the AI needs to remember.

### Save data

```js
configure({what:"store",
           store_action: "save",
           namespace: "baselines",
           key: "homepage-vitals",
           data: {lcp: 1200, cls: 0.05}})
```

### Load data

```js
configure({what:"store",
           store_action: "load",
           namespace: "baselines",
           key: "homepage-vitals"})
```

### List keys

```js
configure({what:"store", store_action: "list"})
configure({what:"store", store_action: "list", namespace: "baselines"})
```

### Delete a key

```js
configure({what:"store", store_action: "delete", key: "homepage-vitals"})
```

### Storage stats

```js
configure({what:"store", store_action: "stats"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `store_action` | string | `save`, `load`, `list`, `delete`, `stats` |
| `namespace` | string | Logical grouping for keys |
| `key` | string | Storage key |
| `data` | object | JSON data to persist (for `save`) |

## load — Load Session Context

Load the saved session context snapshot (project ID, baseline list, noise config, schema hints, and performance summary) from the server-side session store.

```js
configure({what: "load"})
```

---

## clear — Clear Buffers

Remove captured data from memory.

```js
configure({what:"clear", buffer: "all"})
configure({what:"clear", buffer: "network"})
configure({what:"clear", buffer: "logs"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `buffer` | string | `network`, `websocket`, `actions`, `logs`, `all` |

---

## event_recording_start — Start Recording

Start capturing a browser session. Records user actions and browser state for later playback or comparison.

```js
configure({what:"event_recording_start"})
configure({what:"event_recording_start", name: "checkout-run"})
```

---

## event_recording_stop — Stop Recording

Stop an active recording session.

```js
configure({what:"event_recording_stop", recording_id: "rec-123"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `recording_id` | string | ID of the recording to stop |

---

## playback — Replay Recording

Replay a previously captured recording.

```js
configure({what:"playback", recording_id: "rec-123"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `recording_id` | string | ID of the recording to replay |

---

## log_diff — Compare Error States

Compare error states between two recordings. Useful for verifying that a bug fix resolved the issue, or detecting regressions after a deploy.

```js
configure({what:"log_diff", original_id: "rec-abc", replay_id: "rec-xyz"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `original_id` | string | Original recording ID (baseline) |
| `replay_id` | string | Replay recording ID (comparison) |

---

## telemetry — Configure Telemetry Metadata

Set the global telemetry metadata mode. Individual tool calls can override this with the `telemetry_mode` parameter.

```js
configure({what:"telemetry", telemetry_mode: "auto"})
configure({what:"telemetry", telemetry_mode: "off"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `telemetry_mode` | string | `off`, `auto`, or `full` |

---

## streaming — Real-Time Event Streaming

Enable or disable real-time event notifications. When enabled, KaBOOM proactively notifies the AI about errors, performance regressions, and security issues as they happen.

```js
configure({what:"streaming", streaming_action: "enable",
           events: ["errors", "performance", "security"],
           severity_min: "warning",
           throttle_seconds: 5})

configure({what:"streaming", streaming_action: "disable"})
configure({what:"streaming", streaming_action: "status"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `streaming_action` | string | `enable`, `disable`, `status` |
| `events` | array | Categories: `errors`, `network_errors`, `performance`, `user_frustration`, `security`, `regression`, `anomaly`, `ci`, `all` |
| `severity_min` | string | Minimum severity: `info`, `warning`, `error` |
| `throttle_seconds` | integer | Minimum seconds between notifications (1-60) |

---

## health — Server Health

Check the KaBOOM server's status, uptime, buffer usage, and connected clients.

```js
configure({what:"health"})
```

No additional parameters. Returns server version, uptime, buffer occupancy, client count, and rate limit status.

---

## test_boundary_start / test_boundary_end — Test Boundaries

Mark the start and end of a test run. Events within boundaries can be correlated for test-specific analysis.

```js
configure({what:"test_boundary_start", test_id: "checkout-flow", label: "Guest Checkout Test"})

// ... run the test ...

configure({what:"test_boundary_end", test_id: "checkout-flow"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `test_id` | string | Unique identifier for the test |
| `label` | string | Human-readable description |

---

## diff_sessions — Session Snapshots & Comparison

Capture named snapshots of the current session state and compare them to detect changes.

### Capture a snapshot

```js
configure({what:"diff_sessions", verif_session_action: "capture", name: "before-deploy"})
```

### Compare two snapshots

```js
configure({what:"diff_sessions",
           verif_session_action: "compare",
           compare_a: "before-deploy",
           compare_b: "after-deploy"})
```

### List snapshots

```js
configure({what:"diff_sessions", verif_session_action: "list"})
```

### Delete a snapshot

```js
configure({what:"diff_sessions", verif_session_action: "delete", name: "old-snapshot"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `verif_session_action` | string | `capture`, `compare`, `list`, `delete` |
| `name` | string | Snapshot name |
| `compare_a` | string | First snapshot (baseline) |
| `compare_b` | string | Second snapshot (comparison target) |

---

## audit_log — MCP Tool Usage History

View a log of all MCP tool calls made during the session. Useful for debugging, compliance, and understanding AI agent behavior.

```js
configure({what:"audit_log"})
configure({what:"audit_log", tool_name: "observe", limit: 20})
configure({what:"audit_log", since: "2026-02-07T10:00:00Z"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `tool_name` | string | Filter by tool name |
| `audit_session_id` | string | Filter by audit session ID |
| `since` | string | Only entries after this ISO 8601 timestamp |
| `limit` | number | Maximum entries to return |

---

## describe_capabilities — Tool Capability Discovery

Returns a description of the tool's capabilities. Useful for AI agents to understand what actions are available.

```js
configure({what: "describe_capabilities"})
configure({what: "describe_capabilities", tool: "observe", mode: "errors"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `tool` | string | Filter to a specific tool (e.g., `observe`, `interact`) |
| `mode` | string | Filter to a specific mode within that tool |

---

## restart — Force Restart Daemon

Force-restart the KaBOOM daemon when it becomes unresponsive. Works even when the daemon is completely hung.

```js
configure({what: "restart"})
```

---

## doctor — Diagnostic Self-Check

Run a diagnostic health check. Verifies binary, port, extension connection, and client configuration.

```js
configure({what: "doctor"})
```

---

## security_mode — Debug Mode for Altered Environments

Opt into an altered-environment debug mode for advanced troubleshooting scenarios like proxied traffic inspection.

```js
configure({what: "security_mode"})                                    // Read current mode
configure({what: "security_mode", mode: "insecure_proxy", confirm: true})  // Enable
configure({what: "security_mode", mode: "normal"})                    // Disable
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `mode` | string | `normal` or `insecure_proxy` |
| `confirm` | boolean | Required `true` when enabling `insecure_proxy` |

---

## Macro Sequences

Save and replay named sequences of interact actions. Useful for repeatable workflows — demo scripts, test setup flows, or multi-step automation.

### save_sequence

```js
configure({what: "save_sequence",
           name: "login-flow",
           description: "Log in as test user",
           steps: [
             {what: "navigate", url: "https://example.com/login"},
             {what: "type", selector: "label=Email", text: "test@example.com"},
             {what: "click", selector: "text=Sign In"}
           ],
           tags: ["auth", "setup"]})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string (required) | Sequence name |
| `steps` | array (required) | Ordered list of interact actions |
| `description` | string | Human-readable description |
| `tags` | array | Labels for categorization |

### replay_sequence

```js
configure({what: "replay_sequence", name: "login-flow"})
configure({what: "replay_sequence", name: "login-flow", step_timeout_ms: 15000, stop_after_step: 2})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string (required) | Sequence name to replay |
| `override_steps` | array | Sparse array of step overrides (null = use saved) |
| `step_timeout_ms` | number | Timeout per step (default 10000) |
| `continue_on_error` | boolean | Continue if a step fails (default true) |
| `stop_after_step` | number | Stop after executing this many steps |

### get_sequence / list_sequences / delete_sequence

```js
configure({what: "get_sequence", name: "login-flow"})
configure({what: "list_sequences"})
configure({what: "delete_sequence", name: "login-flow"})
```

---

## tutorial — Quick Start Guidance

Return quickstart snippets and context-aware setup guidance.

```js
configure({what: "tutorial"})
configure({what: "tutorial"})
```

---

## network_recording — Network Traffic Recording

Manage network traffic recording for specific domains.

```js
configure({what: "network_recording", operation: "start", domain: "api.example.com"})
configure({what: "network_recording", operation: "status"})
configure({what: "network_recording", operation: "stop"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | string | `start`, `stop`, or `status` |
| `domain` | string | Domain to record traffic for |

---

## action_jitter — Random Action Delays

Configure random delays before interact actions. Useful for making automated flows look more natural or for stress-testing race conditions.

```js
configure({what: "action_jitter", action_jitter_ms: 500})
configure({what: "action_jitter", action_jitter_ms: 0})  // Disable
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `action_jitter_ms` | number | Maximum random delay in milliseconds (0 to disable) |

---

## report_issue — Submit a GitHub Issue

Create or preview a structured issue report directly from the running session. This bundles environment context, recent diagnostics, and your notes.

```js
configure({what: "report_issue", operation: "list_templates"})
configure({what: "report_issue", operation: "preview", template: "bug", user_context: "Daemon disconnected while replaying"})
configure({what: "report_issue", operation: "submit", template: "bug", title: "Replay disconnects intermittently", user_context: "Happens after ~20 actions"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | string | `list_templates`, `preview`, or `submit` |
| `template` | string | Issue template name |
| `title` | string | Issue title (required for `submit`) |
| `user_context` | string | Your repro notes/context attached to the report |

## setup_quality_gates — Code Quality Gate Scaffolding

Scaffold `.kaboom.json` and `kaboom-code-standards.md` into your project so agent code changes are checked by the KaBOOM `kaboom-hooks` quality gates — duplicate detection, blast-radius, decision-guard, session tracking, and output compression. Requires an active codebase (set `active_codebase` first).

```js
configure({what: "setup_quality_gates"})
configure({what: "setup_quality_gates", target_dir: "/path/within/project"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `target_dir` | string | Directory to write the config into. Must be within the project. Defaults to the active codebase root. |

---

## qa_fixture — Transactional Browser Test State

Validate, apply, inspect, or restore a versioned browser fixture. Applying a
fixture snapshots the current page state before mutation and returns an opaque
`transaction_id`; restore with that handle when the test finishes. Cookie and
storage values remain local to the extension and are never echoed in tool
responses or Doctor diagnostics.

```js
configure({what: "qa_fixture", fixture_action: "validate", fixture: {version: 1, viewport: {width: 1280, height: 720}}})
configure({what: "qa_fixture", fixture_action: "apply", fixture: {version: 1, user_state: "fresh", feature_flags: {new_checkout: true}}})
configure({what: "qa_fixture", fixture_action: "status"})
configure({what: "qa_fixture", fixture_action: "restore", transaction_id: "fixture_transaction_opaque"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `fixture_action` | string | `validate`, `apply`, `status`, or `restore` |
| `fixture` | object | Version-1 fixture for validate/apply; supports target URL, viewport, locale, permissions, network profile, cookies, browser storage, feature flags, seed data, user state, auth role, and a bounded setup timeout |
| `transaction_id` | string | Opaque handle returned by `apply`; required by `restore` |
