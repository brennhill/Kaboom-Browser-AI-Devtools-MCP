---
doc_type: product-spec
feature_id: feature-design-drift-audit
status: shipped
owners: []
last_reviewed: 2026-08-22
links:
  index: ./index.md
  tech: ./tech-spec.md
  qa: ./qa-plan.md
---

# Design Drift Audit Product Spec

## TL;DR

- Problem: visual inconsistency passes every functional test. A step header
  rendering Roboto 11px beside two rendering Inter 12px is 100% functional and
  visibly unpolished, and nothing in a unit or E2E suite objects.
- User value: one call over a rendered page returns the specific elements that
  drifted, each with the value observed, the value expected, and where that
  expectation came from.
- Surface: `analyze({ what: "design_audit", selector, categories?, spec? })`.

## Problem

Automated tests assert presence, roles, and behavior. They do not assert that a
page looks like one product. The defects that survive are:

| Category | Issue | Example |
| --- | --- | --- |
| `style_consistency` | [#693](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/issues/693) | Step 2's header is Roboto 11px; Steps 1 and 3 are Inter 12px |
| `design_tokens` | [#694](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/issues/694) | `padding: 15px` where `--spacing-md: 16px`; `#2b56e2` where the token is `#2a55e1` |
| `spacing` | [#695](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/issues/695) | Card gaps run 24 / 24 / 14 / 24 down a stack |

An agent asked to "check the design" without this tool reads a screenshot and
guesses, or dumps computed styles and drowns: a page's computed styles contain
hundreds of legitimate non-token values, so reporting all of them buries the one
value that is actually wrong.

## Solution

One mode, `design_audit`, taking a `categories` filter — not three modes. All
three categories are the same pipeline with different analyzers, and splitting
them would have widened the reachability-only coverage gap without giving the
caller anything.

The page measures and Go judges. `src/inject/computed-styles.ts` reports raw
observed values and makes no decisions; the analyzers are pure functions.

### Expectations come from one of two places

| Provenance | Severity | Meaning |
| --- | --- | --- |
| `declared` | `error` | A stated rule was broken — the caller's `spec`, or the page's own `:root` tokens |
| `inferred` | `warning` | A majority vote among peers; a statistical outlier can be intentional |

Severity is derived from provenance, never chosen by an analyzer. This is the
triage axis users act on: **fix all errors** is safe to batch because every error
contradicts something explicitly declared, while **fix all warnings** is a review
pass because a legitimate variant is indistinguishable from drift in a computed
style dump.

Precedence resolves per property, never per call. A `spec` naming only
`spacing_scale` makes spacing deviations errors while font and colour deviations
in the same response stay warnings.

Inference alone cannot flag a page that is uniformly wrong — there the majority
*is* the wrong value — which is why a caller can declare the design system.

## User stories

- As an agent finishing a UI change, I audit the component I touched and get the
  elements that drifted, so I fix them before review instead of after.
- As an agent with a design system on hand, I pass `spec` so deviations are
  reported as errors against the stated rules rather than as majority votes.
- As an agent triaging a large page, I filter to `categories: ["spacing"]` so one
  question gets one answer.
- As a reviewer, I trust that an empty result means "audited and clean", because
  a category that could not run reports `checks_skipped` with a reason instead of
  producing silence.

## Requirements

1. Report drift in three categories: `style_consistency`, `design_tokens`,
   `spacing`. A `categories` argument restricts the run.
2. Every finding carries the observed value, the expected value, its provenance,
   and the element it belongs to.
3. Severity is a function of provenance only. A finding may not claim an inferred
   expectation with error severity.
4. When a caller's `spec` disagrees with the page's own tokens, report it as a
   finding. One of the two is stale, and picking a winner quietly hides which.
5. Report only near-misses against tokens, never every literal value. An exact
   token match is the success state and produces nothing.
6. A category that could not run reports `checks_skipped` with a reason. Zero
   findings because nothing ran must not be reported as a clean page.
7. Groups under three peers report `insufficient_peers` rather than a verdict.
8. Confidence scales with majority strength.

## Non-goals

- **Pixel regression.** No baseline is stored or compared; see
  [Design Audit Archival](../design-audit-archival/index.md) and
  `visual_baseline` / `visual_diff`. Those catch *what changed*; this catches
  *what is inconsistent right now*, with no baseline needed.
- **Accessibility.** Contrast and roles belong to the accessibility audit.
- **Auto-fixing.** The tool reports; edits stay with the caller.
- **Auditing every property.** `style_consistency` deliberately covers only
  properties where variation is almost never intentional.

## False-positive policy

Legitimate variation looks exactly like drift in a raw computed-style dump, so
the consistency analyzer is deliberately narrow: it audits only font-family,
font-size, font-weight, line-height, letter-spacing, and color; it excludes state
variants (`.active`, `.selected`, `.disabled`, …) from the peer group entirely
rather than reporting them at low confidence, since leaving them in would also
skew the majority everything else is judged against.

## Success criteria

- Every defect from #693, #694, and #695 is reported against the fixture page.
- All five negative controls produce nothing: first-child margin, state variant,
  exact token match, parent-owned flex gap, two-element group.
- A uniformly-wrong page produces errors when a `spec` is supplied.
- Both directions are asserted in CI without a browser.
