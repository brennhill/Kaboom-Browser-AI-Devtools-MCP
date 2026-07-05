---
doc_type: tech-spec
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

# Observe Tech Spec (TARGET)

## Dispatcher
- Entry: `toolObserve` in `cmd/browser-agent/tools_observe.go`
- Dispatch map: `observeHandlers` in `cmd/browser-agent/tools_observe_registry.go`, keyed by `what`
- Response augmentation helpers: `cmd/browser-agent/tools_observe_response.go`

## Data Sources
1. Server log buffers (`errors`, `logs`, `timeline`, `error_bundles`)
2. Capture network/websocket/action buffers (`network_*`, `websocket_*`, `actions`)
3. Extension status and tracking state (`pilot`, `page`, `tabs`)
4. Command tracker (`command_result`, `pending_commands`, `failed_commands`)
5. Recording/video stores (`saved_videos`, `recordings`, `recording_actions`, `playback_results`, `log_diff_report`)

## Async Command Observation Path
- Command IDs are registered when active commands are queued.
- `toolObserveCommandResult` reads from command tracker.
- Annotation wait path supports blocking retrieval for draw mode correlation IDs (`ann_*`).

## Pagination and Filtering
- Cursor pagination implemented for logs via `internal/pagination`.
- Filter keys (`url`, `method`, `status_min`, `status_max`, `level`, `min_level`, etc.) are mode-specific.

## Response Metadata Contract
- Buffer-backed observe modes include `metadata` with:
  - `retrieved_at`, `is_stale`, `data_age`, `data_age_ms`
- `data_age_ms` is a machine-readable freshness field:
  - non-negative when data exists
  - `-1` sentinel when no data exists
- `observe({what:"page"})` includes readiness fields:
  - `tab_status` (tab lifecycle status from extension sync)
  - `page_ready_for_commands` (ready gate used by interact/automation flows)

## Error and Resilience Behavior
- Missing/unknown `what` yields structured errors.
- Disconnect warning is prepended for extension-dependent modes.
- Stale/expired command results are surfaced with explicit retry guidance.
- `screenshot` with `full_page:true` expands nested scroll containers in the page context before CDP capture, then always restores original container styles in a `finally` path.
- Full-page screenshot dimensions are derived from CDP layout metrics plus an expansion height hint, then clamped to Chrome texture limits.
- If expansion script execution fails, screenshot capture degrades to CDP layout-only sizing; if CDP attach/capture fails, behavior degrades to viewport capture.

## Code Anchors
- `cmd/browser-agent/tools_observe.go`
- `cmd/browser-agent/tools_observe_registry.go`
- `cmd/browser-agent/tools_observe_response.go`
- `cmd/browser-agent/tools_observe_analysis.go`
- `cmd/browser-agent/tools_observe_bundling.go`
- `internal/capture/queries.go`
- `src/background/commands/observe.ts`
