---
doc_type: flow_map
flow_id: shared-extraction-and-contract-normalization
status: active
last_reviewed: 2026-03-05
owners:
  - Brenn
entrypoints:
  - src/lib/daemon-http.ts:buildDaemonHeaders
  - src/background/commands/analyze.ts:runFrameAwareAnalyzeQuery
  - src/popup/action-recording.ts:callConfigureFromPopup
  - src/background/sync-client.ts:SyncClient.doSync
  - src/background/dom/dom-dispatch.ts:resolveExecutionTarget
  - src/background/commands/interact-content.ts:contentExtractorCommand
  - src/background/context-menus.ts:installContextMenus
  - src/background/keyboard-shortcuts.ts:installDrawModeCommandListener
  - internal/tools/analyze/args_parse.go:parseAnalyzeArgs
  - internal/pagination/test_helpers_test.go:assertPaginationCountAndTotal
  - internal/session/verify_actions.go:VerificationManager.Watch
  - internal/tools/configure/rewrite.go:RewriteNoiseRuleArgs
code_paths:
  - src/lib/daemon-http.ts
  - src/background/server.ts
  - src/background/sync-client.ts
  - src/background/upload-handler.ts
  - src/background/message-handlers.ts
  - src/background/commands/observe.ts
  - src/offscreen/recording-worker.ts
  - src/popup/action-recording.ts
  - src/background/frame-targeting.ts
  - src/background/dom/dom-frame-probe.ts
  - src/background/commands/analyze.ts
  - src/background/dom/dom-dispatch.ts
  - src/background/content-fallback-scripts.ts
  - src/background/commands/interact-content.ts
  - src/background/commands/interact-explore.ts
  - src/background/draw-mode-toggle.ts
  - src/background/context-menus.ts
  - src/background/keyboard-shortcuts.ts
  - src/background/recording/utils.ts
  - src/background/recording/listeners.ts
  - src/inject/observers.ts
  - src/lib/network.ts
  - internal/tools/analyze/args_parse.go
  - internal/tools/analyze/forms.go
  - internal/tools/analyze/computed_styles.go
  - internal/tools/analyze/visual_diff.go
  - internal/pagination/test_helpers_test.go
  - internal/pagination/pagination_test.go
  - internal/pagination/pagination_actions_test.go
  - internal/pagination/pagination_websocket_test.go
  - internal/types/snapshot.go
  - internal/session/types.go
  - internal/session/verify_actions.go
  - internal/mcp/response_json.go
  - internal/tools/configure/boundaries.go
  - internal/tools/configure/rewrite.go
  - internal/tools/observe/handlers_extension_logs.go
  - internal/schema/generate.go
test_paths:
  - extension/background/__tests__/dom-dispatch-structured.test.js
  - tests/extension/interact-content-fallback.test.js
  - tests/extension/sync-client.test.js
  - tests/extension/recording-shortcut-command.test.js
  - internal/tools/analyze/forms_test.go
  - internal/tools/analyze/computed_styles_test.go
  - internal/tools/analyze/visual_diff_test.go
  - internal/pagination/pagination_test.go
  - internal/pagination/pagination_actions_test.go
  - internal/pagination/pagination_websocket_test.go
  - internal/pagination/cursor_test.go
  - internal/session/verify_test.go
  - internal/mcp/response_test.go
  - internal/tools/configure/boundaries_test.go
  - internal/tools/configure/rewrite_test.go
  - internal/tools/observe/analysis_test.go
  - internal/schema/interact_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Shared Extraction and Contract Normalization

## Scope

Covers shared helper extraction and contract normalization refactors applied across interact/analyze/observe/session/configure/generate/pagination code paths to reduce drift and duplicated logic.

## Entrypoints

1. Frame-targeted analyze/interact commands route through shared frame normalization/probing helpers.
2. Content extraction fallbacks are centralized and reused by both `interact-content` and `explore_page`.
3. Draw-mode toggle behavior is centralized and reused by keyboard shortcut and context-menu entrypoints.
4. Daemon JSON POST/request headers are built through a shared helper used by background and popup/offscreen paths.
5. Analyze argument parsing now uses one typed helper and pagination test suites share assertion helpers.

## Primary Flow

1. Normalize input once in shared helpers (`frame-targeting`, `daemon-http`, analyze arg parsers, response builders).
2. Reuse a single utility for execution-path-specific behavior (frame ID resolution, draw-mode toggling, recording slug generation, extension log shaping).
3. Keep caller paths thin and focused on entrypoint-specific concerns (toast/log wording, command registration, response routing).
4. Preserve existing wire contracts while removing internal duplicate branches.

## Error and Recovery Paths

1. Shared frame helpers throw stable `invalid_frame` and `frame_not_found` errors for all consumers.
2. Draw-mode helper falls back to start when state query fails, then propagates unreachable errors to caller-specific UX.
3. Configure and analyze parsing helpers return consistent structured errors for invalid JSON and missing required params.
4. Response JSON helpers consistently use safe marshal fallbacks for both success and error payloads.
5. Cursor-pagination test helpers keep metadata assertions aligned across log/action/websocket paths.

## State and Contracts

1. Snapshot schema is canonicalized in `internal/types/snapshot.go`; session package uses aliases to avoid contract drift.
2. Shared fallback extractors preserve existing response fields and `fallback: true` flag.
3. Header sanitization in fetch error/success paths uses shared `sanitizeHeaders`.
4. Daemon HTTP client headers now come from a single builder to avoid version/header drift.
5. Failure schema fragments in generate schema are sourced from one helper to avoid divergent field definitions.

## Code Paths

- `src/lib/daemon-http.ts`
- `src/background/server.ts`
- `src/background/sync-client.ts`
- `src/background/upload-handler.ts`
- `src/background/message-handlers.ts`
- `src/background/commands/observe.ts`
- `src/offscreen/recording-worker.ts`
- `src/popup/action-recording.ts`
- `src/background/frame-targeting.ts`
- `src/background/dom/dom-frame-probe.ts`
- `src/background/commands/analyze.ts`
- `src/background/dom/dom-dispatch.ts`
- `src/background/content-fallback-scripts.ts`
- `src/background/commands/interact-content.ts`
- `src/background/commands/interact-explore.ts`
- `src/background/draw-mode-toggle.ts`
- `src/background/context-menus.ts`
- `src/background/keyboard-shortcuts.ts`
- `src/background/recording/utils.ts`
- `src/background/recording/listeners.ts`
- `src/inject/observers.ts`
- `internal/tools/analyze/args_parse.go`
- `internal/tools/analyze/forms.go`
- `internal/tools/analyze/computed_styles.go`
- `internal/tools/analyze/visual_diff.go`
- `internal/pagination/test_helpers_test.go`
- `internal/pagination/pagination_test.go`
- `internal/pagination/pagination_actions_test.go`
- `internal/pagination/pagination_websocket_test.go`
- `internal/types/snapshot.go`
- `internal/session/types.go`
- `internal/session/verify_actions.go`
- `internal/mcp/response_json.go`
- `internal/tools/configure/boundaries.go`
- `internal/tools/configure/rewrite.go`
- `internal/tools/observe/handlers_extension_logs.go`
- `internal/schema/generate.go`

## Test Paths

- `extension/background/__tests__/dom-dispatch-structured.test.js`
- `tests/extension/interact-content-fallback.test.js`
- `tests/extension/sync-client.test.js`
- `tests/extension/recording-shortcut-command.test.js`
- `internal/tools/analyze/forms_test.go`
- `internal/tools/analyze/computed_styles_test.go`
- `internal/tools/analyze/visual_diff_test.go`
- `internal/pagination/pagination_test.go`
- `internal/pagination/pagination_actions_test.go`
- `internal/pagination/pagination_websocket_test.go`
- `internal/pagination/cursor_test.go`
- `internal/session/verify_test.go`
- `internal/mcp/response_test.go`
- `internal/tools/configure/boundaries_test.go`
- `internal/tools/configure/rewrite_test.go`
- `internal/tools/observe/analysis_test.go`
- `internal/schema/interact_test.go`

## Edit Guardrails

1. Add new frame-targeted actions by reusing `normalizeFrameArg` and `resolveMatchedFrameIds`.
2. Keep executeScript fallback functions self-contained; add new fallbacks in `content-fallback-scripts.ts` first.
3. Keep daemon-facing JSON POSTs on `buildDaemonJSONRequestInit`/`postDaemonJSON` to avoid client/header drift.
4. Keep analyze arg parsing in `parseAnalyzeArgs` for new analyze modes.
5. Add cursor-pagination assertion changes to shared pagination test helpers before touching per-buffer suites.
6. Keep feature docs in sync:
   - `docs/features/feature/interact-explore/flow-map.md`
   - `docs/features/feature/analyze-tool/flow-map.md`
   - `docs/features/feature/flow-recording/flow-map.md`
   - `docs/features/feature/observe/flow-map.md`
   - `docs/features/feature/backend-log-streaming/flow-map.md`
   - `docs/features/feature/cursor-pagination/flow-map.md`
   - `docs/features/feature/tab-recording/flow-map.md`
