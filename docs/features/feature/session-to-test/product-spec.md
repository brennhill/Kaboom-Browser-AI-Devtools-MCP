---
feature: Session to Test
status: in-progress
tool: interact
format: reproduction
doc_type: product-spec
feature_id: feature-session-to-test
last_reviewed: 2026-09-05
---

# Product Spec: Session to Test

## Problem Statement

A generated regression test is only worth having if it still runs next week. Two things broke that.

**One locator per step.** Every recorded step described its target with a single CSS selector, which
is a claim about where an element sits in the markup. A wrapper element, a renamed utility class or a
reordered list breaks it, and the replay reports "selector not found" — about an element that is
still on the page, still named the same thing, still in the same position on screen. The recording
observed all of that and kept none of it.

**An unheld environment.** A session recorded at a particular instant, in a particular timezone, at a
particular window size, with a live `Math.random`, produces a test that asserts on values that will
never recur. Pinning the environment fixes the replay. Pinning it *without saying so* makes it worse:
the test passes on the machine that recorded it and fails everywhere else, and nothing in the
artifact explains why.

## Users and Jobs

| User | Job |
| --- | --- |
| Agent driving a browser | Turn the session it just performed into a test the team can run in CI. |
| Engineer reading the artifact | Decide whether to trust a pass, and repair a failure without re-recording. |
| CI | Run the test on a machine that is not the one that recorded it. |

## Requirements

- **FEATURE_SESSION_TO_TEST_001** — Every recorded step carries three independent locators: derived
  selector, accessibility semantics (role + accessible name, plus a CDP ref where one is meaningful),
  and viewport coordinate with its frame and viewport context.
- **FEATURE_SESSION_TO_TEST_002** — The fallback order is fixed, documented and emitted with the
  artifact: selector, then accessibility, then coordinate.
- **FEATURE_SESSION_TO_TEST_003** — Both existing emission backends (Playwright and kaboom-native)
  carry the locators. No third output format is introduced.
- **FEATURE_SESSION_TO_TEST_004** — Environment pinning is opt-in per session and covers clock,
  timezone, geolocation, viewport and seeded randomness.
- **FEATURE_SESSION_TO_TEST_005** — The emitted artifact states what was pinned and to what, states
  plainly when nothing was pinned, and names any knob the browser refused.

## Non-goals

- Assertions derived from observed effects. That is `kaboom-x0li.1` and lands separately.
- Network response interception and replay. Named in the artifact as not pinned rather than
  half-implemented.
- Replacing the selector strategy. The three locators sit alongside each other; none is retired.

## Success

An agent records a checkout flow, the vendor ships a redesign that renames every class, and the
generated test still identifies each target — because the accessible name did not change. An engineer
reading a failure sees, in the test file itself, the two other ways that step used to be reached.
