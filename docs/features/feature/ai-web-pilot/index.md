---
doc_type: feature_index
feature_id: feature-ai-web-pilot
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-11-21
code_paths:
  - internal/tools/guard/guards.go
  - internal/composition/toolruntime/tools_core.go
  - internal/tools/interact/handler/action_owners.go
  - src/popup.ts
  - src/popup/ai-web-pilot.ts
  - scripts/templates/partials/_dom-selectors.tpl
  - src/background/dom/primitives/dom-primitives-form.ts
  - src/background/dom/primitives/dom-primitives-intent.ts
  - src/background/dom/primitives/dom-primitives-overlay.ts
  - src/background/dom/primitives/dom-primitives-pointer.ts
  - src/background/dom/primitives/dom-primitives-read.ts
  - src/background/commands/interact.ts
  - src/types/runtime-messages.ts
  - src/content/window-message-listener.ts
  - src/inject/message-handlers.ts
  - src/inject/state.ts
  - src/content/ui/panel/shell.ts
  - src/content/ui/subtitle.ts
  - src/content/ui/toast.ts
  - src/content/ui/tracked-hover-launcher.ts
  - internal/sync/runtime/tracking_projection.go
test_paths:
  - internal/sync/runtime/extension_state_unit_test.go
  - internal/tools/asynccommand/execute_test.go
  - internal/tools/guard/guards_test.go
  - internal/tools/interact/handler/contracts/gates_test.go
  - internal/tools/asynccommand/handler_test.go
  - tests/extension/pilot/pilot-toggle.test.js
  - tests/extension/pilot/pilot-command-response.test.js
  - tests/extension/content/content-message-correlation.test.js
  - tests/extension/contracts/no-compatibility-facades.test.js
  - tests/extension/dom/dom-primitives-branding.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Ai Web Pilot

## TL;DR

- Status: shipped
- Tool: interact
- Mode/Action: navigate, execute_js, highlight
- Location: `docs/features/feature/ai-web-pilot`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_AI_WEB_PILOT_001
- FEATURE_AI_WEB_PILOT_002
- FEATURE_AI_WEB_PILOT_003

## Code and Tests

Pilot, extension-connectivity, tracked-tab, and CSP preconditions are owned by
`internal/tools/guard`. Tool adapters receive those canonical guard methods
directly; package-main no longer carries a duplicate guard surface.
The popup's batched storage read calls `applyAiWebPilotToggle` directly. There
is no self-loading compatibility initializer or second storage-read path.
DOM actions distinguish Kaboom-owned overlays through the explicit
`data-kaboom-owned="true"` marker. Page IDs and classes that happen to begin
with `kaboom-` remain ordinary, targetable application DOM.
Pilot command delivery requires an explicit terminal response from the content
script. Missing or falsy responses return `pilot_command_no_response` and emit
a redacted local diagnostic; they can never be converted into success.
All page-bridge request and response literals live in the canonical runtime
message contract and require the page nonce. Highlight dispatch shares the one
authenticated inject dispatcher with DOM, accessibility, state, computed-style,
and form queries; state ownership contains no second listener or nonce reader.

## Pilot state and tracked-tab ordering

A `/sync` heartbeat always applies its Pilot state. That holds even when the
daemon drops the heartbeat's tracked-tab fields as stale after a synchronous
`switch_tab` (see the ordering rule in
[tab-tracking-ux](../tab-tracking-ux/index.md)). Command eligibility does not
change: Pilot and extension guards still run before every command.
