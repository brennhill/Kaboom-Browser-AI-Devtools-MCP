---
doc_type: feature_index
feature_id: feature-interact-explore
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - cmd/browser-agent/internal/toolinteract/deps.go
  - cmd/browser-agent/internal/toolguard/guards.go
  - cmd/browser-agent/internal/toolinteract/interactstate/state.go
  - cmd/browser-agent/internal/toolinteract/interactupload/upload.go
  - cmd/browser-agent/internal/toolinteract/interact_action_handler.go
  - cmd/browser-agent/internal/toolinteract/interact_command_builder.go
  - cmd/browser-agent/internal/toolinteract/interact_browser.go
  - cmd/browser-agent/internal/toolinteract/interact_dom.go
  - cmd/browser-agent/internal/toolinteract/interact_storage.go
  - cmd/browser-agent/internal/toolinteract/interact_page.go
  - cmd/browser-agent/internal/toolinteract/interact_workflow.go
  - cmd/browser-agent/internal/toolinteract/interact_evidence.go
  - cmd/browser-agent/internal/toolinteract/interact_batch.go
  - cmd/browser-agent/internal/toolinteract/elemindex/registry.go
  - cmd/browser-agent/tools_core.go
  - cmd/browser-agent/tools_interact_dispatch.go
  - cmd/browser-agent/tools_async_completion.go
  - internal/tools/interact/workflow.go
  - internal/schema/interact/actions.go
  - internal/schema/interact/properties_targeting.go
  - internal/schema/interact/properties_output_batch.go
  - internal/tools/configure/capabilities/modespecs_interact.go
  - scripts/docs/reference/check-reference-schema-sync.mjs
  - src/background.ts
  - src/background/pending-queries.ts
  - src/background/exec/query-execution.ts
  - src/background/commands/helpers.ts
  - src/background/exec/browser-actions.ts
  - src/background/dom/cdp/cdp-dispatch.ts
  - src/background/dom/dom-dispatch.ts
  - src/background/exec/frame-targeting.ts
  - src/background/exec/content-fallback-scripts.ts
  - src/background/exec/upload-handler.ts
  - src/lib/daemon-http.ts
  - src/background/ui/draw-mode-toggle.ts
  - src/background/dom/dom-types.ts
  - src/background/dom/primitives/dom-primitives-pointer.ts
  - src/background/dom/primitives/dom-primitives-form.ts
  - src/background/dom/primitives/dom-primitives-read.ts
  - src/inject.ts
  - src/inject/execute-js.ts
  - src/content/runtime-message-listener.ts
  - src/background/dom/primitives/dom-primitives-list-interactive.ts
  - src/background/dom/primitives/dom-primitives-intent.ts
  - src/background/dom/primitives/dom-primitives-overlay.ts
  - src/background/dom/primitives/dom-primitives-stability.ts
  - scripts/templates/partials/_dom-intent.tpl
  - scripts/templates/partials/_dom-selectors.tpl
  - scripts/templates/dom-primitives.ts.tpl
  - cmd/browser-agent/internal/asyncresult/normalization.go
  - cmd/browser-agent/internal/asyncresult/enrichment.go
  - cmd/browser-agent/internal/asyncresult/enrichment_csp.go
  - cmd/browser-agent/internal/asyncresult/enrichment_recovery.go
  - cmd/browser-agent/internal/asyncresult/lifecycle.go
  - cmd/browser-agent/tools_async_completion.go
  - cmd/browser-agent/internal/summarypref/cache.go
  - cmd/browser-agent/tools_core.go
test_paths:
  - cmd/browser-agent/internal/summarypref/cache_test.go
  - cmd/browser-agent/tools_summary_pref_test.go
  - cmd/browser-agent/internal/toolinteract/fake_deps_test.go
  - cmd/browser-agent/internal/toolinteract/test_helpers_test.go
  - cmd/browser-agent/internal/toolinteract/interact_browser_actions_test.go
  - cmd/browser-agent/internal/toolinteract/interact_dom_primitive_test.go
  - cmd/browser-agent/internal/toolinteract/interact_elements_test.go
  - cmd/browser-agent/internal/toolinteract/interact_page_test.go
  - cmd/browser-agent/internal/toolinteract/interact_storage_test.go
  - cmd/browser-agent/internal/toolinteract/interact_workflow_test.go
  - cmd/browser-agent/internal/toolinteract/elemindex/registry_test.go
  - cmd/browser-agent/tools_interact_handler_test.go
  - cmd/browser-agent/tools_interact_dom_params_test.go
  - cmd/browser-agent/tools_interact_workflows_test.go
  - cmd/browser-agent/tools_pending_query_enqueue_test.go
  - cmd/browser-agent/tools_interact_rich_test.go
  - cmd/browser-agent/tools_interact_navigate_document_test.go
  - cmd/browser-agent/tools_schema_parity_test.go
  - internal/schema/interact/schema_test.go
  - cmd/browser-agent/tools_interact_evidence_test.go
  - cmd/browser-agent/tools_interact_state_test.go
  - cmd/browser-agent/internal/toolinteract/interactstate/state_test.go
  - extension/background/__tests__/dom-dispatch-structured.test.js
  - tests/extension/dom-primitives-branding.test.js
  - tests/extension/dom-action-family-routing.test.js
  - tests/extension/action-toast-labels.test.js
  - tests/extension/execute-js.test.js
  - internal/tools/interact/workflow_test.go
  - internal/tools/configure/capabilities/modespecs_test.go
  - tests/extension/toggle-overlay.test.js
  - cmd/browser-agent/internal/asyncresult/asyncresult_test.go
  - cmd/browser-agent/tools_async_formatting_test.go
  - tests/extension/interact-content-fallback.test.js
  - tests/extension/async-timeout.test.js
  - tests/extension/pending-query-targeting.test.js
  - tests/extension/pilot-execute.test.js
  - tests/extension/pilot-state.test.js
  - tests/extension/pilot-toggle.test.js
  - tests/extension/no-compatibility-facades.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Interact Tool

The background service-worker entrypoint owns startup only. Interaction
consumers import state, command, snapshot, and query APIs from their focused
owner modules; no compatibility facade is retained.
Page-world interaction tests import action, state, serialization, and message
handler modules directly; the injected bundle is not an API surface.
Screenshot capture belongs only to `observe({what:"screenshot"})`; the former
`interact` screenshot compatibility action has been removed.
State snapshot handlers accept only the canonical `snapshot_name` parameter;
the former generic `name` request alias has been removed.
Public state actions likewise use only `save_state`, `load_state`,
`list_states`, and `delete_state`; duplicate `state_*` entry points are not
registered. The similarly named extension pending-query types remain internal.

## TL;DR
- Status: shipped
- Tool: `interact`
- Mode key: `what`
- Contract source: `internal/schema/interact/tool.go`

## Specs
- Product: `product-spec.md`
- Tech: `tech-spec.md`
- QA: `qa-plan.md`
- Flow Map Pointer: `flow-map.md`

## Canonical Note
This feature documents the shipped `interact` action surface (not a batched `interact.explore` action).

The generated pointer, form, and read primitive modules intentionally contain
the same selector and result machinery. Chrome serializes each injected
function independently, so imports or shared closures would fail at runtime.
Their duplication is generated from one template; only the action handlers
differ. This is the intentional `jscpd` exception for these three files.

`get_text` supports `structured:true` for hierarchical extraction (for example accordion/list sections), and this option must be forwarded through DOM dispatch into extension primitives.

`execute_js` host-object serialization must preserve prototype-backed values (for example `DOMRect`) so return payloads remain structured and parse-safe.

`navigate_and_document` combines click-driven navigation, optional URL-change/stability waits, and page-context enrichment (`url`, `title`, `tab_id`) in a single interact workflow.

Workflow types and response classification come directly from
`internal/tools/interact/workflow.go`; browser-agent layers do not maintain
aliases or pass-through response helpers.

Evidence capture uses the concrete
`toolinteract.EvidenceShot` contract directly in runtime wiring and tests; the
former private/exported type alias and root-package test shim have been deleted.

Pilot, extension, and tab preconditions use the canonical `toolguard.Check`
contract; interact packages do not mirror the host guard signature.

Interact handlers and their dependency seams use `internal/mcp` and
`internal/toolresp` directly. Package-local protocol type, error-code, and
structured-error option aliases are prohibited so the canonical contract stays
visible at every call site.

`navigate_and_document` now returns structured metadata for machine consumers:
1. `metadata.page_context` (`url`, `title`, `tab_id`) while preserving the legacy text block.
2. `metadata.workflow_trace` (`trace_id`, `status`, stage-level timing/status envelope).
3. Explicit `tab_id` now requires an actively tracked tab and must match tracked context before click dispatch.

Interact action metadata now has a single canonical registry in `internal/schema/interact/actions.go`, consumed by both schema enum generation and `describe_capabilities` mode specs.

Extension-dispatched interact actions now use shared enqueue fail-fast handling: when queue capacity is saturated, responses return structured `queue_full` immediately rather than entering async wait mode.

Async execution is controlled only by the canonical `background` parameter.
The entrypoint does not translate alternate parameter names.

All MCP tools route exclusively through `what`; `interact` does not accept
`action` as a selector.
