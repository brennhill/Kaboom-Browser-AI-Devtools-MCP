---
doc_type: feature_index
feature_id: feature-observe
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-05
code_paths:
  - internal/capture/accessors.go
  - internal/queries/dispatcher_queries.go
  - internal/capture/sync.go
  - internal/capture/events.go
  - internal/capture/wsconn/status.go
  - internal/capture/wsconn/store.go
  - internal/capture/wsconn/tracker.go
  - cmd/browser-agent/internal/toolobserve/dispatcher.go
  - cmd/browser-agent/internal/toolobserve/deps.go
  - cmd/browser-agent/internal/toolobserve/response.go
  - cmd/browser-agent/internal/toolobserve/inbox.go
  - cmd/browser-agent/internal/toolobserve/registry.go
  - cmd/browser-agent/internal/toolobserve/page_inventory.go
  - cmd/browser-agent/internal/toolobserve/site_menus.go
  - cmd/browser-agent/internal/toolresp/toolresp.go
  - internal/mcp/response.go
  - cmd/browser-agent/internal/asynccommand/handler.go
  - internal/a11ysummary/summary.go
  - internal/tools/observe/core/deps.go
  - internal/tools/observe/core/filtering.go
  - internal/tools/observe/core/metadata.go
  - internal/tools/observe/logs/logs.go
  - internal/tools/observe/logs/summarized_logs.go
  - internal/types/wire_log.go
  - internal/tools/observe/network/network.go
  - internal/tools/observe/session/session.go
  - internal/tools/observe/timeline/correlation.go
  - internal/tools/observe/page/page_state.go
  - cmd/browser-agent/internal/mcphttp/handler.go
  - internal/tools/observe/hints/hints.go
  - internal/tools/observe/idbquery/execute.go
  - internal/tools/observe/idbquery/scripts.go
  - src/background.ts
  - src/background/commands/observe.ts
  - src/background/commands/helpers.ts
  - src/background/commands/results/element-results.ts
  - src/lib/brand.ts
  - src/lib/page/context.ts
  - src/lib/daemon-http.ts
  - src/content/message-forwarding.ts
  - src/content/message-handlers.ts
  - src/content/runtime-message-listener.ts
  - src/content/window-message-listener.ts
  - src/inject.ts
  - src/inject/observers.ts
  - src/lib/net/network.ts
  - src/lib/net/websocket.ts
  - src/lib/net/websocket-tracking.ts
test_paths:
  - cmd/browser-agent/waterfall_ondemand_test.go
  - internal/tools/observe/timeline/correlation_test.go
  - internal/tools/observe/logs/logs_edge_test.go
  - internal/tools/observe/idbquery/execute_test.go
  - internal/capture/health_reader_owner_test.go
  - tests/extension/dom/command-element-results.test.js
  - tests/extension/dom/page-query-targeting.test.js
  - cmd/browser-agent/lint_hardening_test.go
  - cmd/browser-agent/internal/toolobserve/toolobserve_coverage_test.go
  - cmd/browser-agent/tools_observe_inbox_test.go
  - cmd/browser-agent/tools_observe_handler_test.go
  - cmd/browser-agent/tools_observe_telemetry_modes_test.go
  - cmd/browser-agent/tools_observe_page_readiness_test.go
  - cmd/browser-agent/tools_observe_blackbox_test.go
  - cmd/browser-agent/tools_observe_audit_test.go
  - cmd/browser-agent/tools_observe_screenshot_test.go
  - cmd/browser-agent/tools_observe_analysis_test.go
  - cmd/browser-agent/tools_observe_commands_test.go
  - cmd/browser-agent/tools_observe_indexeddb_test.go
  - extension/background/commands/observe.fullpage.test.js
  - internal/a11ysummary/summary_test.go
  - internal/capture/websocket_test.go
  - internal/capture/websocket_status_test.go
  - internal/capture/websocket_handlers_test.go
  - internal/capture/wsconn/store_test.go
  - internal/tools/observe/logs/logs_test.go
  - internal/tools/observe/core/metadata_test.go
  - internal/tools/observe/network/network_test.go
  - internal/tools/observe/session/session_test.go
  - internal/tools/observe/session/session_transients_test.go
  - internal/tools/observe/timeline/correlation_test.go
  - internal/tools/observe/logs/summarized_logs_test.go
  - internal/tools/observe/contracts/validation_test.go
  - internal/tools/observe/testsupport/helpers.go
  - internal/tools/observe/page/page_state_test.go
  - cmd/browser-agent/internal/mcphttp/handler_test.go
  - internal/tools/observe/page/page_state_storage_test.go
  - internal/tools/observe/page/page_state_screenshot_test.go

  - internal/tools/observe/hints/hints_test.go
  - tests/extension/injection/inject-console-network-exceptions.test.js
  - tests/extension/network-http/network-bodies-fixture.js
  - tests/extension/network-http/network-bodies-xhr.test.js
  - tests/extension/network-http/network-bodies.test.js
  - tests/extension/network-http/network-body-e2e-fixture.js
  - tests/extension/network-http/network-body-e2e.test.js
  - tests/extension/network-realtime/websocket.test.js
  - tests/extension/network-realtime/websocket-tracking.test.js
  - tests/extension/content/content.test.js
  - tests/extension/content/content-message-correlation.test.js
  - tests/extension/capture/observe-waterfall.test.js
  - tests/extension/branding/runtime-log-branding.test.js
  - tests/extension/misc/background-errors-comms.test.js
  - tests/extension/performance/performance.test.js
  - tests/extension/reliability/reliability-fixes.test.js
  - tests/extension/contracts/no-compatibility-facades.test.js
  - tests/extension/sync/sync-client-commands.test.js
  - tests/extension/sync/sync-client-fixture.js
  - tests/extension/sync/sync-client-resilience.test.js
  - tests/extension/sync/sync-client.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Observe

The Go implementation is partitioned by change-coupled observation family.
`core` owns the minimal dependency, metadata, and filtering contracts shared by
those families; `logs`, `network`, `page`, `session`, and `timeline` own their
respective handlers. Callers import those owners directly. The former root
package has been removed rather than retained as a compatibility facade, and
cross-family integration is verified by the `contracts` suite. Shared fixture
construction lives in `testsupport` and is imported only by tests.

WebSocket event retention and the connection state derived from those events
share one independently synchronized `wsconn.Store`. It owns bounded retention,
memory pressure, filtering, detached snapshots, status projection, and atomic
clearing. Observe consumers read that owner directly so event evidence and
connection status cannot race through separate capture facades.

The background service-worker entrypoint owns startup only. Telemetry tests and
runtime code import caches, batching, transport, and log processing directly
from the modules that own those APIs.
On-demand waterfall refresh uses an explicit dependency-owned timeout budget:
production retains the bounded five-second extension wait, while deterministic
tests provide a short budget without sleeping or mutating global timing state.
Network-waterfall freshness crosses that dependency boundary through an
injected clock. Tests place the clock immediately before and exactly at the
one-second threshold, then use the query dispatcher's notification barrier to
deliver extension results. On-demand, empty-buffer, threshold, and concurrent
coverage therefore use no scheduler sleeps; concurrent-response validation also
fails explicitly when a content block is absent instead of accidentally sending
a nil error.
Queued observation modes receive command admission and completion functions
from `internal/asynccommand.Handler`; observe owns no host interface and the
composition root provides no forwarding methods.
The injected page-world entrypoint is also startup-only; observer and telemetry
APIs remain owned by their focused `src/inject` and `src/lib` modules.
Go observe modes receive the canonical capture owner plus explicit log, noise,
accessibility, and diagnostic reads. No ToolHandler-satisfied observation
interface or observation-only root getter remains.
Page titles and URLs containing raw control bytes are escaped through both the
nested tool payload and outer JSON-RPC envelope. The HTTP boundary rejects any
malformed raw result with a structured protocol error instead of returning an
empty or partial response.
On-demand waterfall capture distinguishes a confirmed empty page from inject
bridge timeout, rejection, and dispatch failure. Failures remain structured
through background dispatch and activate a redacted Doctor diagnostic; the next
authenticated response resolves that incident even when it contains no entries.
Failed-command projection tests transition commands explicitly; deadline
scheduling remains owned by the query-dispatcher suite rather than a sleep in
the observe adapter test.
IndexedDB responders block on the canonical pending-query notification and
complete each state or execute query exactly once. They no longer poll the
queue on a wall-clock interval.

## TL;DR
- Status: shipped
- Tool: `observe`
- Mode key: `what`
- Contract source: `internal/schema/observe.go`

## Specs
- Product: `product-spec.md`
- Tech: `tech-spec.md`
- QA: `qa-plan.md`
- Flow Map: `flow-map.md`

## Canonical Note
`observe` is the passive read surface for captured browser/server state. It is the canonical polling surface for async command completion via `what:"command_result"`.

Cross-owner runtime health is read through `capture.HealthReader`; no aggregate
health facade remains on the `Capture` composition root.
Element collection, visibility filtering, limits, and tab metadata use the
shared command helpers also consumed by `interact-explore`; viewport screenshot
capture/upload has one implementation for both normal capture and CDP fallback.
Live-page storage, IndexedDB, screenshot, and accessibility failures use the
canonical `mcp.Fail` response boundary; queue saturation guidance is built once
with the canonical `pending_commands` recovery call.
Live-page command tests wait on the query dispatcher's canonical pending-query
notification before returning extension results; they do not poll queue state.
Screenshot response tests share that notification helper for inline JPEG/PNG,
text-only, save-to, and validation flows. Completion remains bounded by an
independent failure timeout, but query creation is never inferred from elapsed
wall-clock time.

Tool dispatch uses only the canonical `what` selector and canonical mode names;
`mode`, `action`, `network`, and `ws` routing shortcuts are not accepted.
The `site_menus` composite translates that public `what` contract into the
canonical internal DOM-command `action` wire field before dispatching
`list_interactive`; public MCP selectors never leak into extension commands.

Accessibility (`what:"accessibility"`) normalizes `summary` counts to the
canonical keys `violations`, `passes`, `incomplete`, and `inapplicable`.
Legacy `*_count` compatibility fields are not part of the contract.
WebSocket status (`what:"websocket_status"`) supports `summary:true` with compact URL/connection-id previews while preserving the full default payload when `summary` is omitted.
Sampled WebSocket capture snapshots mutable `ArrayBuffer` payloads and the exact
byte range of outbound typed-array views before deferred formatting, preserving
the wire bytes even when application code mutates or transfers the source buffer.
Network-bodies empty-result hints now echo all active filters (`url`, `method`, `status_*`, `body_path`) so retry guidance is specific to the current query.
Log severity filtering uses only `min_level`, with threshold semantics (for
example, `warn` returns warning and error entries).
Storage summary tests now share common assertions for `key_count`, `sample_keys`, and `total_bytes` shape checks.
If the extension reloads while an old content script is still attached to the page, the bridge now emits a Kaboom-branded refresh warning and stops retrying dead `chrome.runtime.sendMessage` calls until the page is refreshed.
Context-annotation warnings and background-sender rejection logs now use the shared Kaboom runtime prefix instead of hardcoded Kaboom labels.
Enhanced action capture now crosses the page/content boundary through the Kaboom-branded `kaboom_enhanced_action` postMessage contract before being normalized to background `enhanced_action` events.
The early-patch adoption globals used before the inject bundle loads are now Kaboom-scoped (`__KABOOM_ORIGINAL_*`, `__KABOOM_EARLY_*`) across the fetch/XHR/WebSocket bridge.
