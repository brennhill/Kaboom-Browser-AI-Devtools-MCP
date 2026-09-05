---
feature: effect-verification
status: shipped
tool: interact
mode: every mutating action
doc_type: product-spec
feature_id: feature-effect-verification
last_reviewed: 2026-09-05
---

# Product Spec: Effect Verification

## Purpose

Tell the caller what a mutating action was observed to **do**, not that it was dispatched.

An agent that only learns "the click was sent" builds its next three steps on a page that may never
have changed. `kaboom-knms` was exactly that: an action that reported success while changing
nothing. Every browser-driving agent has this failure mode by default, because dispatch is the only
thing a control API returns.

## Requirements

| # | Requirement |
| --- | --- |
| 1 | Every mutating `interact` action returns an `effects` block naming one of four outcomes. |
| 2 | An action that changed nothing is reported as `dispatched_and_no_observable_effect`, never as a plain success. |
| 3 | An unrun window is reported as `not_evaluated`, never as "no effect". |
| 4 | The block states that its attribution is temporal, not causal, in the payload itself. |
| 5 | Verification costs no latency on read-only actions. |
| 6 | An action that worked returns in about one poll interval; only an action that did nothing spends the full budget. |
| 7 | `dispatched_and_no_observable_effect` stops a retry that cannot succeed and says why. |
| 8 | The caller can opt out (`effects: false`) and can widen the window (`effect_window_ms`, max 5000). |

## What it deliberately does not do

- **It does not prove causation.** Entries are attributed because they were recorded inside the
  window. A page with a polling timer will attribute its own request to whatever action was running.
  The block says so; it does not pretend otherwise.
- **It does not block, retry, or re-issue anything.** It reports; the caller decides.
- **It does not add a new telemetry channel.** Every signal it reads was already being captured, and
  the DOM half was already being sent by the extension and discarded.
- **It does not gate on user approval.** No prompt, no consent step, nothing in the driving path.

## Deprecations

Nothing is removed. A caller that ignores `effects` sees the response it saw before, plus one field.

## See also

- [Effect Verification index](./index.md)
- [Effect Verification Tech Spec](./tech-spec.md)
- [Effect Verification QA Plan](./qa-plan.md)
