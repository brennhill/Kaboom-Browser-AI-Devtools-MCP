---
doc_type: feature_index
feature_id: feature-backend-log-streaming
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-05
code_paths:
  - internal/capture/accessor.go
  - internal/capture/buffer_clear.go
  - internal/capture/buffer-types.go
  - internal/capture/capture-struct.go
  - internal/capture/circuit_breaker.go
  - internal/capture/constants.go
  - internal/capture/debug_logger.go
  - internal/capture/debug.go
  - internal/capture/enhanced_actions.go
  - internal/capture/enhanced-actions-types.go
  - internal/capture/extension_log_redaction.go
  - internal/capture/extension_log_store.go
  - internal/capture/extension_logs.go
  - internal/capture/extension_state.go
  - internal/capture/extension-logging-types.go
  - internal/capture/handlers.go
  - internal/capture/helpers.go
  - internal/capture/http_debug_redaction.go
  - internal/capture/interfaces.go
  - internal/capture/internal-types.go
  - internal/capture/log-diff.go
  - internal/capture/memory.go
  - internal/capture/network_bodies.go
  - internal/capture/network_waterfall.go
  - internal/capture/network-types.go
  - internal/capture/playback.go
  - internal/capture/queries.go
  - internal/capture/query_dispatcher.go
  - internal/capture/rate_limit.go
  - internal/capture/recording_manager.go
  - internal/capture/recording.go
  - internal/capture/security-types.go
  - internal/capture/session-types.go
  - internal/capture/settings.go
  - internal/capture/status.go
  - internal/capture/sync.go
  - internal/capture/sync_processing.go
  - internal/capture/test_helpers.go
  - internal/capture/testhelpers.go
  - internal/capture/ttl.go
  - internal/capture/type-aliases.go
  - internal/capture/types.go
  - internal/capture/websocket-types.go
  - internal/capture/websocket.go
  - src/background/server.ts
  - src/background/index.ts
  - src/background/sync-client.ts
  - src/lib/daemon-http.ts
  - src/lib/network.ts
  - src/lib/websocket.ts
test_paths:
  - internal/capture/sync_test.go
  - internal/capture/sync_test_helpers_test.go
  - internal/capture/settings_path_test.go
  - internal/capture/coverage_gaps_part2_test.go
  - internal/capture/api_contract_test.go
  - internal/capture/extension_log_store_test.go
  - internal/capture/buffer_clear_test.go
  - tests/extension/sync-client.test.js
  - tests/extension/server.test.js
  - tests/extension/background-batching.test.js
last_verified_version: 0.8.1
last_verified_date: 2026-04-13
---

# Backend Log Streaming

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/backend-log-streaming`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Flow Map: [flow-map.md](./flow-map.md)

## Requirement IDs

- FEATURE_BACKEND_LOG_STREAMING_001
- FEATURE_BACKEND_LOG_STREAMING_002
- FEATURE_BACKEND_LOG_STREAMING_003

## Code and Tests

- `internal/capture/sync_test_helpers_test.go` centralizes `/sync` request marshaling, transport dispatch, and response decoding helpers.
- `internal/capture/sync_test.go` now reuses those helpers across heartbeat, adaptive polling, and command lifecycle tests.
- Additional capture contract tests (`settings_path_test`, `coverage_gaps_part2_test`, `api_contract_test`) now reuse shared helper assertions to keep endpoint/status checks consistent.
- `src/background/server.ts` now treats popup/background `connected` as daemon-confirmed heartbeat state instead of raw `/health` reachability.
