---
doc_type: qa-plan
feature_id: feature-design-drift-audit
status: shipped
owners: []
last_reviewed: 2026-08-22
links:
  index: ./index.md
  product: ./product-spec.md
  tech: ./tech-spec.md
---

# Design Drift Audit QA Plan

## Strategy

The analyzers are pure functions over `[]elementView`, so the whole judgment
layer is table-tested in Go with no browser. A browser is needed only to capture
what a real page reports, and that capture is committed as a fixture.

| Layer | Where | Needs a browser |
| --- | --- | --- |
| Analyzer units | `analyzers_test.go`, `tokens_test.go` | No |
| Mode contract | `contract_test.go` | No |
| Fixture end-to-end | `testdata/fixture-probe.json` → `TestFixtureProducesExactlyTheExpectedFindings` | No |
| Live page | `scripts/tests/browser/cat-36-design-drift.sh` (UAT category 36) | Yes |

## Single source of truth

`testdata/expected-findings.json` holds the expected verdicts and is consumed by
**both** the Go tests and UAT category 36. Three hand-maintained lists would
drift apart, and a design-drift tool whose own expectations have drifted is not
worth much.

`testdata/fixture-probe.json` holds real computed styles captured from
`cmd/browser-agent/internal/testpages/pages/design-drift.html` in Chrome, so the
end-to-end assertion runs in CI without a browser.

## Fixture coverage

The fixture plants every example from the three issues plus five negative
controls. Both directions are asserted: every planted positive is found, **and**
every control produces nothing.

### Positives

| Case | Category | Issue |
| --- | --- | --- |
| Step 2 header Roboto 11px among Inter 12px peers | `style_consistency` | #693 |
| `padding: 15px` against `--spacing-md: 16px` | `design_tokens` | #694 |
| `#2b56e2` against token `#2a55e1` | `design_tokens` | #694 |
| Card gaps 24 / 24 / 14 / 24 | `spacing` | #695 |

### Negative controls

| Control | Why it must produce nothing |
| --- | --- |
| First-child margin | No preceding sibling, so no gap to judge |
| State variant (`.active`) | Excluded from the peer group before the vote |
| Exact token match | An exact match is the success state |
| Parent-owned flex `gap` | The gap is not the child's to answer for |
| Two-element group | No majority; must report `insufficient_peers` |

## Cases

### Severity derivation

- [ ] A caller `spec` violation reports `severity: error`, `provenance: declared`.
- [ ] A majority-vote outlier reports `severity: warning`, `provenance: inferred`.
- [ ] No finding exists with `provenance: inferred` and `severity: error`
      (`severityFor` is the only setter — assert the invariant, not one example).
- [ ] A spec declaring only `spacing_scale` yields spacing **errors** and font or
      colour **warnings** in the same response (per-property precedence).

### Skips vs. clean

- [ ] A category that cannot run reports `checks_skipped` with a reason.
- [ ] A group of two reports `insufficient_peers`, not a verdict.
- [ ] An empty `findings` list with a populated `skipped` list is never presented
      as a clean audit.

### Token near-misses

- [ ] `15px` against a `16px` token is reported.
- [ ] An exact token match produces nothing.
- [ ] A 2px deviation from a 4px token and from a 64px token are judged
      differently (relative, not absolute).
- [ ] `#2b56e2` vs `#2a55e1` is inside the perceptual threshold; the OKLab
      calibration in `tokens_test.go` is the guard against an RGB regression.

### Spacing

- [ ] The modal gap is the rhythm; a single 14px among 24s flags once, not four
      times.
- [ ] Gaps derive from rendered rects — a collapsed-margin case does not report
      `margin-bottom + margin-top`.

### Spec conflict

- [ ] A caller `spec` that contradicts the page's `:root` tokens produces a
      finding naming the conflict rather than silently picking a winner.

### Confidence

- [ ] 9 of 10 reports `high`; 3 of 4 reports `low`.

## Regression guards

- `make check-wire-drift` — `wire_style_probe.go` and `wire-style-probe.ts` are
  two halves of one contract.
- UAT category 36 replays against a recorded transcript in CI and against a live
  browser when re-recorded (`scripts/tests/transcripts/record-connected-transcripts.sh --category 36`).
