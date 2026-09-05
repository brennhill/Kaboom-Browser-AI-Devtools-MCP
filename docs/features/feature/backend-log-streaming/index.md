---
doc_type: feature_index
feature_id: feature-backend-log-streaming
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-09-05
code_paths:
  - cmd/browser-agent/openapi.json
  - cmd/browser-agent/internal/toolruntime/tools_configure.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher.go
  - cmd/browser-agent/internal/toolconfigure/netrecord/handlers.go
  - cmd/browser-agent/internal/toolconfigure/netrecord/state.go
  - cmd/browser-agent/internal/toolconfigure/netrecord/filters.go
  - internal/capture/healthreader/reader.go
  - internal/capture/actionstore/store.go
  - internal/capture/bodystore/store.go
  - internal/capture/capture.go
  - internal/capture/clientstore/owner.go
  - internal/capture/settingscache/loader.go
  - internal/capture/telemetrystore/store.go
  - internal/capture/resetter/resetter.go
  - internal/capture/logstore/store.go
  - internal/capture/perfstore/store.go
  - internal/capture/pressure/stats.go
  - internal/capture/ringstore/store.go
  - internal/capture/waterfallstore/store.go
  - internal/capture/syncruntime/runtime.go
  - internal/capture/featureusage/observer.go
  - internal/capture/httpingest/handlers.go
  - internal/util/media.go
  - internal/queries/dispatcher_queries.go
  - internal/capture/syncruntime/handler.go
  - internal/capture/syncruntime/wire_sync.go
  - internal/commandcontract/generated.go
  - internal/capturefixture/sync.go
  - internal/circuit/breaker.go
  - internal/debuglog/logger.go
  - internal/lifecycle/observer.go
  - internal/capture/wsconn/doc.go
  - internal/capture/wsconn/status.go
  - internal/capture/wsconn/store.go
  - internal/capture/wsconn/tracker.go
  - internal/types/wire_log.go
  - internal/types/network.go
  - src/background.ts
  - src/inject.ts
  - src/background/sync/server.ts
  - src/background/sync/circuit-breaker.ts
  - src/background/sync/batchers.ts
  - src/background/sync/log-processing.ts
  - src/background/sync/screenshot.ts
  - src/background/debug.ts
  - src/background/orchestration/connection-monitor.ts
  - src/background/orchestration/stream-runtime.ts
  - src/background/init.ts
  - src/background/message-handlers.ts
  - src/background/message-routing/
  - src/background/runtime-state/
  - src/background/runtime-state/connection-generation.ts
  - src/background/runtime-state/csp-state.ts
  - src/background/runtime-state/state-recovery.ts
  - src/background/runtime-state/log-queue.ts
  - src/background/caches/cache-limits.ts
  - src/background/caches/error-groups.ts
  - src/background/caches/snapshots.ts
  - src/background/caches/debug-log.ts
  - src/background/sync/batchers.ts
  - src/background/sync/batcher-instances.ts
  - src/background/sync/sync-manager.ts
  - src/background/sync/sync-client.ts
  - src/types/runtime/command-contract.ts
  - src/types/wire/wire-sync.ts
  - src/types/wire/wire-extension-log.ts
  - src/background/sync/install-identity.ts
  - src/content/page-telemetry.ts
  - src/content/window-message-listener.ts
  - src/lib/daemon-http.ts
  - src/lib/page/channel.ts
  - src/lib/storage/recovery.ts
  - src/lib/storage/fault.ts
  - src/lib/storage/validated.ts
  - src/lib/net/network.ts
  - src/lib/net/websocket.ts
  - src/lib/net/websocket-tracking.ts
  - src/early-patch.ts
  - src/lib/page/safe-global-patch.ts
test_paths:
  - internal/capture/resetter/resetter_test.go
  - internal/capture/httpingest/handlers_test.go
  - internal/capture/clientstore/owner_test.go
  - internal/capture/actionstore/store_test.go
  - internal/capture/wsconn/store_test.go
  - internal/capture/bodystore/store_test.go
  - internal/capture/settingscache/loader_test.go
  - internal/capture/telemetrystore/store_test.go
  - internal/capture/perfstore/store_test.go
  - internal/capture/ringstore/store_test.go
  - internal/capture/waterfallstore/store_test.go
  - scripts/contracts/check-architecture-boundaries.test.cjs
  - tests/extension/contracts/background-boundaries.test.js
  - tests/extension/contracts/no-dynamic-import-background.test.js
  - tests/extension/state-recovery/state-recovery-contract.test.js
  - tests/extension/state-recovery/validated-storage.test.js
  - tests/extension/state-recovery/storage-fault-fixture.js
  - tests/extension/reliability/diagnostic-log-queue.test.js
  - tests/extension/branding/install-identity-faults.test.js
  - cmd/browser-agent/internal/toolconfigure/dispatcher_test.go
  - cmd/browser-agent/internal/toolconfigure/netrecord/netrecord_test.go
  - internal/capture/syncruntime/sync_test.go
  - tests/extension/sync/sync-client.test.js
  - internal/capture/syncruntime/sync_test_helpers_test.go
  - internal/capturefixture/sync_test.go
  - internal/capture/syncruntime/sync_command_lifecycle_test.go
  - internal/capture/syncruntime/readiness_gate_test.go
  - internal/capture/async_queue_integration_test.go
  - internal/capture/syncruntime/integrationtest/sync_waterfall_test.go
  - internal/capture/websockettest/websocket_test.go
  - internal/capture/websockettest/websocket_status_test.go
  - internal/capture/websockettest/websocket_handlers_test.go
  - internal/capture/websockettest/websocket-streaming_test.go
  - internal/capture/syncruntime/sync_test_helpers_test.go
  - internal/capture/coverage_gaps_part2_test.go
  - internal/capture/pipelinetest/api_contract_test.go
  - internal/capture/logstore/store_test.go
  - internal/capture/logstore/diagnostic_test.go
  - internal/capture/pipelinetest/accessor_unit_test.go
  - internal/capture/featureusage/observer_test.go
  - internal/capture/syncruntime/extension_state_unit_test.go
  - internal/capture/pipelinetest/buffer_clear_test.go
  - internal/capture/pipelinetest/capture_stress_test.go
  - internal/capture/pipelinetest/capture_bench_test.go
  - internal/capture/testhelpers_test.go
  - internal/capture/no_facade_test.go
  - internal/capture/healthreader/reader_test.go
  - internal/capture/syncruntime/integrationtest/sync_handler_owner_test.go
  - internal/circuit/breaker_test.go
  - internal/debuglog/logger_test.go
  - internal/lifecycle/observer_test.go
  - tests/extension/sync/sync-client-commands.test.js
  - tests/extension/sync/sync-client-fixture.js
  - tests/extension/sync/sync-client-resilience.test.js
  - tests/extension/sync/sync-client.test.js
  - tests/extension/branding/install-id.test.js
  - tests/extension/reliability/server.test.js
  - tests/extension/reliability/diagnostic-log-queue.test.js
  - tests/extension/sync/background-batching.test.js
  - tests/extension/sync/batcher-instances.test.js
  - tests/extension/performance/rate-limit.test.js
  - tests/extension/sync/sync-manager.test.js
  - tests/extension/ui-controls/ui-usage-tracker.test.js
  - scripts/contracts/sync-wire-generated.test.cjs
  - scripts/contracts/testdata/sync-roundtrip.json
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
- Tool: See feature contract and `docs/core/protocol/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/protocol/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
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
Capture stores take ownership of nested mutable values at ingestion and return
deeply detached snapshots. Response-header maps, test-ID slices, WebSocket
sampling metadata, extension-log JSON, enhanced-action selector trees, and
performance collections cannot be mutated through caller-owned backing memory
or through a later read.
Concurrent capture stress tests release all readers, writers, clearers, and
snapshotters through an explicit start barrier. Scheduler yields broaden the
interleavings under the race detector without using elapsed time as a
correctness signal.
HTTP ingestion, query-result, recording-storage, performance, WebSocket, and
circuit-health routes are owned by `httpingest.Handlers`. Server registration
and tests construct that owner directly; the corresponding `Capture.Handle*`
methods were deleted rather than retained as forwarding facades. The sync
transport is likewise owned by `capture.SyncHandler`, which composes extension
liveness, command results, long-poll delivery, lifecycle events, and sync
diagnostics without adding forwarding methods to `Capture`.
The extension sync client retains only the five newest acknowledged command
signatures for duplicate-delivery protection. In-progress commands remain in
their separate active map until completion. The short terminal window prevents
a restarted daemon's reused query IDs from being mistaken for stale commands;
durable event recordings are owned independently by `RecordingManager`.
The sync lifecycle receives its clock, scheduler, retry randomness, and HTTP
transport through one explicit runtime boundary. Deterministic tests advance
polls and command deadlines directly, without wall-clock sleeps. Every repeated
daemon delivery is checked against the full in-progress set before the bounded
acknowledged history, and transport-created timeout results preserve the
original correlation ID and connection generation while aborting the command
execution signal.
Terminal command results remain owned by the sync client until a successful
daemon acknowledgement. The client does not truncate older terminal outcomes:
the daemon's bounded delivery capacity limits how much new work can be accepted
between successful syncs, while lossless retention prevents an already-executed
mutation from later appearing safe to retry.
Every injected telemetry producer uses the canonical authenticated page
channel. The content boundary rejects missing or incorrect nonces, malformed
wire shapes, cyclic/deep structures, excessive collections, and payloads above
the byte budget before they reach background queues. Rejections produce only a
deduplicated, redacted local diagnostic; captured page values are never copied
into the diagnostic.
Daemon HTTP reachability and extension sync-heartbeat state are tracked as
separate signals. A missing or stale heartbeat remains visible to Doctor and
the connection log, but cannot label a daemon that answered `/health` as
offline. Sync reconnect transitions update only heartbeat state; actual HTTP
failures own the daemon-offline state.
The canonical extension diagnostic queue is a 200-entry redacted ring persisted
in `chrome.storage.session`. It restores before startup logging, survives MV3
worker restarts and daemon outages, and acknowledges only the entries accepted
by a successful `/sync` cycle. Queue saturation, persistence failures, worker
startup/suspension approximation, sync transitions, fetch failures, and runtime
message-handler failures remain local and carry correlation IDs where they
cross an asynchronous lifecycle boundary. Diagnostic contents never enter the
external usage-telemetry producer.
Sync reconciliation tolerates partial heartbeat/result batches: an
acknowledged command that briefly disappears from `in_progress` remains pending
for a bounded two-second result-delivery grace. A command still absent after
that grace fails with `extension_lost_command`, preserving fast recovery from a
genuinely lost extension worker without racing valid subtitle, highlight, or
DOM-action results already being flushed.
Aggregate health reads are owned by `healthreader.Reader`; it snapshots the
independently synchronized telemetry, query, extension, and circuit owners
without a cross-owner method on `Capture`.
Coordinated runtime clearing is owned by `resetter.Resetter`; it resets test
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
gone as well. Behavioral tests now inspect canonical detached snapshots and
allocation-free owner statistics directly.
Extension runtime logs now live in an independently synchronized
`ExtensionLogStore` that owns timestamp normalization, redaction, bounded
retention, snapshots, and clearing. Production and test callers use
`Capture.ExtensionLogs()` directly; the former capture-level add/get facade and
raw buffer type have been deleted.
Daemon polling and HTTP diagnostics likewise use the canonical
`DiagnosticLogStore` returned by `Capture.DiagnosticLogs()`. It owns HTTP-field
redaction and the bounded debug logger; the four Capture-level forwarding and
redaction methods are deleted.
Browser resource timings likewise live in the independently synchronized
`waterfallstore.Store`, which owns page/timestamp tagging, capacity eviction,
snapshots, and clearing. All ingestion and analysis callers use
`Capture.Telemetry().NetworkWaterfall()`; the former capture-level access,
add/get facades, raw waterfall buffer, embedded implementation, and root store
tests are deleted.
Waterfall timings, WebSocket events, network bodies, and enhanced actions use
the canonical `ringstore.Store` fixed-capacity FIFO. The store owns circular
retention and slot release while each feature owner retains its surrounding
lock and counters; steady-state eviction therefore overwrites the oldest slot
without allocating or copying the retained window. The former private ring
implementation and root-level tests are deleted rather than retained as a
parallel storage surface.
Enhanced actions live in the independently synchronized `actionstore.Store`,
which owns navigation signaling, deep selector cloning, bounded retention,
timestamps, allocation-free statistics, detached evidence, pressure, and
clearing. Consumers use `Capture.Telemetry().Actions()` directly; the two
Telemetry forwarding methods and shared buffer fields are deleted.
Network bodies live in the independently synchronized `bodystore.Store`, which
owns bounded retention, deep cloning, error and ingestion totals, timestamps,
memory-pressure eviction, snapshots, and clearing. Consumers use
`Capture.Telemetry().NetworkBodies()` directly; the three Telemetry forwarding
methods and the former shared-buffer fields are deleted. WebSocket events and
their derived connection state live together in `wsconn.Store`; ingestion,
retention, filtering, status, memory pressure, and clearing share one lock.
Consumers use `Capture.Telemetry().WebSockets()` directly, and the former
Telemetry forwarding methods plus `BufferStore` are deleted. Navigation
callbacks and the focused action, body, waterfall, and WebSocket owners compose
`telemetrystore.Store`; `resetter.Resetter` coordinates the genuinely cross-owner reset.
Configure network-recording dispatch calls `netrecord.HandleNetworkRecording`
with the telemetry and recording-state owners directly; the root ToolHandler
forwarding method is deleted.
Performance snapshots and pre-action correlation snapshots now share an
independently synchronized `perfstore.Store`. Callers use
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
synchronized `clientstore.Owner` returned by `Capture.Clients()`; the
Capture-level set/get facades are deleted. With every mutable field assigned to
an owner, Capture is now a lock-free composition root. Extension-state tests
also acquire the extension owner lock rather than the former unrelated Capture
mutex.
Extension settings cache I/O lives in the focused `settingscache` loader rather
than the live extension-state lock owner. Missing and stale cache entries are
explicit expected fallbacks; read, parse, and timestamp corruption fail open to
safe defaults and emit fixed, redacted Doctor diagnostics. A later valid load
resolves the same diagnostic lifecycle without retaining raw paths or values.
The former mixed `model.go` is deleted. Telemetry capacities and memory budgets
now live beside `telemetrystore.Store`, the shared extension-ingest body limit lives
beside the HTTP boundary, disconnect timing lives beside extension state, and
rate-limit consumers use the canonical circuit contract directly.
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

The daemon owns a monotonic connection generation for the active extension
runtime. `/sync` responses and delivered commands carry that generation; the
extension returns it with heartbeats and command results. A superseded
heartbeat, result, long-poll response, or command is rejected before it can
mutate current state, and the rejection is retained as a correlated lifecycle
diagnostic. Daemon handoffs also invalidate in-flight extension responses so an
old server cannot dispatch work after the client changes endpoints.

The complete `/sync` request/response graph now has one Go wire owner in
`internal/capture/syncruntime/wire_sync.go`; extension diagnostic entries have their one
nested owner in `internal/types/wire_log.go`. TypeScript contracts and
the OpenAPI sync components are generated from those structs, and CI compares
the generated OpenAPI client plus a shared bidirectional JSON fixture. The old
hand-maintained optionality checker and duplicate TypeScript interfaces were
deleted rather than retained as compatibility surfaces.

Disposable network-body, WebSocket, enhanced-action, and extension-diagnostic
streams enforce their declared capacity on every ingestion. Their canonical
owners expose current size, capacity, cumulative drops, and oldest age so
pressure and recovery are observable without competing with active commands.
Extension telemetry batchers retain failed batches and schedule one
lifecycle-cancellable half-open probe even when no new page event arrives.
Circuit recovery tests inject one controlled clock and timer queue, then advance
exactly one reset window before awaiting the active flush. A later retry window
therefore cannot race success, reopen, or retained-buffer assertions.
Each stream accounts exactly for received entries as delivered, retained, or
dropped; capacity and requeue overflow emit redacted structured diagnostics and
a correlated Doctor incident, which resolves after delivery recovers. The dead
non-circuit log batcher was removed so every production telemetry stream shares
this one bounded delivery contract.

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_BACKEND_LOG_STREAMING_001
- FEATURE_BACKEND_LOG_STREAMING_002
- FEATURE_BACKEND_LOG_STREAMING_003

## Code and Tests

Extension and daemon diagnostic logs now belong to the focused
`internal/capture/logstore` package. It owns redaction, bounded retention,
detached reads, and drop-pressure reporting; the root `Capture` type only
composes and exposes that canonical owner. Generic bounded-resource metrics
belong to `internal/capture/pressure`, so telemetry, performance, and health
reporting share one neutral value contract without importing a broad capture
implementation or preserving an obsolete compatibility surface.

- `internal/capture/syncruntime/sync_test_helpers_test.go` centralizes `/sync` request marshaling, transport dispatch, and response decoding helpers.
- `internal/capture/syncruntime/sync_test.go` reuses those helpers for request ingestion, heartbeats, and connection state.
- `internal/capture/syncruntime/sync_command_lifecycle_test.go` owns adaptive polling and command-result lifecycle coverage.
- `internal/capture/syncruntime/integrationtest/sync_waterfall_test.go` owns waterfall query and result delivery coverage.
- Additional capture contract tests (`coverage_gaps_part2_test`, `api_contract_test`) reuse shared helper assertions to keep endpoint/status checks consistent; canonical settings-path and recovery coverage lives with `settingscache`.
- `src/background/sync/server.ts` now treats popup/background `connected` as daemon-confirmed heartbeat state instead of raw `/health` reachability.
- `tests/extension/performance/rate-limit.test.js` deterministically covers autonomous half-open probe success, failure, buffer drain, and reopen transitions without wall-clock sleeps.
- Correlation-view tests drive canonical command terminal transitions directly;
  deadline scheduling and cleanup signaling remain isolated in the query
  expiration suite, so lifecycle projection tests never poll wall-clock time.
- Internal log timestamp bounds use the server's private clock boundary;
  snapshot and accessor tests assert exact instants without scheduler delays.
- Enhanced-action navigation callbacks cross one private dispatch boundary.
  Production uses the panic-safe goroutine runner; tests execute synchronously
  to prove callback cardinality, negative cases, and lock release exactly.
- SyncHandler owns a private pending-query wait boundary. Handler tests control
  the wait and prove the required resnapshot; QueryDispatcher tests exclusively
  own timer and wakeup behavior.
- Extension readiness uses a lock-owned generation notification shared by sync
  connection and authoritative disconnect transitions. Waiters cannot miss a
  reconnect between checking state and blocking, and cancellation no longer
  waits for a periodic polling tick. Transition coverage lives in the canonical
  readiness-gate suite; the older sleep-before-connect duplicate was deleted.
- Query cleanup lifecycle coverage lives with the canonical QueryDispatcher;
  the former Capture-level goroutine-count and duplicate result-wait tests were
  deleted rather than preserved as a cross-owner test facade.
- Capture queue integration tests prove only the Capture-to-QueryDispatcher
  composition. Queue concurrency, capacity, timeout, and cleanup semantics live
  with `internal/queries`; the duplicate jitter-driven reliability suite was
  removed so capture tests no longer depend on scheduler timing.
