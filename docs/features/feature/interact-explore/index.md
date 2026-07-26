---
doc_type: feature_index
feature_id: feature-interact-explore
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-27
code_paths:
  - cmd/browser-agent/internal/toolinteract/deps.go
  - cmd/browser-agent/internal/toolinteract/helpers.go
  - cmd/browser-agent/internal/toolinteract/interact_action_handler.go
  - cmd/browser-agent/internal/toolinteract/interact_command_builder.go
  - cmd/browser-agent/internal/toolinteract/interact_browser.go
  - cmd/browser-agent/internal/toolinteract/interact_dom.go
  - cmd/browser-agent/internal/toolinteract/interact_elements.go
  - cmd/browser-agent/internal/toolinteract/interact_page.go
  - cmd/browser-agent/internal/toolinteract/interact_storage.go
  - cmd/browser-agent/internal/toolinteract/interact_workflow.go
  - cmd/browser-agent/internal/toolinteract/interact_evidence.go
  - cmd/browser-agent/internal/toolinteract/interact_retry_contract.go
  - cmd/browser-agent/internal/toolinteract/interact_batch.go
  - cmd/browser-agent/internal/toolinteract/elemindex/registry.go
  - cmd/browser-agent/tools_interact_adapter.go
  - cmd/browser-agent/tools_interact_entrypoint.go
  - cmd/browser-agent/tools_interact_dispatch.go
  - cmd/browser-agent/tools_pending_query_enqueue.go
  - internal/tools/interact/workflow.go
  - internal/schema/interact/actions.go
  - internal/schema/interact/properties_targeting.go
  - internal/schema/interact/properties_output_batch.go
  - internal/tools/configure/capabilities/modespecs_interact.go
  - scripts/docs/reference/check-reference-schema-sync.mjs
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
  - src/background/dom/primitives/dom-primitives.ts
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
  - cmd/browser-agent/tools_async_formatting.go
  - cmd/browser-agent/tools_summary_pref.go
test_paths:
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
  - cmd/browser-agent/tools_pending_query_enqueue_test.go
  - cmd/browser-agent/tools_interact_rich_test.go
  - cmd/browser-agent/tools_interact_navigate_document_test.go
  - cmd/browser-agent/tools_schema_parity_test.go
  - cmd/browser-agent/tools_interact_evidence_test.go
  - cmd/browser-agent/tools_interact_state_test.go
  - extension/background/__tests__/dom-dispatch-structured.test.js
  - extension/background/dom-primitives.test.js
  - tests/extension/action-toast-labels.test.js
  - tests/extension/execute-js.test.js
  - internal/tools/interact/workflow_test.go
  - internal/tools/configure/capabilities/modespecs_test.go
  - extension/background/dom-primitives-overlay.test.js
  - cmd/browser-agent/internal/asyncresult/asyncresult_test.go
  - cmd/browser-agent/tools_async_formatting_test.go
  - tests/extension/interact-content-fallback.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Interact Tool

## TL;DR
- Status: shipped
- Tool: `interact`
- Mode key: `what` (deprecated alias: `action`)
- Contract source: `cmd/browser-agent/tools_schema.go`

## Specs
- Product: `product-spec.md`
- Tech: `tech-spec.md`
- QA: `qa-plan.md`
- Flow Map Pointer: `flow-map.md`

## Canonical Note
This feature documents the shipped `interact` action surface (not a batched `interact.explore` action).

`get_text` supports `structured:true` for hierarchical extraction (for example accordion/list sections), and this option must be forwarded through DOM dispatch into extension primitives.

`execute_js` host-object serialization must preserve prototype-backed values (for example `DOMRect`) so return payloads remain structured and parse-safe.

`navigate_and_document` combines click-driven navigation, optional URL-change/stability waits, and page-context enrichment (`url`, `title`, `tab_id`) in a single interact workflow.

`navigate_and_document` now returns structured metadata for machine consumers:
1. `metadata.page_context` (`url`, `title`, `tab_id`) while preserving the legacy text block.
2. `metadata.workflow_trace` (`trace_id`, `status`, stage-level timing/status envelope).
3. Explicit `tab_id` now requires an actively tracked tab and must match tracked context before click dispatch.

Interact action metadata now has a single canonical registry in `internal/schema/interact/actions.go`, consumed by both schema enum generation and `describe_capabilities` mode specs.

Extension-dispatched interact actions now use shared enqueue fail-fast handling: when queue capacity is saturated, responses return structured `queue_full` immediately rather than entering async wait mode.
