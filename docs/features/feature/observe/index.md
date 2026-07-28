---
doc_type: feature_index
feature_id: feature-observe
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - internal/capture/accessors.go
  - internal/queries/dispatcher_queries.go
  - internal/capture/sync.go
  - internal/capture/events.go
  - internal/capture/wsconn/status.go
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
  - internal/tools/observe/deps.go
  - internal/tools/observe/filtering.go
  - internal/tools/observe/metadata.go
  - internal/tools/observe/logs.go
  - internal/tools/observe/summarized_logs.go
  - internal/types/log.go
  - internal/tools/observe/network.go
  - internal/tools/observe/session.go
  - internal/tools/observe/correlation.go
  - internal/tools/observe/page_state.go
  - internal/tools/observe/hints/hints.go
  - internal/tools/observe/idbquery/execute.go
  - internal/tools/observe/idbquery/scripts.go
  - src/background.ts
  - src/background/commands/observe.ts
  - src/lib/brand.ts
  - src/lib/page/context.ts
  - src/lib/daemon-http.ts
  - src/content/message-forwarding.ts
  - src/content/runtime-message-listener.ts
  - src/content/window-message-listener.ts
  - src/inject.ts
  - src/inject/observers.ts
  - src/lib/net/network.ts
test_paths:
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
  - extension/background/commands/observe.fullpage.test.js
  - internal/a11ysummary/summary_test.go
  - internal/capture/websocket_test.go
  - internal/capture/websocket_status_test.go
  - internal/capture/websocket_handlers_test.go
  - internal/tools/observe/logs_test.go
  - internal/tools/observe/metadata_test.go
  - internal/tools/observe/network_test.go
  - internal/tools/observe/session_test.go
  - internal/tools/observe/session_transients_test.go
  - internal/tools/observe/correlation_test.go
  - internal/tools/observe/summarized_logs_test.go
  - internal/tools/observe/validation_test.go
  - internal/tools/observe/page_state_test.go
  - internal/tools/observe/page_state_storage_test.go
  - internal/tools/observe/page_state_screenshot_test.go
  - internal/tools/observe/hints/hints_test.go
  - tests/extension/inject-console-network-exceptions.test.js
  - tests/extension/network-bodies.test.js
  - tests/extension/content.test.js
  - tests/extension/runtime-log-branding.test.js
  - tests/extension/background-errors-comms.test.js
  - tests/extension/performance.test.js
  - tests/extension/reliability-fixes.test.js
  - tests/extension/no-compatibility-facades.test.js
  - tests/extension/sync-client-commands.test.js
  - tests/extension/sync-client-fixture.js
  - tests/extension/sync-client-resilience.test.js
  - tests/extension/sync-client.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Observe

The background service-worker entrypoint owns startup only. Telemetry tests and
runtime code import caches, batching, transport, and log processing directly
from the modules that own those APIs.
Queued observation modes receive command admission and completion functions
from `internal/asynccommand.Handler`; observe owns no host interface and the
composition root provides no forwarding methods.
The injected page-world entrypoint is also startup-only; observer and telemetry
APIs remain owned by their focused `src/inject` and `src/lib` modules.
Go observe modes receive the canonical capture owner plus explicit log, noise,
accessibility, and diagnostic reads. No ToolHandler-satisfied observation
interface or observation-only root getter remains.

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

Tool dispatch uses only the canonical `what` selector and canonical mode names;
`mode`, `action`, `network`, and `ws` routing shortcuts are not accepted.

Accessibility (`what:"accessibility"`) normalizes `summary` counts to the
canonical keys `violations`, `passes`, `incomplete`, and `inapplicable`.
Legacy `*_count` compatibility fields are not part of the contract.
WebSocket status (`what:"websocket_status"`) supports `summary:true` with compact URL/connection-id previews while preserving the full default payload when `summary` is omitted.
Network-bodies empty-result hints now echo all active filters (`url`, `method`, `status_*`, `body_path`) so retry guidance is specific to the current query.
Log severity filtering uses only `min_level`, with threshold semantics (for
example, `warn` returns warning and error entries).
Storage summary tests now share common assertions for `key_count`, `sample_keys`, and `total_bytes` shape checks.
If the extension reloads while an old content script is still attached to the page, the bridge now emits a Kaboom-branded refresh warning and stops retrying dead `chrome.runtime.sendMessage` calls until the page is refreshed.
Context-annotation warnings and background-sender rejection logs now use the shared Kaboom runtime prefix instead of hardcoded Kaboom labels.
Enhanced action capture now crosses the page/content boundary through the Kaboom-branded `kaboom_enhanced_action` postMessage contract before being normalized to background `enhanced_action` events.
The early-patch adoption globals used before the inject bundle loads are now Kaboom-scoped (`__KABOOM_ORIGINAL_*`, `__KABOOM_EARLY_*`) across the fetch/XHR/WebSocket bridge.
