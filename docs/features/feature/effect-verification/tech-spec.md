---
feature: effect-verification
status: shipped
tool: interact
doc_type: tech-spec
feature_id: feature-effect-verification
last_reviewed: 2026-09-05
---

# Tech Spec: Effect Verification

## Components

| Component | Responsibility |
| --- | --- |
| `cmd/browser-agent/internal/actioneffects/effects.go` | `Mark`, `Deps`, `Budget`, `Open`, `Collect`. Reads the capture buffers and builds the attributed set. |
| `cmd/browser-agent/internal/actioneffects/classify.go` | `Classify`, `Effects.Payload`, `ApplyRetryAdvice`, `DOMChangeFrom`. Turns the attributed set into an outcome and the retry advice it implies. |
| `cmd/browser-agent/internal/interactdispatch/effects.go` | Decides which calls earn a window, resolves and clamps the budget, attaches the block. |
| `cmd/browser-agent/internal/interactdispatch/handler.go` | Opens the mark before `toolrouting.Dispatch` and closes the window after it. |
| `cmd/browser-agent/tools_interact_dispatch.go` | `buildEffectDeps` names the production readers. |
| `internal/mcp/response_content.go` | `ReadResultPayload` / `MutateResultPayload` — the shared decode/mutate/re-encode of a tool result's JSON payload behind its summary line. |

## Control flow

1. `Handler.Handle` parses `effectArgs` and calls `wantsEffectWindow`.
2. If the call qualifies, `Effects()` supplies the readers and `actioneffects.Open` takes the mark:
   the wall clock and the tracked tab URL. Nothing else is read yet.
3. `toolrouting.Dispatch` runs the action owner.
4. If the dispatch did not fail, `attachEffects` reads the DOM verdict out of the action's own
   payload with `DOMChangeFrom`, then runs `actioneffects.Collect` with it.
5. `DOMChanged` returns immediately with `window_ms: 0` — the answer is already in hand, and
   waiting after it would tax every working action.
6. Otherwise `Collect` waits one poll interval, re-reads every buffer, and rebuilds the attributed
   set from scratch. Rebuilding rather than accumulating is what keeps a late-arriving telemetry
   batch from being counted twice.
7. The loop breaks on the first observed effect, or at the total budget.
8. `MutateResultPayload` writes `effects` onto the payload and applies retry advice.

The window lives in the dispatcher, not in the action owners: it is the one funnel every interact
action passes through, so no owner can forget it and no new owner has to remember it.

## Why the window opens before dispatch

The mark is the boundary between "already there" and "arrived after". Taking it after the action
would attribute the action's own fastest effects to the page's history instead. Taking it before
costs one clock read and one URL read, which is why it is taken even for calls that turn out to fail.

## Which calls earn a window

| Condition | Window |
| --- | --- |
| `Deps.Effects` unwired | no |
| Dispatch returned an error | no — the outcome is not in doubt, and the block would only add noise |
| `effects: false` | no |
| `background: true` or `async: true` | no — the response is a queue receipt, so the window would time the queueing |
| `what` in `effectBlindActions` | no — storage and cookie writes mutate state no signal here sees, and `highlight` injects Kaboom's own overlay |
| `toolinteract.IsMutationAction(what)` false | no — a read-only action has no effect to verify |
| otherwise | yes |

`IsMutationAction` is the same predicate the evidence-capture path uses to decide whether to take a
before-shot. It was unexported; this change exports it rather than restating the list.

`effectBlindActions` is written as a **subtraction** from that predicate rather than as a second
inclusion list, so the two cannot drift apart. A contract test holds every entry to still being a
mutation action, because an entry that is not one silently excludes nothing.

## Budget

| Bound | Value |
| --- | --- |
| Default total | 600 ms — clears the console batcher's 100 ms debounce, the action batcher's 200 ms, and an HTTP round trip. A shorter window reports "nothing happened" when the evidence had merely not arrived. |
| Poll interval | 50 ms |
| Maximum total | 5000 ms (`effect_window_ms` is clamped, so a caller cannot park the dispatcher) |

Poll is clamped down to the total when a caller asks for a window shorter than one poll.

## Signals and their sources

| Signal | Source | Boundary test |
| --- | --- | --- |
| DOM mutation | `dom_summary` / `dom_changes` on the action's own payload | the literal `no DOM changes`, or all-zero counts |
| Network | `capture.Telemetry().NetworkBodies().Snapshot()` | parallel timestamp slice `.After(mark.At)` — server ingest time |
| Console | `server.logs.EntriesWithAddedAt()` | parallel added-at slice `.After(mark.At)` |
| Actions and transients | `capture.Telemetry().Actions().Snapshot()` | parallel timestamp slice `.After(mark.At)` |
| Navigation | `capture.Extension().GetTrackingStatus()` | URL differs from `mark.URL` and is non-empty |

The waterfall is **not** a source here. Nothing pushes it: `get_network_waterfall` is a
content-script handler that `observe network_waterfall` pulls on demand, so a window reading
`waterfallstore` would report zero requests for a page that made several, and the resulting
`no_observable_effect` would stop a retry that should have happened. `bodystore` is fed by the
extension's own 200 ms-debounced batcher, which is why it is the network signal.

The DOM signal costs nothing new. `withMutationTracking` in
`scripts/templates/partials/_dom-action-helpers.tpl` already installs a `MutationObserver` on
`document.body` **before** running the action and always computes a summary, so a synchronous
re-render is caught. This change reads a report that was already being sent and discarded.

## Invariants

| Invariant | Why |
| --- | --- |
| A response this cannot parse is returned untouched | An enrichment that blanked an unreadable result would lose the answer the caller asked for. `MutateResultPayload` returns the original on any decode failure. |
| Counts are exact; listings are capped | 10 requests, 3 error messages, 200 chars each. A chatty page must not crowd the response out. |
| `attribution` is always `temporal_window` | There is one join and it is temporal. Naming it `caused_by` would be a claim the caller cannot check. |
| An explicit retry decision is never overwritten | `ApplyRetryAdvice` returns early if either `retryable` or `retry` is already set. |
| Content blocks after the first survive | A screenshot appended by a composable enrichment is not dropped by the payload rewrite. |

## Failure modes

| Failure | Behaviour |
| --- | --- |
| No readers wired | `Collect` returns immediately, `Evaluated` false, outcome `not_evaluated`. No wait is spent. |
| Malformed action args | `parseEffectArgs` returns zero values; canonical dispatch already reported the JSON fault, so this path logs nothing (`EXPECTED_ABSENCE`). |
| Buffer evicted mid-window | The attributed set shrinks; the outcome degrades toward `no_observable_effect`, never toward a false positive. |
| Extension disconnected | No telemetry arrives, no DOM report arrives; outcome is `no_observable_effect` with `dom_changed` absent. |

## See also

- [Effect Verification index](./index.md)
- [Effect Verification Product Spec](./product-spec.md)
- [Effect Verification QA Plan](./qa-plan.md)
