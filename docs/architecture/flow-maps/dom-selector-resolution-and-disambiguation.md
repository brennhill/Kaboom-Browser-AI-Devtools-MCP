---
doc_type: flow_map
flow_id: dom-selector-resolution-and-disambiguation
status: active
last_reviewed: 2026-03-06
owners:
  - Brenn
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# DOM Selector Resolution and Disambiguation

## Scope

Selector resolution inside extension DOM primitives used by `interact` mutating actions (`click`, `type`, `select`, `check`, `focus`, `paste`, `key_press`, `scroll_to`, and intent helpers), including modal-overlay blocking guards.

**#502 Split:** Intent actions (`open_composer`, `submit_active_composer`, `confirm_top_dialog`), overlay actions (`dismiss_top_overlay`, `auto_dismiss_overlays`), and stability actions (`wait_for_stable`, `action_diff`) are dispatched directly by `dom-dispatch.ts` to self-contained extracted modules (`dom-primitives-intent.ts`, `dom-primitives-overlay.ts`, `dom-primitives-stability.ts`). They no longer route through the main `domPrimitive` function.

## Entrypoints

1. `interact` tool calls DOM primitive actions through the extension command path.
2. Action target resolution calls `resolveElements` and `resolveElement` in DOM primitives.

## Primary Flow

1. `list_interactive` enumerates candidates and may emit `:nth-match(N)` selectors for duplicates.
2. A later mutating action receives selector targeting and optional disambiguation inputs (`nth`, `scope_selector`, `scope_rect`).
3. If `nth` is provided, `resolveActionTarget` resolves all selector matches, applies visibility/scope filtering, and picks a 0-based index (`-1` means last).
4. If selector contains `:nth-match(N)`, `parseNthMatchSelector` extracts base selector + 1-based ordinal.
5. `resolveElements` resolves the base selector and narrows to the indexed candidate.
6. `resolveElement` uses the same helper for single-target resolution.
7. Action runs against the resolved element and reports match strategy (`nth_param`, `nth_match_selector`, `selector`, or scoped variants).
8. For `scroll_to` with directional intent, the resolved target picks a container in priority order: target if scrollable -> nearest scrollable ancestor -> document scrolling root.
9. Before mutating/focus actions execute, modal-overlay guard checks whether the target is outside the top dialog and returns `blocked_by_overlay` when input would be hijacked.

## Error and Recovery Paths

1. Invalid `:nth-match(N)` format returns no match.
2. Out-of-range ordinal returns no match.
3. Invalid `nth` type returns `invalid_nth`.
4. Out-of-range `nth` returns `nth_out_of_range`.
5. Duplicate unresolved selectors surface `ambiguous_target` guidance to use `nth`, `:nth-match(N)`, or `scope_selector`.
6. If a modal dialog blocks the resolved target, actions return `blocked_by_overlay` with `dismiss_top_overlay` recovery guidance.

## State and Contracts

1. `:nth-match(N)` is a stable contract between `list_interactive` output and mutating action input.
2. Ordinal is 1-based and scoped to the evaluated selector context.
3. `nth` is 0-based after filtering (negative values count from end) and is shared across selector-based mutating/read-only actions.
4. Successful mutating actions return selector diagnostics in `matched` (tag/role/aria/text/classes/bbox/selector/element_id).
5. `observe(command_result)` surfaces `matched` at top level and removes the duplicate nested copy for token efficiency.
6. `scroll_to` accepts semantic `direction` (`top|bottom|up|down`); legacy `value` remains supported for backward compatibility.

## Code Paths

- `src/background/dom/dom-primitives.ts`
- `scripts/templates/partials/_dom-selectors.tpl`
- `scripts/templates/dom-primitives.ts.tpl`
- `extension/background/dom/dom-primitives.js`
- `src/background/dom/dom-dispatch.ts`
- `src/background/dom/dom-primitives-intent.ts`
- `src/background/dom/dom-primitives-overlay.ts`
- `src/background/dom/dom-primitives-stability.ts`
- `src/background/dom/dom-types.ts`
- `cmd/browser-agent/tools_async_result_enrichment.go`
- `cmd/browser-agent/tools_async_result_normalization.go`

## Test Paths

- `extension/background/dom-primitives.test.js`
- `tests/extension/list-interactive-selector-roundtrip.test.js`
- `cmd/browser-agent/tools_async_enrich_test.go`
- `cmd/browser-agent/tools_interact_rich_test.go`
- `cmd/browser-agent/cli_test.go`

## Edit Guardrails

1. Any change to `:nth-match(N)` parsing must keep `resolveElements` and `resolveElement` behavior aligned.
2. Template and generated source must stay synchronized (`node scripts/generate-dom-primitives.js --check`).
3. Add regression coverage when changing ambiguity/disambiguation behavior.
