---
doc_type: feature_index
feature_id: feature-observe
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-09-05
code_paths:
  - internal/screenshotframe/wire_screenshot.go
  - src/lib/screenshot/coordinate-frame.ts
  - src/lib/screenshot/image-size.ts
  - src/background/commands/results/screenshot-delivery.ts
  - src/background/commands/observe.ts
  - src/background/ui/tracked-tab-state.ts
  - cmd/browser-agent/internal/mediaapi/screenshots.go
  - internal/capture/healthreader/reader.go
  - internal/queries/dispatcher_queries.go
  - internal/capture/syncruntime/handler.go
  - internal/capture/telemetrystore/store.go
  - internal/capture/waterfallstore/store.go
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
  - cmd/browser-agent/internal/telemetryapi/handler.go
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
  - src/background/ui/tracked-tab-state.ts
  - src/background/dom/cdp/cdp-session.ts
  - src/background/sync/screenshot.ts
  - src/background/message-routing/capture-handler.ts
  - src/background/push-handler.ts
  - src/lib/tabs/tab-focus.ts
test_paths:
  - internal/screenshotframe/wire_screenshot_test.go
  - cmd/browser-agent/internal/mediaapi/screenshot_frame_test.go
  - tests/extension/capture/screenshot-coordinate-frame.test.js
  - tests/extension/capture/background/background-tab-capture.test.js
  - tests/extension/capture/background/visible-tab-capture-fallback.test.js
  - cmd/browser-agent/internal/telemetryapi/handler_test.go
  - internal/tools/observe/core/filtering_test.go
  - internal/tools/observe/network/network_test.go
  - internal/tools/observe/timeline/correlation_test.go
  - internal/tools/observe/logs/logs_edge_test.go
  - internal/tools/observe/idbquery/execute_test.go
  - internal/capture/healthreader/reader_test.go
  - tests/extension/dom/command-element-results.test.js
  - tests/extension/dom/page-query-targeting.test.js
  - scripts/contracts/goarchitecturetests/contracts_test.go
  - cmd/browser-agent/internal/toolobserve/toolobserve_coverage_test.go
  - internal/tools/observe/page/page_readiness_test.go
  - cmd/browser-agent/internal/toolobserve/dispatcher_commands_test.go
  - internal/tools/observe/idbquery/execute_test.go
  - extension/background/commands/observe.fullpage.test.js
  - internal/a11ysummary/summary_test.go
  - internal/capture/websockettest/websocket_test.go
  - internal/capture/websockettest/websocket_status_test.go
  - internal/capture/websockettest/websocket_handlers_test.go
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
  - internal/capture/waterfallstore/store_test.go
  - tests/extension/capture/observe-screenshot.test.js
  - tests/extension/capture/background/background-tab-capture.test.js
  - tests/extension/capture/background/visible-tab-capture-fallback.test.js
  - extension/background/__tests__/cdp-session.test.js
  - tests/extension/content/overlay-capture-stripping.test.js

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

Cross-owner runtime health is read through `healthreader.Reader`; no aggregate
health facade remains on the `Capture` composition root.
The local `/telemetry` read endpoint is owned by `internal/telemetryapi`, which
applies one generic bounded-tail policy across every buffer family. The root
server only composes that handler and contains no telemetry switch or query
parsing logic.
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
Error-bundle response, timestamp, limit, and correlation-window contracts live
with the timeline owner; the browser-agent root does not duplicate its capture
and log fixtures.
Freshness metadata is verified at its canonical builder and at the page, log,
network, and session handlers that select each stream's newest timestamp.
Tracked-tab activity is likewise owned by the page-response contract, including
the distinction between a known inactive tab and unknown activity state.
Web-vital, history, error-cluster, screenshot, and accessibility edge contracts
are tested by their session, log, and page owners rather than by a root-level
coverage harness coupled to the browser-agent composition fixture.
Canonical response fields and filters for errors, browser logs, extension logs,
network bodies, WebSockets, and actions are likewise asserted by their stream
owners; telemetry modes have no parallel root-level response-shape suite.
Cross-mode response contracts are distributed to their canonical stream,
command-state, and recording owners. This keeps dispatcher tests focused on
routing and server-side projections instead of recreating every feature state.
`pending_commands` always returns JSON arrays for pending, completed, failed,
and extension-owned work, including when an attached command store has no
entries; clients never need to distinguish an empty list from `null`.
Composition-root MCP routing remains covered by the shared browser-agent
integration suite. Stream ingestion and response behavior are tested at their
HTTP and observe owners, avoiding root fixtures that only seeded internal state
while describing themselves as browser-to-MCP black-box tests.

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
Network and WebSocket empty-result, prospective-capture, and compact-summary
contracts live with the canonical network observe owner; the browser-agent root
does not maintain a duplicate integration fixture for those response shapes.
Log severity filtering uses only `min_level`, with threshold semantics (for
example, `warn` returns warning and error entries).
Storage summary tests now share common assertions for `key_count`, `sample_keys`, and `total_bytes` shape checks.
If the extension reloads while an old content script is still attached to the page, the bridge now emits a Kaboom-branded refresh warning and stops retrying dead `chrome.runtime.sendMessage` calls until the page is refreshed.
Context-annotation warnings and background-sender rejection logs now use the shared Kaboom runtime prefix instead of hardcoded Kaboom labels.
Enhanced action capture now crosses the page/content boundary through the Kaboom-branded `kaboom_enhanced_action` postMessage contract before being normalized to background `enhanced_action` events.
The early-patch adoption globals used before the inject bundle loads are now Kaboom-scoped (`__KABOOM_ORIGINAL_*`, `__KABOOM_EARLY_*`) across the fetch/XHR/WebSocket bridge.

## Background capture

`observe({what:"screenshot"})` captures a tab the user is not looking at. The image comes from
`Page.captureScreenshot` over the tab's persistent CDP lease, clipped to the visual viewport
(`Page.getLayoutMetrics.cssVisualViewport`) and scaled by the page's device pixel ratio so the
result matches what `chrome.tabs.captureVisibleTab` used to produce. No tab is activated, so a
screenshot no longer pulls the browser window away from the person using it and no longer drops
the focus out of whatever they were typing.

**CDP is only used for tabs the user is not looking at.** If the target is already the active
tab in its window, the capture goes straight through `chrome.tabs.captureVisibleTab` and no
debugger is attached — those are the same pixels, and attaching would raise Chrome's *"Kaboom is
debugging this browser"* infobar over the user's own browsing for the lease's idle grace. That
matters because `screenshot_on_error` fires on any page error, so the banner would otherwise
appear unprompted while someone is simply using their browser. A `captureVisibleTab` that fails
on an active tab is reported and then falls through to CDP rather than being treated as "this
tab is backgrounded" — those are different facts and only the second is expected.

`chrome.tabs.captureVisibleTab` is also the fallback when CDP is unreachable. That API can only
photograph the visible tab, so in that case it still activates the target and hands the
foreground straight back. Every fallback is reported, and the report separates the recoverable
cases from the defect:

| Reason | Meaning | Signal |
| --- | --- | --- |
| `no_debugger_api` | this context has no `chrome.debugger` at all | `debugLog` |
| `session_unavailable` | a performance trace holds the tab exclusively, the drain timed out, the attach was refused (restricted page, user cancelled the debugging banner), or the session was invalidated | `debugLog` |
| `cdp_capture_failed` | we hold the lease and `Page.captureScreenshot` still failed | `console.warn` + `debugLog` |

The third row is the one that matters: it is a real failure that just took the user's
foreground, so it is loud in the service-worker console even with extension debug logging off.
Reporting it the same way as the first two is how a broken lease hides behind tabs that merely
flicker.

Kaboom's own overlays are stripped by the `data-kaboom-overlay` marker attribute across both
paths, so a screenshot never contains the supervision badge or phantom cursor Kaboom drew.

Taking the foreground is now an explicit request rather than a side effect of capturing:
`activate_tab`, a popup click, and a screen recording that needs a user gesture. Draw mode and
the `push_screenshot` shortcut keep using the visible-tab API because both act on the tab
already in front of the user.
