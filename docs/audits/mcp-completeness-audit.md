---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# MCP Completeness Audit

**Date:** 2026-02-14
**Auditor:** Claude Opus 4.6 (automated)
**Scope:** Schema/handler parity, stub detection, dead code, test coverage, guide accuracy

---

## 1. Schema <-> Handler Map Parity

### 1.1 observe tool

**Schema enum** (tools_schema.go line 18, 24 values):

| Schema Enum Value     | Handler Map Key       | Status   |
|-----------------------|-----------------------|----------|
| errors                | errors                | MATCH    |
| logs                  | logs                  | MATCH    |
| extension_logs        | extension_logs        | MATCH    |
| network_waterfall     | network_waterfall     | MATCH    |
| network_bodies        | network_bodies        | MATCH    |
| websocket_events      | websocket_events      | MATCH    |
| websocket_status      | websocket_status      | MATCH    |
| actions               | actions               | MATCH    |
| vitals                | vitals                | MATCH    |
| page                  | page                  | MATCH    |
| tabs                  | tabs                  | MATCH    |
| pilot                 | pilot                 | MATCH    |
| timeline              | timeline              | MATCH    |
| error_bundles         | error_bundles         | MATCH    |
| screenshot            | screenshot            | MATCH    |
| command_result        | command_result        | MATCH    |
| pending_commands      | pending_commands      | MATCH    |
| failed_commands       | failed_commands       | MATCH    |
| saved_videos          | saved_videos          | MATCH    |
| recordings            | recordings            | MATCH    |
| recording_actions     | recording_actions     | MATCH    |
| playback_results      | playback_results      | MATCH    |
| log_diff_report       | log_diff_report       | MATCH    |

**Handler map keys** (tools_observe.go lines 22-46, 23 entries): All 23 keys appear in the schema enum.

**Verdict: CLEAN** -- Full bidirectional parity. 23 schema values, 23 handler keys.

---

### 1.2 analyze tool

**Schema enum** (tools_schema.go line 107, 14 values):

| Schema Enum Value     | Handler Map Key       | Status   |
|-----------------------|-----------------------|----------|
| dom                   | dom                   | MATCH    |
| performance           | performance           | MATCH    |
| accessibility         | accessibility         | MATCH    |
| error_clusters        | error_clusters        | MATCH    |
| history               | history               | MATCH    |
| security_audit        | security_audit        | MATCH    |
| third_party_audit     | third_party_audit     | MATCH    |
| link_health           | link_health           | MATCH    |
| link_validation       | link_validation       | MATCH    |
| annotations           | annotations           | MATCH    |
| annotation_detail     | annotation_detail     | MATCH    |
| api_validation        | api_validation        | MATCH    |
| draw_history          | draw_history          | MATCH    |
| draw_session          | draw_session          | MATCH    |

**Handler map keys** (tools_analyze.go lines 32-81, 14 entries): All 14 keys appear in the schema enum.

**Verdict: CLEAN** -- Full bidirectional parity. 14 schema values, 14 handler keys.

---

### 1.3 generate tool

**Schema enum** (tools_schema.go line 198, 13 values):

| Schema Enum Value     | Handler Map Key       | Status   |
|-----------------------|-----------------------|----------|
| reproduction          | reproduction          | MATCH    |
| test                  | test                  | MATCH    |
| pr_summary            | pr_summary            | MATCH    |
| har                   | har                   | MATCH    |
| csp                   | csp                   | MATCH    |
| sri                   | sri                   | MATCH    |
| sarif                 | sarif                 | MATCH    |
| visual_test           | visual_test           | MATCH    |
| annotation_report     | annotation_report     | MATCH    |
| annotation_issues     | annotation_issues     | MATCH    |
| test_from_context     | test_from_context     | MATCH    |
| test_heal             | test_heal             | MATCH    |
| test_classify         | test_classify         | MATCH    |

**Handler map keys** (tools_generate.go lines 21-35, 13 entries): All 13 keys appear in the schema enum.

**Verdict: CLEAN** -- Full bidirectional parity. 13 schema values, 13 handler keys.

---

### 1.4 configure tool

**Schema enum** (tools_schema.go line 307, 14 values):

| Schema Enum Value       | Handler Map Key         | Status   |
|-------------------------|-------------------------|----------|
| store                   | store                   | MATCH    |
| load                    | load                    | MATCH    |
| noise_rule              | noise_rule              | MATCH    |
| clear                   | clear                   | MATCH    |
| health                  | health                  | MATCH    |
| streaming               | streaming               | MATCH    |
| test_boundary_start     | test_boundary_start     | MATCH    |
| test_boundary_end       | test_boundary_end       | MATCH    |
| recording_start         | recording_start         | MATCH    |
| recording_stop          | recording_stop          | MATCH    |
| playback                | playback                | MATCH    |
| log_diff                | log_diff                | MATCH    |
| diff_sessions           | diff_sessions           | MATCH    |
| audit_log               | audit_log               | MATCH    |

**Handler map keys** (tools_configure.go lines 33-48, 14 entries): All 14 keys appear in the schema enum.

**Verdict: CLEAN** -- Full bidirectional parity. 14 schema values, 14 handler keys.

---

### 1.5 interact tool

The interact tool uses a split dispatch: `interactDispatch()` returns a map for complex actions, `domPrimitiveActions` is a static map for DOM actions, and the schema has its own enum.

**Schema enum** (tools_schema.go lines 437-445, 28 values):

| Schema Enum Value   | interactDispatch() | domPrimitiveActions | Status   |
|---------------------|--------------------|---------------------|----------|
| highlight           | YES                | -                   | MATCH    |
| subtitle            | YES                | -                   | MATCH    |
| save_state          | YES                | -                   | MATCH    |
| load_state          | YES                | -                   | MATCH    |
| list_states         | YES                | -                   | MATCH    |
| delete_state        | YES                | -                   | MATCH    |
| execute_js          | YES                | -                   | MATCH    |
| navigate            | YES                | -                   | MATCH    |
| refresh             | YES                | -                   | MATCH    |
| back                | YES                | -                   | MATCH    |
| forward             | YES                | -                   | MATCH    |
| new_tab             | YES                | -                   | MATCH    |
| list_interactive    | YES                | -                   | MATCH    |
| record_start        | YES                | -                   | MATCH    |
| record_stop         | YES                | -                   | MATCH    |
| upload              | YES                | -                   | MATCH    |
| draw_mode_start     | YES                | -                   | MATCH    |
| click               | -                  | YES                 | MATCH    |
| type                | -                  | YES                 | MATCH    |
| select              | -                  | YES                 | MATCH    |
| check               | -                  | YES                 | MATCH    |
| get_text            | -                  | YES                 | MATCH    |
| get_value           | -                  | YES                 | MATCH    |
| get_attribute       | -                  | YES                 | MATCH    |
| set_attribute       | -                  | YES                 | MATCH    |
| focus               | -                  | YES                 | MATCH    |
| scroll_to           | -                  | YES                 | MATCH    |
| wait_for            | -                  | YES                 | MATCH    |
| key_press           | -                  | YES                 | MATCH    |

**interactDispatch() keys** (tools_interact.go lines 24-42, 17 entries): All 17 appear in the schema enum.

**domPrimitiveActions keys** (tools_interact.go lines 65-70, 12 entries): All 12 appear in the schema enum.

**Combined: 17 + 12 = 29 handler keys, 28 schema enum values.**

Wait -- recount. The schema enum has 28 values. The combined handler maps have 29 keys? Let me verify there is no overlap.

Checking for overlap between interactDispatch() and domPrimitiveActions: None found. The two maps are disjoint.

Re-counting the schema enum values:
highlight, subtitle, save_state, load_state, list_states, delete_state, execute_js, navigate, refresh, back, forward, new_tab, click, type, select, check, get_text, get_value, get_attribute, set_attribute, focus, scroll_to, wait_for, key_press, list_interactive, record_start, record_stop, upload, draw_mode_start = **29 values**.

**Verdict: CLEAN** -- Full bidirectional parity. 29 schema values = 17 dispatch + 12 DOM primitives = 29 handler keys.

---

## 2. Stub / "Not Implemented" Findings

### 2.1 Grep Results

Searched all Go source files in `cmd/browser-agent/` for:
- `"not_implemented"` -- **0 matches** in source (only in test assertions that verify absence)
- `"not implemented"` (case-insensitive) -- **0 matches** in source (only in test code that rejects them)
- `"stub"` -- **0 matches** in source
- `"placeholder"` -- **2 matches** in source comments (see below)

### 2.2 Thin / Hollow Handlers

The following handlers accept parameters but return hardcoded/empty data without performing real work:

| Handler | File:Line | Issue |
|---------|-----------|-------|
| `toolGetAuditLog` | tools_configure.go:421-439 | Parses `session_id`, `tool_name`, `limit`, `since` params but **ignores all of them**. Always returns `{"status":"ok","entries":[]}`. This is a hollow shell -- it advertises filtering but does nothing. |
| `toolDiffSessions` | tools_configure.go:403-419 | Parses `session_action` and `name` but **only echoes `session_action`** back. Returns `{"status":"ok","action":"..."}` regardless of input. Does not actually capture, compare, list, or delete snapshots. |
| `toolGetPlaybackResults` | recording_handlers.go:264-286 | Comment on line 278 literally says `"For now, return a placeholder"`. Always returns `{"results":[]}` with a message "Playback results would be stored here for later retrieval". |
| `toolConfigureTestBoundaryStart` | tools_configure.go:464-492 | Parses `test_id` and `label` but only returns the parsed values back. Does not actually record a boundary in any state store. |
| `toolConfigureTestBoundaryEnd` | tools_configure.go:494-516 | Returns `"was_active": true` unconditionally. Does not check if a boundary was actually started. |
| `toolValidateAPI` (analyze) | tools_analyze.go:161-200 | "analyze" and "report" operations return hardcoded empty `violations: []` and `endpoints: []`. No actual API schema validation occurs. |

**Severity assessment:**
- `toolGetPlaybackResults`: Explicitly labeled as placeholder in a code comment. **Should be flagged.**
- `toolGetAuditLog`: Silently ignores all filter params, returns empty data. **Hollow stub.**
- `toolDiffSessions`: Silently ignores session_action semantics. **Hollow stub.**
- `toolValidateAPI`: Returns empty violations/endpoints without checking anything. **Minimal scaffolding only.**
- `toolConfigureTestBoundary*`: Echo params back without persisting state. **Partial stubs.**

### 2.3 Placeholder Comments in Source

| File | Line | Comment |
|------|------|---------|
| tools_analyze.go | 203 | `// Link Health (Placeholder)` |
| tools_analyze.go | 207 | `// This is a placeholder that will be fully implemented in Phase 1.` |
| recording_handlers.go | 278 | `// For now, return a placeholder (would need to store playback sessions)` |

Note: `toolAnalyzeLinkHealth` itself is NOT a stub -- it correctly queues a pending query to the extension. The comment is stale/misleading.

---

## 3. Dead Code

### 3.1 Unreferenced Tool Functions

| Function | File:Line | Issue |
|----------|-----------|-------|
| `toolCheckPerformance` | tools_observe.go:516-522 | **NOT dead** -- Referenced by `analyzeHandlers["performance"]` in tools_analyze.go:43. However, it is defined in tools_observe.go despite being exclusively used by the analyze tool. This is a code organization issue, not dead code. |

No other unreferenced `tool*` or `handle*` functions found. All functions with `tool` or `handle` prefix in `cmd/browser-agent/` are referenced by at least one handler map or called by another function.

### 3.2 Stale Integration Test References

The integration test (`integration_test.go` lines 151-157) calls the following modes via the `observe` tool:

- `observe performance` (line 151)
- `observe accessibility` (line 152)
- `observe error_clusters` (line 154)
- `observe history` (line 155)
- `observe security_audit` (line 156)
- `observe third_party_audit` (line 157)

**These modes were moved to the `analyze` tool** and no longer exist in `observeHandlers`. These test cases will receive "Unknown observe mode" errors at runtime. The release gate test may be tolerant of this (it checks for "not implemented" specifically, not unknown modes), but these lines test nothing useful.

---

## 4. Test Coverage by Handler

### 4.1 observe handlers (23 modes)

| Mode | Has Dedicated Test? | Coverage Source |
|------|---------------------|-----------------|
| errors | YES | internal/tools/observe/logs/logs_edge_test.go |
| logs | YES | internal/tools/observe/logs/logs_edge_test.go |
| extension_logs | YES | internal/tools/observe/logs/logs_edge_test.go |
| network_waterfall | YES | internal/tools/observe/network/network_test.go |
| network_bodies | YES | internal/tools/observe/network/network_test.go |
| websocket_events | YES | internal/tools/observe/network/network_test.go |
| websocket_status | YES | internal/tools/observe/network/network_test.go |
| actions | YES | internal/tools/observe/session/session_transients_test.go |
| vitals | YES | internal/tools/observe/session/session_test.go |
| page | YES | internal/tools/observe/page/page_readiness_test.go |
| tabs | YES | internal/tools/observe/session/session_test.go |
| pilot | YES | cmd/browser-agent/internal/toolobserve/dispatcher_commands_test.go |
| timeline | YES | internal/tools/observe/timeline/correlation_test.go |
| error_bundles | YES | internal/tools/observe/timeline/correlation_test.go |
| screenshot | YES | internal/tools/observe/page/page_state_test.go |
| command_result | YES | cmd/browser-agent/internal/toolobserve/dispatcher_commands_test.go |
| pending_commands | YES | cmd/browser-agent/internal/toolobserve/dispatcher_commands_test.go |
| failed_commands | YES | cmd/browser-agent/internal/toolobserve/dispatcher_commands_test.go |
| saved_videos | YES | cmd/browser-agent/internal/screenrec/screenrec_test.go |
| recordings | YES | cmd/browser-agent/internal/toolrecording/handler_test.go |
| recording_actions | YES | cmd/browser-agent/internal/toolrecording/handler_test.go |
| playback_results | YES | cmd/browser-agent/internal/toolrecording/handler_test.go |
| log_diff_report | YES | cmd/browser-agent/internal/toolrecording/handler_test.go |

**Verdict:** All 23 observe modes have test coverage.

### 4.2 analyze handlers (14 modes)

| Mode | Has Dedicated Test? | Coverage Source |
|------|---------------------|-----------------|
| dom | YES | tools_analyze_handler_test.go |
| performance | YES | internal/tools/observe/session/session_test.go |
| accessibility | YES | internal/tools/observe/page/page_state_test.go |
| error_clusters | YES | internal/tools/observe/logs/logs_edge_test.go |
| history | YES | internal/tools/observe/session/session_test.go |
| security_audit | YES | tools_analyze_link_health_test.go |
| third_party_audit | YES | tools_analyze_link_health_test.go |
| link_health | YES | tools_analyze_link_health_test.go, tools_analyze_link_health_contract_test.go |
| link_validation | YES | tools_analyze_handler_test.go, tools_analyze_validation_test.go |
| annotations | YES | tools_analyze_annotations_test.go |
| annotation_detail | YES | tools_analyze_annotations_test.go |
| api_validation | YES | tools_analyze_handler_test.go, tools_analyze_link_health_contract_test.go |
| draw_history | YES | tools_analyze_annotations_test.go |
| draw_session | YES | tools_analyze_annotations_test.go |

**Verdict:** All 14 analyze modes have test coverage.

### 4.3 generate handlers (13 formats)

| Format | Has Dedicated Test? | Coverage Source |
|--------|---------------------|-----------------|
| reproduction | YES | reproduction_test.go |
| test | YES | testgen_unit_test.go, testgen_generate_test.go |
| pr_summary | YES | tools_generate_handler_test.go |
| sarif | YES | tools_generate_handler_test.go |
| har | YES | tools_generate_har_test.go |
| csp | YES | tools_generate_csp_test.go |
| sri | YES | tools_generate_handler_test.go |
| visual_test | YES | tools_generate_annotations_test.go |
| annotation_report | YES | tools_generate_annotations_test.go |
| annotation_issues | YES | tools_generate_annotations_test.go |
| test_from_context | YES | testgen_context_test.go, tools_generate_audit_test.go |
| test_heal | YES | testgen_heal_test.go, tools_generate_audit_test.go |
| test_classify | YES | testgen_classify_test.go, tools_generate_audit_test.go |

**Verdict:** All 13 generate formats have test coverage.

### 4.4 configure handlers (14 actions)

| Action | Has Dedicated Test? | Coverage Source |
|--------|---------------------|-----------------|
| store | YES | tools_configure_handler_test.go, tools_configure_session_test.go |
| load | YES | tools_configure_handler_test.go |
| noise_rule | YES | tools_configure_noise_test.go |
| clear | YES | tools_configure_coverage_test.go |
| health | YES | health_unit_test.go |
| streaming | YES | streaming_test.go, tools_contract_test.go |
| test_boundary_start | YES | tools_configure_handler_test.go, tools_configure_audit_test.go |
| test_boundary_end | YES | tools_configure_handler_test.go, tools_configure_session_test.go |
| recording_start | YES | tools_configure_audit_test.go, recording_handlers_test.go |
| recording_stop | YES | tools_configure_audit_test.go |
| playback | YES | tools_configure_audit_test.go |
| log_diff | YES | tools_configure_audit_test.go, recording_handlers_test.go |
| diff_sessions | YES | tools_configure_handler_test.go, tools_configure_audit_test.go |
| audit_log | YES | tools_configure_handler_test.go, tools_configure_audit_test.go |

**Verdict:** All 14 configure actions have test coverage.

### 4.5 interact handlers (29 actions)

| Action | Has Dedicated Test? | Coverage Source |
|--------|---------------------|-----------------|
| highlight | YES | cmd/browser-agent/internal/toolinteract/interact_browser_test.go |
| subtitle | YES | cmd/browser-agent/internal/toolinteract/interact_browser_test.go |
| save_state | YES | cmd/browser-agent/internal/toolinteract/interactstate/state_test.go, tools_interact_state_test.go |
| load_state | YES | tools_interact_state_test.go |
| list_states | YES | cmd/browser-agent/internal/toolinteract/interactstate/state_test.go, integration_test.go |
| delete_state | YES | cmd/browser-agent/internal/toolinteract/interactstate/state_test.go |
| execute_js | YES | cmd/browser-agent/internal/toolinteract/interact_browser_test.go |
| navigate | YES | tools_interact_nav_test.go, handler_consistency_test.go |
| refresh | YES | tools_interact_nav_test.go, handler_consistency_test.go |
| back | YES | tools_interact_nav_test.go, handler_consistency_test.go |
| forward | YES | tools_interact_nav_test.go, handler_consistency_test.go |
| new_tab | YES | tools_interact_nav_test.go, handler_consistency_test.go |
| click | YES | tools_interact_dom_test.go, handler_consistency_test.go |
| type | YES | tools_interact_dom_test.go, handler_consistency_test.go |
| select | YES | tools_interact_dom_test.go |
| check | YES | tools_interact_dom_test.go |
| get_text | YES | tools_interact_dom_test.go, handler_consistency_test.go |
| get_value | YES | tools_interact_dom_test.go |
| get_attribute | YES | tools_interact_dom_test.go |
| set_attribute | YES | tools_interact_dom_test.go |
| focus | YES | tools_interact_dom_test.go, handler_consistency_test.go |
| scroll_to | YES | tools_interact_dom_test.go, handler_consistency_test.go |
| wait_for | YES | tools_interact_dom_test.go |
| key_press | YES | tools_interact_dom_test.go |
| list_interactive | YES | cmd/browser-agent/internal/toolinteract/interact_browser_test.go |
| record_start | YES | tools_recording_video_test.go |
| record_stop | YES | tools_recording_video_test.go |
| upload | YES | tools_interact_upload_test.go, upload_integration_test.go |
| draw_mode_start | YES | cmd/browser-agent/internal/toolinteract/interact_browser_test.go |

**Verdict:** All 29 interact actions have test coverage.

---

## 5. Guide Accuracy

The guide is defined in `handler.go` lines 332-397 as the `kaboom://guide` resource.

### 5.1 observe modes listed in guide vs schema

**Guide lists** (line 340):
`errors, logs, extension_logs, network_waterfall, network_bodies, websocket_events, websocket_status, actions, vitals, page, tabs, pilot, timeline, error_bundles, screenshot, command_result, pending_commands, failed_commands, saved_videos, recordings, recording_actions, playback_results, log_diff_report`

**Schema enum**: Same 23 values.

**Verdict: MATCH** -- All 23 observe modes in guide match the schema exactly.

### 5.2 analyze modes listed in guide vs schema

**Guide lists** (line 341):
`dom, accessibility, performance, security_audit, third_party_audit, link_health, link_validation, error_clusters, history, api_validation, annotations, annotation_detail, draw_history, draw_session`

**Schema enum**: Same 14 values.

**Verdict: MATCH** -- All 14 analyze modes in guide match the schema exactly.

### 5.3 generate formats listed in guide vs schema

**Guide lists** (line 342):
`test, reproduction, pr_summary, sarif, har, csp, sri, visual_test, annotation_report, annotation_issues, test_from_context, test_heal, test_classify`

**Schema enum**: Same 13 values.

**Verdict: MATCH** -- All 13 generate formats in guide match the schema exactly.

### 5.4 configure actions listed in guide vs schema

**Guide lists** (line 343):
`health, store, load, noise_rule, clear, diff_sessions, audit_log, streaming, test_boundary_start, test_boundary_end, recording_start, recording_stop, playback, log_diff`

**Schema enum**: Same 14 values.

**Verdict: MATCH** -- All 14 configure actions in guide match the schema exactly.

### 5.5 interact actions listed in guide vs schema

**Guide lists** (line 344):
`click, type, select, check, navigate, refresh, execute_js, highlight, subtitle, key_press, scroll_to, wait_for, get_text, get_value, get_attribute, set_attribute, focus, list_interactive, save_state, load_state, list_states, delete_state, record_start, record_stop, upload, draw_mode_start, back, forward, new_tab`

**Schema enum**: Same 29 values.

**Verdict: MATCH** -- All 29 interact actions in guide match the schema exactly.

---

## 6. Overall Status

### CLEAN with CAVEATS

The schema-handler parity audit, guide accuracy check, and test coverage checks all pass cleanly. No "not_implemented" strings remain in production source code. All schema enum values have corresponding handlers, and vice versa.

### Items Requiring Attention

| # | Severity | Item | Location |
|---|----------|------|----------|
| 1 | **MEDIUM** | `toolGetAuditLog` is a hollow stub: parses params, ignores all, returns empty `entries: []` | tools_configure.go:421-439 |
| 2 | **MEDIUM** | `toolDiffSessions` is a hollow stub: parses params, echoes action, does no real work | tools_configure.go:403-419 |
| 3 | **MEDIUM** | `toolGetPlaybackResults` has explicit "placeholder" comment and returns empty results | recording_handlers.go:264-286 |
| 4 | **LOW** | `toolConfigureTestBoundaryStart/End` echo params without persisting state; `was_active: true` is hardcoded | tools_configure.go:464-516 |
| 5 | **LOW** | `toolValidateAPI` operations return empty violations/endpoints without performing real validation | tools_analyze.go:161-200 |
| 6 | **LOW** | Stale "Placeholder" comment on `toolAnalyzeLinkHealth` (function is actually implemented) | tools_analyze.go:203-207 |
| 7 | **LOW** | `toolCheckPerformance` is defined in tools_observe.go but only used by the analyze tool | tools_observe.go:516-522 |
| 8 | **INFO** | Integration test (lines 151-157) calls 6 modes via `observe` that now belong to `analyze` -- these test cases are silently broken (they get "Unknown observe mode" errors, but the test may pass since it checks for "not implemented" specifically) | integration_test.go:151-157 |

### Summary

- **Schema <-> Handler parity**: CLEAN across all 5 tools (93 total enum values = 93 handler keys)
- **"Not implemented" stubs**: NONE remaining in production code
- **Hollow stubs**: 3 handlers (audit_log, diff_sessions, playback_results) return hardcoded empty data
- **Dead code**: No unreferenced tool/handle functions found
- **Test coverage**: All 93 handler modes have at least one test
- **Guide accuracy**: Perfect alignment between guide, schema, and handler maps
