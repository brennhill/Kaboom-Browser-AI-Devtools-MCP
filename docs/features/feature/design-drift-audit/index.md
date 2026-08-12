---
doc_type: feature_index
feature_id: feature-design-drift-audit
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-12
code_paths:
  - cmd/browser-agent/internal/toolanalyze/designdrift/handler.go
  - cmd/browser-agent/internal/toolanalyze/designdrift/finding.go
  - cmd/browser-agent/internal/toolanalyze/designdrift/spec.go
  - cmd/browser-agent/internal/toolanalyze/designdrift/consistency.go
  - cmd/browser-agent/internal/toolanalyze/designdrift/tokens.go
  - cmd/browser-agent/internal/toolanalyze/designdrift/spacing.go
  - cmd/browser-agent/internal/toolanalyze/analyzedispatch/dispatcher.go
  - internal/styleprobe/wire_style_probe.go
  - internal/schema/analyze.go
  - internal/tools/configure/capabilities/modespecs_analyze.go
  - src/inject/computed-styles.ts
  - src/types/wire/wire-style-probe.ts
test_paths:
  - cmd/browser-agent/internal/toolanalyze/designdrift/contract_test.go
  - cmd/browser-agent/internal/toolanalyze/designdrift/analyzers_test.go
  - cmd/browser-agent/internal/toolanalyze/designdrift/tokens_test.go
  - cmd/browser-agent/internal/toolanalyze/designdrift/testdata/expected-findings.json
  - cmd/browser-agent/internal/toolanalyze/designdrift/testdata/fixture-probe.json
  - cmd/browser-agent/internal/testpages/pages/design-drift.html
  - scripts/tests/browser/cat-36-design-drift.sh
---

# Design Drift Audit

| Field       | Value                                              |
| ----------- | -------------------------------------------------- |
| **Status**  | shipped                                             |
| **Tool**    | `analyze`                                           |
| **Mode**    | `design_audit`                                      |
| **Issues**  | [#693](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/issues/693), [#694](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/issues/694), [#695](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/issues/695) |

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
| `declared` | `error`   | A stated rule was broken — the caller's spec, or the page's own `:root` token |
| `inferred` | `warning` | A majority vote among peers; a statistical outlier can be intentional |

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

## Design decisions worth knowing

**Gaps are measured from rendered bounding rects, never declared margins.**
Adjacent vertical margins collapse to the larger of the two, so
`margin-bottom + margin-top` is not the rendered gap. Do not "simplify" the gap
calculation back to margin arithmetic.

**The rhythm is the modal gap, not the mean.** One 14px among 24s drags a mean
to 21.5px, which flags every correct gap and understates the real outlier.

**Colour distance is perceptual (OKLab), not RGB.** RGB weights the channels
equally when the eye does not, so it calls distinct colours close and identical
looking ones far apart. The threshold is calibrated in `tokens_test.go` against
the `#2b56e2` / `#2a55e1` pair from #694.

**Length near-misses are relative, not absolute.** 2px off a 4px token is a
different value; 2px off a 64px token is a slip.

**Only near-misses are reported, never "every literal value".** A page's
computed styles contain hundreds of legitimate non-token lengths, and flagging
them all would bury the one value that is wrong. An exact token match is the
success state and produces nothing.

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
- Reports `insufficient_peers` for groups under three, because two elements have
  no majority and a verdict would be a coin flip.
- Scales confidence with majority strength: 9 of 10 is `high`, 3 of 4 is `low`.

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

## Related

- [Analyze Tool](../analyze-tool/index.md) — the parent tool and its dispatcher.
- [Design Audit Archival](../design-audit-archival/index.md) — a *different*
  feature despite the similar name: screenshot archival across breakpoints for
  pixel regression.
- `visual_baseline` / `visual_diff` are complementary rather than overlapping.
  Pixels catch **what changed** against a stored baseline; this catches **what
  is inconsistent right now**, with no baseline needed.
