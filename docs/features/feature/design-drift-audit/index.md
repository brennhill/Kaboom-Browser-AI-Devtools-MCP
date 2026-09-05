---
doc_type: feature_index
feature_id: feature-design-drift-audit
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-09-05
code_paths:
  - cmd/browser-agent/internal/toolanalyze/designdrift/handler.go
  - cmd/browser-agent/internal/toolanalyze/designdrift/finding.go
  - cmd/browser-agent/internal/toolanalyze/designdrift/spec.go
  - cmd/browser-agent/internal/toolanalyze/designdrift/consistency.go
  - cmd/browser-agent/internal/toolanalyze/designdrift/tokens.go
  - cmd/browser-agent/internal/toolanalyze/designdrift/spacing.go
  - cmd/browser-agent/internal/toolanalyze/analyzedispatch/dispatcher.go
  - cmd/browser-agent/internal/cli/parser/observe_analyze.go
  - internal/styleprobe/wire_style_probe.go
  - internal/schema/analyze.go
  - internal/tools/configure/capabilities/modespecs_analyze.go
  - src/inject/computed-styles.ts
  - src/types/wire/wire-style-probe.ts
  - gokaboom.dev/src/content/docs/reference/analyze.md
test_paths:
  - cmd/browser-agent/internal/toolanalyze/designdrift/contract_test.go
  - cmd/browser-agent/internal/toolanalyze/designdrift/analyzers_test.go
  - cmd/browser-agent/internal/toolanalyze/designdrift/spacing_test.go
  - cmd/browser-agent/internal/toolanalyze/designdrift/tokens_test.go
  - cmd/browser-agent/internal/toolanalyze/designdrift/testdata/expected-findings.json
  - cmd/browser-agent/internal/toolanalyze/designdrift/testdata/fixture-probe.json
  - cmd/browser-agent/internal/testpages/pages/design-drift.html
  - scripts/tests/browser/cat-36-design-drift.sh
  - scripts/docs/reference/check-reference-schema-sync.mjs
---

# Design Drift Audit

| Field       | Value                                              |
| ----------- | -------------------------------------------------- |
| **Status**  | shipped                                             |
| **Tool**    | `analyze`                                           |
| **Mode**    | `design_audit`                                      |
| **Issues**  | [#693](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/issues/693), [#694](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/issues/694), [#695](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/issues/695) |

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Summary

Detects design drift on a rendered page: computed-style outliers within a group
of matching elements, values that near-miss a declared CSS token, and uneven
spacing between siblings.

These are the defects that pass every functional test. Unit and E2E tests check
presence and roles; a step header rendering Roboto 11px next to two rendering
Inter 12px is 100% functional and visibly unpolished.

## What it catches

| Category            | Issue | Example                                                                    |
| ------------------- | ----- | -------------------------------------------------------------------------- |
| `style_consistency` | #693  | Step 2's header is Roboto 11px; Steps 1 and 3 are Inter 12px                |
| `design_tokens`     | #694  | `padding: 15px` where `--spacing-md: 16px`; `#2b56e2` where the token is `#2a55e1` |
| `spacing`           | #695  | Card gaps run 24 / 24 / 14 / 24 down a stack                                |

One mode taking `categories`, not three modes. All three are the same pipeline
with different analyzers, and adding three modes would have made the
reachability-only coverage problem worse.

## Usage

```jsonc
// Infer the norm from the page itself.
analyze({ what: "design_audit", selector: ".step-card__header" })

// One category only.
analyze({ what: "design_audit", selector: ".card", categories: ["spacing"] })

// Page past a bounded response. Follow next_offset until it is absent.
analyze({ what: "design_audit", selector: ".card", offset: 50 })

// Declare the design system. Inference cannot flag a page that is uniformly
// wrong, because there the majority IS the wrong value.
analyze({
  what: "design_audit",
  selector: ".card",
  spec: {
    spacing_scale: [4, 8, 16, 24, 32],
    font_families: ["Inter"],
    colors: ["#2a55e1", "#111827"],
    font_sizes: [12, 14, 16]
  }
})
```

## Severity is derived, not chosen

| Provenance | Severity  | Meaning                                                        |
| ---------- | --------- | -------------------------------------------------------------- |
| `declared` | `error`   | A stated rule was broken — the caller's `spec`, or the page's own `:root` disagreeing with it |
| `inferred` | `warning` | The analyzer supplied the expectation: a majority vote among peers, or proximity to a page token |

A page token on its own does not make an element's value `declared`. The page
declared `--spacing-md: 16px`; it never declared that *this* element's padding
must use it, and that last step is proximity — a guess. Page-token near-misses
are therefore `inferred`/`warning`, and a caller-supplied `spec` is the only
thing here that makes a token finding an error. Before this rule, a 14px
`margin-top` was reported as a high-confidence *error* against `--spacing-md`
(16px) while the correct target, `--spacing-lg` (24px), came back as a
low-confidence spacing warning — so "fix all errors" made the page worse.

This is the triage axis. **Fix all errors** is a safe batch operation because
every error contradicts something explicitly declared. **Fix all warnings** is a
review pass because a legitimate variant looks identical to drift in a computed
style dump. `severityFor` is the only way to set severity, so a finding cannot
claim an inferred expectation with error severity.

Precedence is resolved **per property**, never per call: a spec naming only
`spacing_scale` makes spacing deviations errors while font and colour deviations
in the same response stay warnings.

When the caller's spec disagrees with the page's own tokens, that is reported as
a finding rather than silently resolved — one of the two is stale, and picking a
winner quietly hides which.

## The response is bounded, and the bound is complete

Each section carries at most `limit` findings (default and maximum 50), starting
at `offset`. The envelope reports all three numbers so a truncation cannot be
mistaken for a clean page:

| Field | Meaning |
| ----- | -------- |
| `total_findings` | every finding the audit made, whatever this response carries |
| `returned_findings` | how many are in this payload |
| `next_offset` | the offset that returns the rest; absent when the response is complete |
| per section | `total`, `returned`, `offset`, `has_more` |

This exists because the mode probes up to 200 elements and a page whose cards
each break one rule produces several findings per element. Unbounded, 200
elements measured 588KB, of which `mcp.ClampResponseSize` kept 46KB — the clamp
cuts the JSON mid-string, so the rest was neither readable nor recoverable, and
its note told the caller to page with `limit`/`offset` that `design_audit` did
not accept. `limit` is capped rather than free so a single call cannot put that
failure back within reach; page with `offset` instead.

**One `padding: 15px` is one finding.** The probe reports longhands because
`padding: 15px 16px` really is two values, but when all four sides carry the same
verdict the author wrote one declaration, and reporting it four times multiplied
40 identical cards into 200 findings for one edit. Only the complete, uniform
group collapses onto the shorthand — a group drifting on two sides keeps its
longhands, because "padding is 15px" would be false about the other two.

## Two categories cannot answer the same question differently

A measured gap and a token near-miss can describe the same pixels. The gap before
`.rhythm-card[3]` *is* that element's `margin-top`, and the DEFAULT call used to
report both — `[design_tokens] margin-top 14px → --spacing-md (16px)` at
`confidence:high`, and `[spacing] gap-vertical 14px → 24px` at `confidence:low`.
24px is right; 16px was picked only because 14 sits inside the 15% band of 16 and
outside the band of 24. An agent triaging by confidence applied the wrong one.

**Proximity loses.** A rhythm is measured from what the page renders across the
element's own peers; a near-miss is the analyzer guessing that the author reached
for a token and mistyped. The token finding is dropped and its expectation is
folded into the surviving finding's evidence, so nothing disappears silently.
Both analyzers read the same probe, so a declared `spec` makes both verdicts
`declared` at once and the precedence never inverts.

## A declared rule needs no peer group

The peer minimums guard **inference**, not the spec. `style_consistency` and
`spacing` used to answer `insufficient_peers` before the spec was consulted, so
two elements both rendering Comic Sans against `spec{font_families:["Inter"]}`
answered "Design audit ran no checks" — a rule the caller explicitly supplied was
unenforceable on every group of two, which is the one case where inference has
nothing to offer either. A stated rule is now judged per element and per gap
whatever the group size; only the majority vote still needs three peers.

A declared violation is `confidence: high` and says so in its message ("which is
not on the declared spacing scale"). Grading a stated rule by how many gaps share
a modal value reported the page's uniformity as doubt about the caller's own
rule, and left an agent filtering to high-confidence errors seeing none of them.

## Design decisions worth knowing

**Gaps are measured from rendered bounding rects, never declared margins.**
Adjacent vertical margins collapse to the larger of the two, so
`margin-bottom + margin-top` is not the rendered gap. Do not "simplify" the gap
calculation back to margin arithmetic.

**The rhythm is the modal gap, not the mean.** One 14px among 24s drags a mean
to 21.5px, which flags every correct gap and understates the real outlier.

**A gap is only measured between two elements that touch.** A selector match is
a flat list, not a line: it can span two rows of a grid, several wrapped chip
lines, or two sections of a page. Rows and columns are derived from the geometry
(mutual centre containment on the cross axis), each line is measured on its own
axis, and a line is split wherever a gap reaches `containerBreakRatio` (3x) its
own rhythm. Measuring the flat list as one run is what reported
`overlap-horizontal observed=-300px` at `confidence:high` on an evenly-spaced
3x2 card grid, and what blamed a section's 120px padding on the margin of the
first card after it. A doubled margin is 2x the rhythm and is still reported.

**A mode of one is not a rhythm.** The modal gap only becomes the norm when it
holds a strict majority of that axis's gaps — the same refusal
`inferredFindings` makes for computed styles. Without it, an escalating 12 / 20 /
32 / 48 scale reported three findings citing "1 of 4 vertical gaps measure 12px",
a rhythm no part of the page has. On the declared-scale path the evidence names
the spec, because the verdict came from the spec and not from any rhythm.

**Colour distance is perceptual (OKLab), not RGB.** RGB weights the channels
equally when the eye does not, so it calls distinct colours close and identical
looking ones far apart. The threshold is calibrated in `tokens_test.go` against
the `#2b56e2` / `#2a55e1` pair from #694.

**Length near-misses are relative, not absolute.** 2px off a 4px token is a
different value; 2px off a 64px token is a slip.

**Only near-misses are reported, never "every literal value".** A page's
computed styles contain hundreds of legitimate non-token lengths, and flagging
them all would bury the one value that is wrong. An exact token match is the
success state and produces nothing. This holds on the declared-`spec` branch
too: a used length near no step of the scale is not a missed step. The 137.5px
that `margin: 0 auto` resolves to and the -1px of the border-collapse idiom are
layout output, not spacing choices, and no scale of positive magnitudes can ever
contain them — reporting them would make permanent unfixable errors out of
correct CSS.

**A length token governs one property family and no other.** Families are read
off the token *name* (typography before spacing, so `--letter-spacing` is
typography and cannot govern padding), because CSS gives a custom property no
type: `--spacing-md` and `--radius-md` are both just "16px" by the time
`getComputedStyle` reports them. A name the classifier cannot place governs
**nothing** rather than everything — a wrong guess must cost a missed finding,
never an invented one. Without this, `--font-size-lg: 18px` turned every 17px
padding into a near-miss of the type scale, and the near-universal
`--font-size-sm: 14px` silently excused every 14px padding that was really a
near-miss of `--spacing-md: 16px`.

**A spec judges spacing choices, not the page's own declarations.** When the
caller's `spec` and the page's `:root` disagree, `detectSpecConflicts` reports it
once against `:root`. An element that renders the page's own token is obeying its
design system, so it is not re-reported — otherwise one disagreement is
multiplied by the element count, including against the fixture's own
exact-token-match control.

**Colour distance counts opacity.** OKLab has no alpha axis, so alpha is carried
as a fourth, independently weighted term. Dropping it made
`rgba(42, 85, 225, 0.1)` an *exact* match of `#2a55e1`, so every tint token
shadowed the opaque token it was derived from and near-misses were attributed to
a colour the element could not have meant.

**The page measures; Go judges.** `src/inject/computed-styles.ts` reports raw
observed values and makes no decisions. Content scripts are bundled and awkward
to test; the analyzers are pure functions with table tests.

## False-positive policy

Legitimate variation is indistinguishable from drift in a raw computed-style
dump, so `style_consistency` is deliberately narrow:

- Audits only properties where variation is almost never intentional
  (font-family, font-size, font-weight, line-height, letter-spacing, color).
- Excludes state variants (`.active`, `.selected`, `.disabled`, …) from the peer
  group entirely rather than reporting them at low confidence — leaving them in
  would also skew the majority the other elements are judged against.
- Reports `insufficient_peers` for groups under three **when it is inferring**,
  because two elements have no majority and a verdict would be a coin flip. A
  declared `spec` is judged on any group size.
- Scales confidence with majority strength on the inferred path: 9 of 10 is
  `high`, 3 of 4 is `low`. A declared violation is always `high`.

A category that could not run reports `checks_skipped` with a reason. Producing
no findings because nothing ran is not a clean page, and reporting the two
identically is how a tool starts claiming success it did not earn.

## Verification

The fixture at `cmd/browser-agent/internal/testpages/pages/design-drift.html`
plants every example from the three issues plus five negative controls
(first-child margin, state variant, exact token match, parent-owned flex gap,
two-element group).

`testdata/expected-findings.json` is the **single source of truth** for expected
verdicts, consumed by both the Go tests and UAT category 36 — three
hand-maintained lists would drift apart.

`testdata/fixture-probe.json` holds the real computed styles captured from that
page in Chrome, so `TestFixtureProducesExactlyTheExpectedFindings` proves the
analyzers and the fixture agree without needing a browser in CI. Both directions
are asserted: every planted positive is found, and every control produces
nothing.

One case in that table takes the DEFAULT call — no `categories` at all. Every
other case narrows, and that gap is why the contradictory-target defect shipped:
the two findings that disagreed could never appear in the same asserted
response.

## Related

- [Analyze Tool](../analyze-tool/index.md) — the parent tool and its dispatcher.
- [Design Audit Archival](../design-audit-archival/index.md) — a *different*
  feature despite the similar name: screenshot archival across breakpoints for
  pixel regression.
- `visual_baseline` / `visual_diff` are complementary rather than overlapping.
  Pixels catch **what changed** against a stored baseline; this catches **what
  is inconsistent right now**, with no baseline needed.
