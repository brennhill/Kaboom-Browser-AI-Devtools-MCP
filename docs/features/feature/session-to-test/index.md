---
doc_type: feature_index
feature_id: feature-session-to-test
status: in-progress
feature_type: feature
owners: []
last_reviewed: 2026-09-05
code_paths:
  - internal/reproduction/reproduction_locators.go
  - internal/reproduction/reproduction_playwright.go
  - internal/reproduction/reproduction_kaboom.go
  - internal/reproduction/reproduction.go
  - internal/types/wire_enhanced_action.go
  - internal/types/network.go
  - internal/capture/actionstore/store.go
  - internal/schema/interact/actions.go
  - internal/schema/interact/properties_core.go
  - cmd/browser-agent/internal/toolinteract/interact_browser.go
  - cmd/browser-agent/internal/cli/parser/interact.go
  - cmd/browser-agent/internal/toolruntime/tools_interact_dispatch.go
  - src/lib/page/reproduction.ts
  - src/background/dom/cdp/cdp-env-pin.ts
  - src/background/environment-transaction/env-pin.ts
  - src/background/message-routing/telemetry-handler.ts
  - src/early-patch.ts
  - src/types/wire/wire-enhanced-action.ts
test_paths:
  - internal/reproduction/reproduction_locators_test.go
  - internal/reproduction/golden_test.go
  - internal/reproduction/testdata/reproduction-playwright.golden.txt
  - internal/capture/actionstore/store_test.go
  - tests/extension/session-to-test/cdp-env-pin.test.js
  - tests/extension/session-to-test/early-patch-seeded-random.test.js
  - tests/extension/session-to-test/three-locator-capture.test.js
  - tests/extension/session-to-test/env-pin-command.test.js
---

# Session to test

An exploratory agent session becomes a regression test that still runs after the page changes:
every recorded step carries three independent locators, and a session may pin the environment it
ran in so the emitted artifact states what it depends on.

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Why this exists

A generated test used to describe each target with one CSS selector. That selector is a statement
about where an element sits in the markup, so any re-render breaks it — a wrapper div, a renamed
class, a reordered list — and the replay then fails with "selector not found" and nothing else.
The element is usually still there, still named the same thing, still in the same place on screen.
The recording simply threw that away.

The second problem is the environment. A session recorded at 14:03 in Bangkok with a live
`Math.random` produces a test that asserts on ids, timestamps and sampled arms that will never come
back. Pinning fixes that, but a test that silently depends on a pinned clock is worse than one that
pins nothing: it passes on the machine that recorded it and fails everywhere else with no stated
cause.

## The three locators

Every recorded step carries all three, in a fixed fallback order.

| Order | Strategy | What it says | Why here |
| --- | --- | --- | --- |
| 1 | `selector` | Where the element sits in the markup | Resolves in-page with no debugger attach, and it is the only strategy Playwright expresses as a first-class locator. Most precise while the markup is unchanged. |
| 2 | `ax` | What the control means — role plus accessible name | Survives DOM restructuring, class churn and wrapper elements, because it describes semantics rather than shape. Costs an accessibility-tree snapshot over CDP, so it is not first. |
| 3 | `coordinate` | The point it occupied, plus the frame and viewport measured in | A point always resolves to *something*, so it can never report "not found" — it silently hits whatever occupies that point now. Correct only once both semantic strategies have failed. |

`FallbackOrder` in `internal/reproduction/reproduction_locators.go` is the single declaration of
that order; both backends render from it.

Three deliberate omissions:

- **The AX locator carries no `ref` from page-side recording.** A CDP AX ref is a backend node id
  valid only inside the snapshot that produced it, so recording one would be stale by replay time.
  Role plus accessible name is what `interact(what:'find')` resolves against. A `ref` supplied by a
  CDP-driven caller is preserved and printed, because there it is still meaningful.
- **A zero-area or non-finite box records no coordinate.** Inventing 0,0 would send a replayed click
  to the top-left corner of the page and report success.
- **A strategy the recording never captured is omitted, not emitted empty.** A locator nothing can
  act on is worse than one fewer answer.

The coordinate never travels alone: it carries the frame URL, the viewport size and the device pixel
ratio it was measured under, because at a different window size it lands somewhere else and in the
wrong frame it lands on the wrong document.

## Environment pinning

Opt-in per session, through `interact(what:'pin_environment', environment:{...})`. Nothing is pinned
until a caller asks, and an unpinned tab stamps nothing on its actions — which is what lets the
artifact state "Environment not pinned" as a fact rather than as a gap in reporting.

| Knob | CDP call | Note |
| --- | --- | --- |
| clock | `Emulation.setVirtualTimePolicy` | `initialVirtualTime` is TimeSinceEpoch in **seconds**; the caller passes `clock_epoch_ms`. Default policy `advance` fixes the origin and lets time run — `pause` stops it outright, which freezes the page being recorded. |
| timezone | `Emulation.setTimezoneOverride` | IANA id. |
| geolocation | `Emulation.setGeolocationOverride` | Accuracy defaults to 1m. |
| viewport | `Emulation.setDeviceMetricsOverride` | Same call the full-page screenshot path already uses. |
| randomness | `Page.addScriptToEvaluateOnNewDocument` + `src/early-patch.ts` | See below. |

**Seeded randomness lives in `early-patch.ts`**, which is the only script that runs in the MAIN world
at `document_start`, before page scripts can capture the originals. It publishes
`window.__KABOOM_SEED_RANDOM__(seed)`, which installs a xorshift128 generator over `Math.random` and
`crypto.getRandomValues`. The pinning path injects a snippet that sets `__KABOOM_RANDOM_SEED__` and
calls that installer — it carries no generator of its own, because two independent `Math.random`
replacements in one page would disagree. The two scripts have no guaranteed order, so both orders
work: the snippet calls the installer if it is already there, and the installer applies a seed that
is already set.

`applyEnvironmentPin` then reads `__KABOOM_RANDOM_SEED_ACTIVE__` back. A seed set on a page where
early-patch never ran (a cloaked domain, or CSP) is reported as **unpinned**, not pinned — setting a
seed nothing reads is not pinning, and claiming it would put a determinism claim in the emitted test
that the recording never honoured.

Every knob is attempted independently. One refusal never abandons the rest, and a refusal is named in
`unpinned` rather than swallowed: the knobs a session asked for and did not get are exactly the ones a
replay diverges on.

## Where the pin is recorded

Per action, not per session. A navigation clears CDP overrides, so a session-level record would claim
a pin that lapsed halfway through. `src/background/message-routing/telemetry-handler.ts` stamps the
live pin for the producing tab onto each enhanced action as it passes through the background — the
context that applied the pin, rather than the page, which cannot know what CDP was told.

`SessionPin` collapses identical pins into one artifact header and reports
`Environment pin changed during the recording` when they differ, instead of describing the whole
recording with the first one.

## What the artifact says

Playwright:

```
// Environment pinned by the recording session. This test depends on it:
// clock: epoch_ms 1705312800000, timezone UTC, virtual time policy pause
// viewport: 1280x720 @2x
// random seed: kaboom-golden (Math.random and crypto.getRandomValues)
// NOT pinned: network responses
// Locator fallback order: selector -> ax -> coordinate

test('reproduction: ...', async ({ page }) => {
  await page.getByTestId('email-input').click();
  // locator fallbacks (selector -> ax -> coordinate):
  // fallback ax: getByRole('textbox', { name: 'Email' })  [role=textbox name="Email" (ref ax_88)]
  // fallback coordinate: page.mouse.click(400, 220)  [(400, 220) in 1280x720 @2x frame https://...]
});
```

The kaboom-native backend states the same facts in `#` comments and indented `fallback` lines. No
third output format was added.

`generate({format:'reproduction'})` also reports `fallback_order`, `locator_coverage` and
`environment_pinned` in its metadata, so a caller can tell a session recorded with all three locators
from one that only ever had selectors without reading the script.

## Related features

- [Reproduction scripts](../reproduction-scripts/index.md) — the two emission backends this extends.
- [Test generation](../test-generation/index.md) — `generate({type:'test_from_context'})`, which
  emits through the same backends.
- [Interact / explore](../interact-explore/index.md) — `interact(what:'find')`, the accessibility-tree
  lookup the `ax` locator is designed to be resolved by.
- [Environment manipulation](../environment-manipulation/index.md) — the sibling surface that
  snapshots and restores cookies and storage.

## Not in scope here

Assertions derived from observed effects (status codes, console-clean, URL) are `kaboom-x0li.1` and
are deliberately absent from this change.
