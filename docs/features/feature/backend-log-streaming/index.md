---
doc_type: feature_index
feature_id: feature-backend-log-streaming
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - cmd/browser-agent/tools_configure.go
  - cmd/browser-agent/internal/toolconfigure/netrecord/handlers.go
  - cmd/browser-agent/internal/toolconfigure/netrecord/state.go
  - cmd/browser-agent/internal/toolconfigure/netrecord/filters.go
  - internal/capture/accessors.go
  - internal/capture/capture.go
  - internal/capture/model.go
  - internal/capture/events.go
  - internal/capture/extension_logs.go
  - internal/capture/extension_state.go
  - internal/capture/handlers.go
  - internal/capture/query_dispatcher.go
  - internal/capture/sync.go
  - internal/capture/test_helpers.go
  - internal/circuit/breaker.go
  - internal/debuglog/logger.go
  - internal/lifecycle/observer.go
  - internal/capture/wsconn/doc.go
  - internal/capture/wsconn/status.go
  - internal/capture/wsconn/tracker.go
  - internal/types/log.go
  - internal/types/network.go
  - src/background/sync/server.ts
  - src/background/sync/circuit-breaker.ts
  - src/background/sync/batchers.ts
  - src/background/sync/log-processing.ts
  - src/background/sync/screenshot.ts
  - src/background/index.ts
  - src/background/init.ts
  - src/background/caches/cache-limits.ts
  - src/background/caches/error-groups.ts
  - src/background/caches/snapshots.ts
  - src/background/caches/debug-log.ts
  - src/background/sync/batchers.ts
  - src/background/sync/batcher-instances.ts
  - src/background/sync/sync-manager.ts
  - src/background/sync/sync-client.ts
  - src/lib/daemon-http.ts
  - src/lib/net/network.ts
  - src/lib/net/websocket.ts
  - src/lib/net/websocket-tracking.ts
  - src/early-patch.ts
  - src/lib/page/safe-global-patch.ts
test_paths:
  - cmd/browser-agent/tools_configure_network_recording_test.go
  - cmd/browser-agent/internal/toolconfigure/netrecord/netrecord_test.go
  - internal/capture/sync_test.go
  - internal/capture/sync_test_helpers_test.go
  - internal/capture/settings_path_test.go
  - internal/capture/coverage_gaps_part2_test.go
  - internal/capture/api_contract_test.go
  - internal/capture/extension_log_store_test.go
  - internal/capture/buffer_clear_test.go
  - internal/capture/testhelpers_test.go
  - internal/capture/no_facade_test.go
  - internal/circuit/breaker_test.go
  - internal/debuglog/logger_test.go
  - internal/lifecycle/observer_test.go
  - tests/extension/sync-client.test.js
  - tests/extension/server.test.js
  - tests/extension/background-batching.test.js
  - tests/extension/batcher-instances.test.js
  - tests/extension/sync-manager.test.js
  - tests/extension/observe-screenshot.test.js
  - tests/extension/no-compatibility-facades.test.js
  - tests/extension/websocket-tracking.test.js
  - tests/extension/early-patch-hardened-restore.test.js
  - tests/extension/early-patch-branding.test.js
  - tests/extension/safe-global-patch.test.js
last_verified_version: 0.8.1
last_verified_date: 2026-04-13
---

# Backend Log Streaming

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/backend-log-streaming`

Capture dependencies use the canonical `capture.Capture` container type.
The former `capture.Store` and `capture.Snapshot` aliases have been removed.
Capture APIs and their callers use the canonical wire contracts from
`internal/types` directly; `internal/capture` does not re-export wire types.

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_BACKEND_LOG_STREAMING_001
- FEATURE_BACKEND_LOG_STREAMING_002
- FEATURE_BACKEND_LOG_STREAMING_003

## Code and Tests

- `internal/capture/sync_test_helpers_test.go` centralizes `/sync` request marshaling, transport dispatch, and response decoding helpers.
- `internal/capture/sync_test.go` now reuses those helpers across heartbeat, adaptive polling, and command lifecycle tests.
- Additional capture contract tests (`settings_path_test`, `coverage_gaps_part2_test`, `api_contract_test`) now reuse shared helper assertions to keep endpoint/status checks consistent.
- `src/background/sync/server.ts` now treats popup/background `connected` as daemon-confirmed heartbeat state instead of raw `/health` reachability.
