---
doc_type: feature_index
feature_id: feature-effect-verification
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-09-05
code_paths:
  - cmd/browser-agent/internal/actioneffects/effects.go
  - cmd/browser-agent/internal/actioneffects/classify.go
  - cmd/browser-agent/internal/interactdispatch/effects.go
  - cmd/browser-agent/internal/interactdispatch/handler.go
  - cmd/browser-agent/tools_interact_dispatch.go
  - cmd/browser-agent/internal/toolinteract/action_owners.go
  - cmd/browser-agent/internal/cli/parser/interact.go
  - cmd/browser-agent/internal/cli/parser/flags.go
  - internal/schema/interact/properties_core.go
  - internal/mcp/response_content.go
  - src/background/dom/primitives/dom-primitives-pointer.ts
  - src/background/dom/primitives/dom-primitives-form.ts
  - src/background/dom/primitives/dom-primitives-read.ts
test_paths:
  - cmd/browser-agent/internal/actioneffects/effects_test.go
  - cmd/browser-agent/internal/actioneffects/classify_test.go
  - cmd/browser-agent/internal/interactdispatch/effects_test.go
  - cmd/browser-agent/internal/cli/parser/commands_test.go
  - internal/mcp/response_test.go
---

# Effect verification

Every mutating `interact` action comes back saying what it was observed to **do**, not merely that
it was dispatched.

## Why this exists

`kaboom-knms` was an action that reported success while changing nothing. That is not an edge case;
it is the default failure mode of any browser agent that reports dispatch. A click lands on a
detached node, an overlay eats the event, a handler bails on a guard — the event fired, so the
dispatch succeeded, so the tool said success, so the agent moved on and built its next three steps
on a page that never changed.

Neither competing browser agent closes this loop. Both report that the action was sent. Kaboom
already captures console, network and action telemetry alongside the page, so it can say what
followed the action — and it already receives a DOM mutation report it was throwing away.

## What it does

| Behaviour | Detail |
| --- | --- |
| Window | Opened before dispatch (a clock read and the tracked URL), closed after. Default 600 ms, `effect_window_ms` up to 5000. |
| DOM short-circuit | The extension's mutation report rides on the action's own response, so a DOM change answers before any polling starts: `window_ms: 0`, no latency at all. The common success case is free. |
| Early close | Otherwise the window closes on the **first** observed effect. Only an action whose DOM report said nothing moved can spend the full budget — the one case where the answer is worth waiting for. |
| Scope | Mutating actions only (`toolinteract.IsMutationAction`), minus the effect-blind set below. A read-only action has no effect to verify, so it is charged no latency. Skipped for `background`/`async` calls, which return a queue receipt rather than an outcome, and for failed dispatches, whose outcome is not in doubt. |
| Effect-blind actions | `set_storage`, `delete_storage`, `clear_storage`, `set_cookie`, `delete_cookie`, `highlight`. They mutate state no signal here observes, so a window over them would report no effect for an action that worked — and then stop the caller retrying it. |
| Opt-out | `effects: false` (`--no-effects`). |

### The three outcomes

| `outcome` | Means |
| --- | --- |
| `dispatched_and_observed_effect` | The action ran and something followed it. |
| `dispatched_and_no_observable_effect` | The action ran, the window ran, and nothing moved. This is `kaboom-knms`, now named instead of reported as a success. |
| `dispatched_then_error` | The action itself failed. |
| `not_evaluated` | No window ran. Reported rather than guessed: calling an unrun window "no effect" is the same lie with the sign flipped. |

### What counts as an effect

| Signal | Source |
| --- | --- |
| DOM mutation | The `dom_summary` / `dom_changes` report the mutating DOM primitives already produce — `click`, `type`, `select`, `check`, `set_attribute`, `paste`, `key_press`, `hover`. `withMutationTracking` installs a `MutationObserver` **before** the action runs, so a synchronous re-render is caught; this is not a post-hoc observer, and it costs nothing new. |
| Network requests | `bodystore` entries whose server ingest timestamp falls inside the window. Captured request bodies are **pushed** by the extension on a 200 ms debounce. The network waterfall is deliberately not used: nothing pushes it, so it is populated only when `observe` pulls it, and a window reading it would report zero requests for a page that made several. |
| Console errors and warnings | Server log entries added inside the window. |
| Navigation | The tracked tab URL moved. |
| Transients | Toast/alert/snackbar classifications recorded on enhanced actions inside the window. |

Counts are exact; listings are capped (10 requests, 3 error messages, 200 chars each) so a chatty
page cannot crowd out the response.

## Attribution is temporal, not causal

Every effects block carries `attribution: "temporal_window"` and a note saying so. Entries land in
the window because they were **recorded inside it**, not because they were shown to be caused by
the action. A page with a five-second polling timer will attribute a request to whatever action
happened to be running. Saying `caused_by` here would be a lie the caller cannot check; saying
`attribution` lets the caller weigh the evidence.

## Retry policy

`dispatched_and_no_observable_effect` sets `retryable: false` and explains that repeating the
identical action against an unchanged page produces the same result — re-target the element or fix
the precondition (scroll it into view, dismiss the overlay, wait for the control to enable) first.
No other outcome writes retry fields, and an explicit decision already on the response is never
overwritten.

## Known limits

- A window that opens while the extension is disconnected reports `not_evaluated`, never `no effect`.
- Network attribution uses server **ingest** time, not the request's page-relative start, and covers
  `fetch()` only — the same limit `observe network_bodies` carries. A request the page made through
  another transport is invisible to the window.
- The default 600 ms budget has to clear the extension's own send cadence (console batcher 100 ms,
  action batcher 200 ms, plus an HTTP round trip). A slower connection can still close the window
  before the evidence lands, reporting no effect for an action that had one.
- Summary mode strips `dom_summary`, and `dom_changes` is only populated under `analyze: true`, so
  the DOM signal is unavailable there; the other four signals still run.
- `focus` and `scroll_to` are not wrapped in `withMutationTracking`, and browser-level actions
  (`navigate`, `refresh`, `back`, `new_tab`, storage and cookie writes) are not DOM primitives at
  all. All of these report `DOMUnknown` and are classified from network, console, navigation and
  transients alone — which is why an absent DOM report is never read as "the DOM did not change".
