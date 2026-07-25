---
doc_type: flow_map
flow_id: interact-action-toast-label-normalization
status: active
last_reviewed: 2026-03-05
owners:
  - Brenn
entrypoints:
  - src/background/commands/helpers.ts:actionToast
code_paths:
  - src/background/commands/helpers.ts
  - src/background/dom/dom-dispatch.ts
  - src/background/browser-actions.ts
  - src/background/dom/cdp/cdp-dispatch.ts
test_paths:
  - tests/extension/action-toast-labels.test.js
  - tests/extension/wait-for-enhanced.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Interact Action Toast Label Normalization

## Scope

Covers how user-facing action toast labels are normalized for `interact` actions so internal enum-style values (for example `wait_for_stable`) are never shown raw in the overlay.

## Entrypoints

1. `actionToast(...)` in `src/background/commands/helpers.ts` sends `KABOOM_ACTION_TOAST` to the content overlay.
2. `executeDOMAction(...)` and `handleBrowserAction(...)` provide action labels (`reason` or `action`) to `actionToast`.
3. `executeCDPAction(...)` emits action-specific copy for CDP toasts (`key_press` -> `Typing...`) while preserving existing `CDP <action>` labels for other CDP actions.

## Primary Flow

1. Caller passes `action` string to `actionToast`.
2. `actionToast` resolves display text through `resolveToastCopy(...)`.
3. For `trying` state, explicit progressive labels are preferred (`scroll_to` -> `Scrolling to`, `dismiss_top_overlay` -> `Dismissing top overlay`).
4. `wait_for` in `trying` state infers the target from `detail` (`#result-panel` -> `Waiting for #result-panel`) and suppresses duplicate detail text.
5. Outside `trying` state, explicit map entries and snake_case humanization still apply (`wait_for_stable` -> `Waiting for page to stabilize...`, `switch_tab` -> `Switch tab`).
6. Content script renders the final toast string without exposing raw enum keys.

## Error and Recovery Paths

1. If toast delivery fails (`chrome.tabs.sendMessage` rejection), the error is swallowed to avoid disrupting command completion.
2. If an unmapped action arrives, fallback humanization still avoids raw snake_case leakage.
3. If `wait_for` lacks inferable detail, `trying` state falls back to `Waiting for condition...`.

## State and Contracts

1. `PRETTY_LABELS` is the source of truth for explicit generic labels.
2. `PRETTY_TRYING_LABELS` is the source of truth for progressive in-flight copy.
3. `humanizeActionLabel` only transforms lowercase snake_case tokens; non-enum text stays unchanged.
4. `reason` overrides from callers still flow through normalization before rendering.

## Code Paths

- `src/background/commands/helpers.ts`
- `src/background/dom/dom-dispatch.ts`
- `src/background/browser-actions.ts`
- `src/background/dom/cdp/cdp-dispatch.ts`

## Test Paths

- `tests/extension/action-toast-labels.test.js`
- `tests/extension/wait-for-enhanced.test.js`

## Edit Guardrails

1. Keep explicit copy in `PRETTY_LABELS` for high-visibility statuses or nuanced phrasing.
2. Keep fallback normalization constrained to enum-like snake_case values to avoid rewriting user-supplied prose.
3. Add/adjust toast-label tests whenever new actions are added to interact/browser dispatch flows.
