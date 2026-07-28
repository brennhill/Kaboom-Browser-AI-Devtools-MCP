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
  - internal/queries/dispatcher_queries.go
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
  - src/background.ts
  - src/inject.ts
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
  - tests/extension/network-bodies.test.js
  - tests/extension/network-waterfall.test.js
  - tests/extension/websocket.test.js
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
Persisted extension settings are read and written only through the canonical
`internal/state.SettingsFile` location; there is no fallback settings reader.
The former `capture.Store` and `capture.Snapshot` aliases have been removed.
Capture APIs and their callers use the canonical wire contracts from
`internal/types` directly; `internal/capture` does not re-export wire types.
The unused `EventBuffers`, `NetworkWaterfallStore`, `ExtensionLogStore`, and
`PerformanceSnapshotStore` read-only view layer has been deleted; it wrapped
canonical capture methods and had no production consumers.
Dead exported capture methods for extension-version reads, lifecycle
unsubscription, settings-cache writes, and extension-log-only clearing have
also been removed. Startup settings loading and atomic `ClearAll` remain the
canonical behaviors.
Count, timestamp, and buffer-memory accessors used only by capture tests are
gone as well. Behavioral tests now count canonical detached snapshots, while
package-internal buffer tests inspect the owning `BufferStore` invariants
directly.
Extension runtime logs now live in an independently synchronized
`ExtensionLogStore` that owns timestamp normalization, redaction, bounded
retention, snapshots, and clearing. Production and test callers use
`Capture.ExtensionLogs()` directly; the former capture-level add/get facade and
raw buffer type have been deleted.
Browser resource timings likewise live in an independently synchronized
`NetworkWaterfallStore`, which owns page/timestamp tagging, capacity eviction,
snapshots, and clearing. All ingestion and analysis callers use
`Capture.NetworkWaterfall()`; the former capture-level add/get facade and raw
waterfall buffer are deleted.
Performance snapshots and pre-action correlation snapshots now share an
independently synchronized `PerformanceStore`. Callers use
`Capture.Performance()` for add/list/URL lookup and consume-on-read correlation;
the five former capture-level forwarding methods have been removed.
Rate limiting and circuit health use the canonical breaker returned by
`Capture.Circuit()`; the former four Capture forwarding methods are deleted.
Extension connection, pilot, tracked-tab, CSP, security-mode, command-heartbeat,
and test-boundary state now share the independently synchronized
`ExtensionRuntime` returned by `Capture.Extension()`. Sync ingestion and every
consumer use that owner directly; the 19 former Capture forwarding methods and
parent-lock coupling are deleted. Event ingestion takes detached test-boundary
snapshots before acquiring the buffer lock. The pre-`/sync` `ExtensionStatus`
envelope and `UpdateExtensionStatus` mutation API remain deleted.
Tests likewise mutate extension-owned pilot, tracking, connection, tab, and CSP
state through `Capture.Extension()`; the seven Capture-level test-helper
facades are deleted. The remaining sync simulation helper stays on Capture
because it coordinates extension state with lifecycle emission.
Buffer-clear APIs likewise return `internal/types.BufferClearCounts` directly.
The background manifest entrypoint performs initialization only; batching,
transport, cache, and processing consumers import their canonical owner modules
directly instead of relying on re-exported compatibility names.
The injected page-world entrypoint likewise owns startup only. Network,
WebSocket, and performance consumers import the focused modules that implement
those contracts.

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
