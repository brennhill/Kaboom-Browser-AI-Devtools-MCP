---
doc_type: flow_map
flow_id: interact-overlay-selector-improvements
status: active
last_reviewed: 2026-03-06
owners:
  - Brenn
entrypoints:
  - scripts/templates/partials/_dom-overlay-helpers.tpl:findTopmostOverlay
  - src/background/dom/dom-primitives-overlay.ts:domPrimitiveOverlay
  - scripts/templates/partials/_dom-selectors.tpl:resolveByText
  - cmd/browser-agent/tools_interact_dom.go:normalizeDOMActionArgs
  - cmd/browser-agent/tools_async_formatting.go:formatCompleteCommand
  - src/background/dom/dom-primitives-list-interactive.ts:domPrimitiveListInteractive
code_paths:
  - scripts/templates/partials/_dom-overlay-helpers.tpl
  - scripts/templates/partials/_dom-selectors.tpl
  - scripts/templates/dom-primitives.ts.tpl
  - src/background/dom/dom-primitives-overlay.ts
  - src/background/dom/dom-primitives-list-interactive.ts
  - src/background/dom/dom-dispatch.ts
  - src/background/dom/dom-types.ts
  - cmd/browser-agent/tools_interact_dom.go
  - cmd/browser-agent/tools_async_formatting.go
  - cmd/browser-agent/tools_async_result_normalization.go
  - cmd/browser-agent/tools_summary_pref.go
test_paths:
  - extension/background/dom-primitives-overlay.test.js
  - cmd/browser-agent/tools_interact_handler_test.go
  - cmd/browser-agent/tools_async_enrich_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Interact Overlay & Selector Improvements

## Scope

Covers overlay dismiss loop detection (#444), cross-extension overlay detection (#445), text= selector auto-resolve to interactive child (#443), interact response verbosity control (#447), and region-scoped element discovery (#448).

## Primary Flows

### 1. Overlay Dismiss Loop Detection (#444)

1. LLM calls `interact({what: "dismiss_top_overlay"})`.
2. `dom-dispatch.ts` routes to `executeOverlayAction` which calls `domPrimitiveOverlay` (self-contained, #502).
3. `domPrimitiveOverlay` finds overlay via its internal `findTopmostOverlay()`.
4. **Loop check**: If overlay has `data-kaboom-dismiss-ts` attribute with timestamp < 30s old, return `dismiss_loop_detected` error with overlay info and guidance.
5. If stamp is > 30s old, clear it and proceed (stale stamp from different page state).
6. Before dismiss action (click or Escape), stamp overlay with `data-kaboom-dismiss-ts = Date.now()` (tracks the attempt even if dismiss fails).
7. On next call, if overlay is still present and stamped, the loop check fires.

### 2. Cross-Extension Overlay Detection (#445)

1. `detectExtensionOverlay()` (now in `dom-primitives-overlay.ts`) checks if an overlay was injected by a browser extension.
2. Detection heuristics: `chrome-extension://` URLs in child iframes/resources, custom element hosts in shadow DOM ancestor chain.
3. When detected, `overlay_source: "extension"` is added to dismiss responses.
4. Combined with loop detection: if a dismissed overlay persists and is extension-sourced, the LLM gets clear guidance to ignore it.

### 3. Text Selector Auto-Resolve (#443)

1. LLM uses `text=Click here` selector to target an element.
2. `resolveByText()` walks text nodes, finds match, gets `parentElement`.
3. Tries `parent.closest('a, button, [role="button"], ...')` for interactive ancestor.
4. **New**: If no interactive ancestor, tries `parent.querySelector('a[href], button, input, ...')` for interactive child.
5. Falls back to `parent` only if no interactive child found.

### 4. Response Verbosity Control (#447)

1. Session preference `response_mode.summary = true` set via configure tool.
2. On interact command completion, `formatCompleteCommand()` checks `loadSummaryPref()`.
3. If summary mode active, `stripSummaryModeFields()` removes verbose fields:
   - `dom_summary`, `dom_mutations`, `perf_diff`, `evidence`, `transient_elements`, `trace`, `analysis`, `viewport`
4. Essential fields preserved: `status`, `timing`, `dom_changes`, `matched`, errors.

### 5. Region-Scoped Element Discovery (#448)

1. LLM calls `interact({what: "list_interactive", near_x: 500, near_y: 300, near_radius: 150})`.
2. `normalizeDOMActionArgs()` converts `near_x/near_y/near_radius` to `scope_rect: {x: 350, y: 150, width: 300, height: 300}`.
3. `domPrimitiveListInteractive()` filters elements by `intersectsScopeRect()`.
4. When `scopeRect` is present, computes `distance_px` from scope center to each element center.
5. Elements sorted by distance (closest first) instead of reading order.

## State and Contracts

1. Dismiss stamps use DOM attribute `data-kaboom-dismiss-ts` — self-cleaning when element is removed.
2. Stamp TTL is 30 seconds. After that, retries are allowed (page navigation may have changed context).
3. `near_*` params are aliases — explicit `scope_rect` takes precedence.
4. Summary mode stripping only applies to successful interact responses, not errors.
5. Extension overlay detection is best-effort heuristic; false negatives are acceptable.

## Code Paths

- `src/background/dom/dom-primitives-overlay.ts` — Loop detection check, dismiss stamp, extension overlay detection (#502 extraction)
- `scripts/templates/partials/_dom-selectors.tpl` — Interactive child fallback in resolveByText
- `src/background/dom/dom-primitives-list-interactive.ts` — Distance calculation, proximity sort
- `src/background/dom/dom-dispatch.ts` — Routes overlay/intent/stability actions to extracted modules
- `cmd/browser-agent/tools_interact_dom.go` — near_* to scope_rect conversion
- `cmd/browser-agent/tools_async_result_normalization.go` — stripSummaryModeFields
- `cmd/browser-agent/tools_async_formatting.go` — Summary mode integration

## Test Paths

- `extension/background/dom-primitives-overlay.test.js` — 10 tests: loop detection (5), extension overlay (2), text= resolve (3)
- `cmd/browser-agent/tools_interact_handler_test.go` — Near params conversion (2 tests)
- `cmd/browser-agent/tools_async_enrich_test.go` — Summary mode stripping (3 tests)

## Edit Guardrails

1. Dismiss stamp must only apply to the overlay element itself, never to dismiss buttons within it.
2. Extension overlay detection must not block dismissal — it only flags the source.
3. Interactive child resolution must not change behavior when an interactive ancestor exists (ancestor takes precedence).
4. Summary mode must never strip error/retry/message fields.
5. `near_*` to `scope_rect` conversion must not override explicit `scope_rect`.
