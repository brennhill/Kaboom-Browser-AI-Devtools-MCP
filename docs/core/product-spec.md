---
doc_type: product-spec
status: active
scope: core/mcp-contract
ai-priority: high
tags: [core, mcp, product-spec, canonical, target-behavior]
relates-to: [tech-spec.md, qa-spec.md, mcp-command-option-matrix.md, ../features/feature-index.md]
last-verified: 2026-02-17
canonical: true
---

# Core MCP Product Spec (TARGET)

## Purpose
Define the user-visible, canonical behavior of the Kaboom MCP surface. This spec is the product contract for all tool calls and resource discovery.

## Scope
- JSON-RPC transport at `POST /mcp`
- MCP methods: `initialize`, `tools/list`, `tools/call`, `resources/list`, `resources/read`, `resources/templates/list`
- Tool contracts for `observe`, `analyze`, `generate`, `configure`, `interact`
- Extension-backed command execution via server command queue

## Tool Contract (Current)

### `observe` (`what`)
`errors`, `logs`, `extension_logs`, `network_waterfall`, `network_bodies`, `websocket_events`, `websocket_status`, `actions`, `vitals`, `page`, `tabs`, `pilot`, `timeline`, `error_bundles`, `screenshot`, `command_result`, `pending_commands`, `failed_commands`, `saved_videos`, `recordings`, `recording_actions`, `playback_results`, `log_diff_report`

### `analyze` (`what`)
`dom`, `performance`, `accessibility`, `error_clusters`, `history`, `security_audit`, `third_party_audit`, `link_health`, `link_validation`, `page_summary`, `annotations`, `annotation_detail`, `api_validation`, `draw_history`, `draw_session`

### `generate` (`format`)
`reproduction`, `test`, `pr_summary`, `har`, `csp`, `sri`, `sarif`, `visual_test`, `annotation_report`, `annotation_issues`, `test_from_context`, `test_heal`, `test_classify`

### `configure` (`action`)
`store`, `load`, `noise_rule`, `clear`, `health`, `streaming`, `test_boundary_start`, `test_boundary_end`, `recording_start`, `recording_stop`, `playback`, `log_diff`, `telemetry`, `diff_sessions`, `audit_log`

### `interact` (`action`)
`highlight`, `subtitle`, `save_state`, `load_state`, `list_states`, `delete_state`, `execute_js`, `navigate`, `refresh`, `back`, `forward`, `new_tab`, `screenshot`, `click`, `type`, `select`, `check`, `get_text`, `get_value`, `get_attribute`, `set_attribute`, `focus`, `scroll_to`, `wait_for`, `key_press`, `paste`, `list_interactive`, `record_start`, `record_stop`, `upload`, `draw_mode_start`

## Product Guarantees
1. Schema-first contract: `tools/list` is the canonical enum/option contract for all MCP clients.
2. Unknown argument behavior: unknown tool args are ignored and surfaced as warnings, not hard failures.
3. Safety defaults:
- Tool call rate limit: 500 calls/minute.
- Response redaction is applied before responses leave the server.
4. Sync-by-default command behavior:
- `analyze` and `interact` are synchronous by default (`sync=true` behavior).
- `background=true` or explicit `sync=false`/`wait=false` returns queued/still-processing handles.
5. Async retrieval path: command completion is always available through `observe({what:"command_result", correlation_id})`.
6. Browser control dependency: `interact` and active browser-backed analyze modes require AI Web Pilot + extension connectivity.

## Compatibility Rules
- Canonical DOM query API is `analyze({what:"dom", selector:"..."})`.
- Legacy `analyze({what:"dom"})` documentation is non-canonical and must not be used for new integrations.
- Screenshot capture is owned exclusively by `observe({what:"screenshot"})`.

## Requirements
- `CORE_MCP_PROD_001`: Every tool call must route through `tools/call` and a declared schema.
- `CORE_MCP_PROD_002`: Every enum value in schema must have an executable server handler.
- `CORE_MCP_PROD_003`: Every extension-backed command must expose correlation-aware completion status.
- `CORE_MCP_PROD_004`: Sync/async behavior must be controllable via `sync`, `wait`, and `background` where supported.
- `CORE_MCP_PROD_005`: Observe pagination cursors (`after_cursor`, `before_cursor`, `since_cursor`) must be stable and documented.
- `CORE_MCP_PROD_006`: Tool responses must include structured errors for invalid params/modes/actions.
- `CORE_MCP_PROD_007`: Commands that cannot run due to extension state must return actionable diagnostics.
- `CORE_MCP_PROD_008`: Documentation must stay aligned with `internal/schema/schema.go`.

## Canonical References
- Schema: `internal/schema/schema.go`
- Dispatcher: `cmd/browser-agent/tools_core.go`
- Command/mode matrix: `docs/core/mcp-command-option-matrix.md`
