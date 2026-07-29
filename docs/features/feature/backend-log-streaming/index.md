---
doc_type: feature_index
feature_id: feature-backend-log-streaming
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-29
code_paths:
  - cmd/browser-agent/tools_configure.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher.go
  - cmd/browser-agent/internal/toolconfigure/netrecord/handlers.go
  - cmd/browser-agent/internal/toolconfigure/netrecord/state.go
  - cmd/browser-agent/internal/toolconfigure/netrecord/filters.go
  - internal/capture/accessors.go
  - internal/capture/capture.go
  - internal/capture/model.go
  - internal/capture/events.go
  - internal/capture/extension_logs.go
  - internal/capture/extension_state.go
  - internal/capture/feature_usage.go
  - internal/capture/handlers.go
  - internal/util/url.go
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
  - src/background/message-handlers.ts
  - src/background/message-routing/
  - src/background/runtime-state/
  - src/background/caches/cache-limits.ts
  - src/background/caches/error-groups.ts
  - src/background/caches/snapshots.ts
  - src/background/caches/debug-log.ts
  - src/background/sync/batchers.ts
  - src/background/sync/batcher-instances.ts
  - src/background/sync/sync-manager.ts
  - src/background/sync/sync-client.ts
  - src/background/sync/install-identity.ts
  - src/lib/daemon-http.ts
  - src/lib/net/network.ts
  - src/lib/net/websocket.ts
  - src/lib/net/websocket-tracking.ts
  - src/early-patch.ts
  - src/lib/page/safe-global-patch.ts
test_paths:
  - tests/extension/contracts/background-boundaries.test.js
  - tests/extension/contracts/no-dynamic-import-background.test.js
  - cmd/browser-agent/internal/toolconfigure/dispatcher_test.go
  - cmd/browser-agent/tools_configure_network_recording_test.go
  - cmd/browser-agent/tools_configure_network_recording_handler_test.go
  - cmd/browser-agent/internal/toolconfigure/netrecord/netrecord_test.go
  - internal/capture/sync_test.go
  - internal/capture/sync_command_lifecycle_test.go
  - internal/capture/sync_waterfall_test.go
  - internal/capture/websocket_test.go
  - internal/capture/websocket_status_test.go
  - internal/capture/websocket_handlers_test.go
  - internal/capture/websocket-streaming_test.go
  - internal/capture/sync_test_helpers_test.go
  - internal/capture/settings_path_test.go
  - internal/capture/coverage_gaps_part2_test.go
  - internal/capture/api_contract_test.go
  - internal/capture/extension_log_store_test.go
  - internal/capture/feature_usage_test.go
  - internal/capture/client_registry_owner_test.go
  - internal/capture/extension_state_test_helpers_test.go
  - internal/capture/buffer_clear_test.go
  - internal/capture/bounded_ring_test.go
  - internal/capture/capture_bench_test.go
  - internal/capture/testhelpers_test.go
  - internal/capture/no_facade_test.go
  - internal/capture/http_handlers_owner_test.go
  - internal/capture/health_reader_owner_test.go
  - internal/capture/state_resetter_owner_test.go
  - internal/capture/sync_handler_owner_test.go
  - internal/circuit/breaker_test.go
  - internal/debuglog/logger_test.go
  - internal/lifecycle/observer_test.go
  - tests/extension/sync/sync-client-commands.test.js
  - tests/extension/sync/sync-client-fixture.js
  - tests/extension/sync/sync-client-resilience.test.js
  - tests/extension/sync/sync-client.test.js
  - tests/extension/branding/install-id.test.js
  - tests/extension/reliability/server.test.js
  - tests/extension/sync/background-batching.test.js
  - tests/extension/sync/batcher-instances.test.js
  - tests/extension/sync/sync-manager.test.js
  - tests/extension/misc/integration.test.cjs
  - tests/extension/capture/observe-screenshot.test.js
  - tests/extension/contracts/no-compatibility-facades.test.js
  - tests/extension/network-http/network-bodies-fixture.js
  - tests/extension/network-http/network-bodies-xhr.test.js
  - tests/extension/network-http/network-bodies.test.js
  - tests/extension/network-http/network-body-e2e-fixture.js
  - tests/extension/network-http/network-body-e2e.test.js
  - tests/extension/network-http/network-waterfall.test.js
  - tests/extension/network-realtime/websocket.test.js
  - tests/extension/network-realtime/websocket-tracking.test.js
  - tests/extension/injection/early-patch-hardened-restore.test.js
  - tests/extension/injection/early-patch-branding.test.js
  - tests/extension/injection/safe-global-patch.test.js
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
`internal/state.SettingsFile` location by `ExtensionRuntime`; there is no
Capture forwarding method or fallback settings reader.
The former `capture.Store` and `capture.Snapshot` aliases have been removed.
URL-path normalization consumers import `internal/util.ExtractURLPath`
directly; the capture-package pass-through and its duplicate tests are deleted.
Capture APIs and their callers use the canonical wire contracts from
`internal/types` directly; `internal/capture` does not re-export wire types.
HTTP ingestion, query-result, recording-storage, performance, WebSocket, and
circuit-health routes are owned by `capture.HTTPHandlers`. Server registration
and tests construct that owner directly; the corresponding `Capture.Handle*`
methods were deleted rather than retained as forwarding facades. The sync
transport is likewise owned by `capture.SyncHandler`, which composes extension
liveness, command results, long-poll delivery, lifecycle events, and sync
diagnostics without adding forwarding methods to `Capture`.
Aggregate health reads are owned by `capture.HealthReader`; it snapshots the
independently synchronized telemetry, query, extension, and circuit owners
without a cross-owner method on `Capture`.
Coordinated runtime clearing is owned by `capture.StateResetter`; it resets test
boundaries, telemetry, performance snapshots, and extension logs together
without exposing `ClearAll` on `Capture`.
The unused `EventBuffers`, `NetworkWaterfallStore`, `ExtensionLogStore`, and
`PerformanceSnapshotStore` read-only view layer has been deleted; it wrapped
canonical capture methods and had no production consumers.
Dead exported capture methods for extension-version reads, lifecycle
unsubscription, settings-cache writes, and extension-log-only clearing have
also been removed. Startup settings loading and atomic `ClearAll` remain the
canonical behaviors.
The background entry point exports only the batchers consumed by extension
startup. Circuit-breaker wrapper instances remain owned by the canonical
batcher factory and are not re-exported as an unused public surface.
Count, timestamp, and buffer-memory accessors used only by capture tests are
gone as well. Behavioral tests now count canonical detached snapshots, while
package-internal buffer tests inspect the owning `BufferStore` invariants
directly.
Extension runtime logs now live in an independently synchronized
`ExtensionLogStore` that owns timestamp normalization, redaction, bounded
retention, snapshots, and clearing. Production and test callers use
`Capture.ExtensionLogs()` directly; the former capture-level add/get facade and
raw buffer type have been deleted.
Daemon polling and HTTP diagnostics likewise use the canonical
`DiagnosticLogStore` returned by `Capture.DiagnosticLogs()`. It owns HTTP-field
redaction and the bounded debug logger; the four Capture-level forwarding and
redaction methods are deleted.
Browser resource timings likewise live in an independently synchronized
`NetworkWaterfallStore`, which owns page/timestamp tagging, capacity eviction,
snapshots, and clearing. All ingestion and analysis callers use
`Capture.Telemetry().NetworkWaterfall()`; the former capture-level access,
add/get facades, and raw waterfall buffer are deleted.
Waterfall timings, WebSocket events, network bodies, and enhanced actions use
fixed-capacity circular storage so steady-state eviction overwrites and releases
the oldest slot without allocating or copying the retained window.
Network bodies, WebSocket events and connection state, enhanced actions,
navigation callbacks, and the waterfall owner now form one independently
synchronized `TelemetryStore` returned by `Capture.Telemetry()`. Its consumers
use that owner directly; the former Capture buffer fields, WebSocket tracker,
navigation callback, five test helpers, and sixteen production forwarding
methods are deleted. `Capture.ClearAll` remains only because it genuinely
coordinates telemetry, extension boundaries, performance, and extension logs.
Configure network-recording dispatch calls `netrecord.HandleNetworkRecording`
with the telemetry and recording-state owners directly; the root ToolHandler
forwarding method is deleted.
Performance snapshots and pre-action correlation snapshots now share an
independently synchronized `PerformanceStore`. Callers use
`Capture.Performance()` for add/list/URL lookup and consume-on-read correlation;
the five former capture-level forwarding methods have been removed.
Rate limiting and circuit health use the canonical breaker returned by
`Capture.Circuit()`; the former four Capture forwarding methods are deleted.
Lifecycle publishers and subscribers likewise use the independently
synchronized observer returned by `Capture.Lifecycle()`; the Capture-level
subscribe and emit forwarding methods are deleted.
Extension feature-usage analytics use the independently synchronized
`FeatureUsageObserver` returned by `Capture.FeatureUsage()`. Callback
replacement and notification live together; Capture no longer stores or
forwards the callback.
Runtime client-registry installation and lookup use the independently
synchronized `ClientRegistryOwner` returned by `Capture.Clients()`; the
Capture-level set/get facades are deleted. With every mutable field assigned to
an owner, Capture is now a lock-free composition root. Extension-state tests
also acquire the extension owner lock rather than the former unrelated Capture
mutex.
Extension connection, pilot, tracked-tab, CSP, security-mode, command-heartbeat,
test-boundary, and server/extension compatibility state now share the independently synchronized
`ExtensionRuntime` returned by `Capture.Extension()`. Sync ingestion and every
consumer use that owner directly; the 19 former Capture forwarding methods and
parent-lock coupling are deleted. Event ingestion takes detached test-boundary
snapshots before acquiring the buffer lock. The pre-`/sync` `ExtensionStatus`
envelope and `UpdateExtensionStatus` mutation API remain deleted.
Server version storage and major/minor mismatch evaluation also live entirely
on `ExtensionRuntime`; the three Capture-level version forwarding methods and
split-lock state are deleted.
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
- `internal/capture/sync_test.go` reuses those helpers for request ingestion, heartbeats, and connection state.
- `internal/capture/sync_command_lifecycle_test.go` owns adaptive polling and command-result lifecycle coverage.
- `internal/capture/sync_waterfall_test.go` owns waterfall query and result delivery coverage.
- Additional capture contract tests (`settings_path_test`, `coverage_gaps_part2_test`, `api_contract_test`) now reuse shared helper assertions to keep endpoint/status checks consistent.
- `src/background/sync/server.ts` now treats popup/background `connected` as daemon-confirmed heartbeat state instead of raw `/health` reachability.
