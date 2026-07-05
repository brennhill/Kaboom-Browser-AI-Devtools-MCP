---
doc_type: product-spec
feature_id: feature-observe
status: shipped
owners: []
last_reviewed: 2026-07-05
links:
  product: ./product-spec.md
  tech: ./tech-spec.md
  qa: ./qa-plan.md
  feature_index: ./index.md
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Observe Product Spec (TARGET)

## Purpose
Provide read-only access to captured runtime state, logs, network artifacts, action history, and command execution status.

## Modes (`what`)
`errors`, `logs`, `extension_logs`, `network_waterfall`, `network_bodies`, `websocket_events`, `websocket_status`, `actions`, `vitals`, `page`, `tabs`, `pilot`, `timeline`, `error_bundles`, `screenshot`, `command_result`, `pending_commands`, `failed_commands`, `saved_videos`, `recordings`, `recording_actions`, `playback_results`, `log_diff_report`

## User Outcomes
1. Read passive telemetry without mutating browser state.
2. Retrieve command completion details for `analyze`/`interact` correlation IDs.
3. Use cursor pagination for large buffers.

## Requirements
- `OBS_PROD_001`: `what` is required and validated against schema enum.
- `OBS_PROD_002`: cursor pagination options must work for log-like streams (`after_cursor`, `before_cursor`, `since_cursor`, `restart_on_eviction`).
- `OBS_PROD_003`: `command_result` must expose terminal and non-terminal states with correlation context.
- `OBS_PROD_004`: extension-disconnected states must return actionable diagnostics.
- `OBS_PROD_005`: `screenshot` mode must remain callable directly and via interact alias.

## Non-Goals
- No browser mutation.
- No execution of new page commands outside screenshot capture path.

## Related
- Core contract: `docs/core/product-spec.md`
- Command matrix: `docs/core/mcp-command-option-matrix.md`
