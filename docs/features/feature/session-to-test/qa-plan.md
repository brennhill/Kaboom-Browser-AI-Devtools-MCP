---
doc_type: qa-plan
feature_id: feature-session-to-test
status: in-progress
owners: []
last_reviewed: 2026-09-05
links:
  product: ./product-spec.md
  tech: ./tech-spec.md
  qa: ./qa-plan.md
  feature_index: ./index.md
---

# Session to Test QA Plan

## Linked Specs

- Product: [product-spec.md](./product-spec.md)
- Tech: [tech-spec.md](./tech-spec.md)
- Feature index: [index.md](./index.md)

## Automated coverage

| Area | Test | What a failure would have cost |
| --- | --- | --- |
| Fallback order | `internal/reproduction/reproduction_locators_test.go` | A replay trying the coordinate before the accessible name silently clicks whatever now occupies the point. |
| Recovery without a selector | same | The failure the feature exists for: no selector, and the step must still be reachable by role and name. |
| Coordinate context | same | A bare x/y replayed at another window size lands somewhere else and reports success. |
| Both backends emit fallbacks | same | Half the artifacts stay brittle. |
| Pin reported in both backends | same | A test that depends on a pinned clock without saying so. |
| Unpinned stated, not omitted | same | A reader cannot tell "not pinned" from "not reported". |
| Mid-session pin change | same | The header claims a pin that lapsed halfway through the recording. |
| `Meta` coverage counts | same | A caller cannot tell three-locator sessions from selector-only ones. |
| Emitted format stability | `internal/reproduction/golden_test.go` + `testdata/reproduction-playwright.golden.txt` | A change that silently drops the AX or coordinate line. |
| Evidence detachment | `internal/capture/actionstore/store_test.go` | A later ingest rewriting locators or a pin a caller already holds. |
| CDP call shapes | `tests/extension/session-to-test/cdp-env-pin.test.js` | Epoch in ms puts the page ~55,000 years ahead; a refused knob reported as pinned. |
| Seeded generator | `tests/extension/session-to-test/early-patch-seeded-random.test.js` | A degenerate all-zeros stream, or a seed reported active when the page refused the replacement. |
| Page-side capture | `tests/extension/session-to-test/three-locator-capture.test.js` | A coordinate on a zero-area box; the AX locator disagreeing with the role selector. |
| Command parsing | `tests/extension/session-to-test/env-pin-command.test.js` | A caller's typo reported as a browser refusal. |

## Manual / UAT

1. `interact(what:'pin_environment', environment:{timezone_id:'UTC', clock_epoch_ms:..., random_seed:'run-1'})`
   against a tracked tab; confirm `Date().toString()` in the page reports UTC and two page loads draw
   the same `Math.random()` sequence.
2. Record a short flow, then `generate({format:'reproduction', output_format:'playwright'})`; confirm
   the header names the clock, the timezone and the seed, and that each targeted step carries its two
   fallbacks.
3. `interact(what:'unpin_environment')`, reload, confirm the tab is back on the machine clock.
4. Pin on a page whose CSP blocks the early patch; confirm the seed appears under `NOT pinned` rather
   than as a pinned seed.

## Known gaps

- Network response interception and replay (`Fetch.enable`) is not implemented; sessions report it
  through `unpinned` when a caller asks for it.
- Assertions from observed effects are `kaboom-x0li.1` and are not covered here.
