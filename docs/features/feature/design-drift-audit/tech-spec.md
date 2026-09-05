---
doc_type: tech-spec
feature_id: feature-design-drift-audit
status: shipped
owners: []
last_reviewed: 2026-08-22
links:
  index: ./index.md
  product: ./product-spec.md
  qa: ./qa-plan.md
---

# Design Drift Audit Tech Spec

## Pipeline

```
analyze(what="design_audit")
  → analyzedispatch/dispatcher.go
  → designdrift.Handle(Deps, req, args)          handler.go
      ├─ resolve spec (caller spec + page :root tokens)   spec.go
      ├─ probe the page for computed styles + rects       src/inject/computed-styles.ts
      └─ run the requested analyzers over []elementView
           ├─ analyzeConsistency   consistency.go
           ├─ analyzeTokens        tokens.go
           └─ analyzeSpacing       spacing.go
  → auditResult{ findings, skipped }             finding.go
```

`Handle` is the only exported function besides `Deps`. Everything else is
package-internal, so the analyzers stay pure functions over `[]elementView` and
are covered by table tests rather than through the dispatcher.

**The page measures; Go judges.** `src/inject/computed-styles.ts` reports raw
observed values and rendered bounding rects and makes no decisions. Content
scripts are bundled and awkward to test; keeping judgment in Go keeps the
decision logic under table tests.

## Wire contract

`internal/styleprobe/wire_style_probe.go` and `src/types/wire/wire-style-probe.ts`
are the two halves of the probe payload and must change together
(`make check-wire-drift`). `internal/schema/analyze.go` declares the `design_audit`
mode arguments (`selector`, `categories`, `spec`);
`internal/tools/configure/capabilities/modespecs_analyze.go` exposes it in the
capability matrix.

## Severity is derived, not chosen

`severityFor(provenance)` in `finding.go` is the only way to set severity:

```go
func severityFor(provenance string) string {
    if provenance == provenanceDeclared {
        return severityError
    }
    return severityWarning
}
```

`declared` → `error`, `inferred` → `warning`. An analyzer cannot construct a
finding that claims an inferred expectation at error severity. Precedence is
resolved **per property** in `consistencyExpectation`, so a spec declaring only
`spacing_scale` leaves font and colour findings inferred in the same response.

Only a caller-supplied `spec` (and the `:root` conflict it produces) is
`declared`. A near-miss of a *page* token is `inferred`: the page declared the
token, not that this element must use it, so the last step is proximity —
`pageTokenLengthFinding` and `pageTokenColorFinding` in `tokens.go`.

Confidence (`confidenceHigh` and its bands) scales with majority strength — 9 of
10 is not 3 of 4 — and is reported separately from severity.

## Analyzer decisions worth preserving

**Gaps come from rendered bounding rects, never declared margins.** Adjacent
vertical margins collapse to the larger of the two, so `margin-bottom +
margin-top` is not the rendered gap. Do not "simplify" this back to margin
arithmetic.

**The rhythm is the modal gap, not the mean.** One 14px among 24s drags a mean to
21.5px, which flags every correct gap and understates the real outlier.

**Colour distance is perceptual (OKLab), not RGB.** RGB weights channels equally
when the eye does not, calling distinct colours close and identical-looking ones
far apart. The threshold is calibrated in `tokens_test.go` against the
`#2b56e2` / `#2a55e1` pair from #694.

**Length near-misses are relative, not absolute.** 2px off a 4px token is a
different value; 2px off a 64px token is a slip.

**Only near-misses are reported.** A page's computed styles hold hundreds of
legitimate non-token lengths; flagging them all buries the wrong one. An exact
token match is the success state and produces nothing.

## Peer-group narrowing (`consistency.go`)

`auditedProperties()` limits the audit to properties where variation is almost
never intentional: font-family, font-size, font-weight, line-height,
letter-spacing, color.

`eligiblePeers` drops state variants before any vote is taken. `isStateVariant`
→ `classesOf` → `classMarksState` → `isStateWord` matches the markers in
`stateVariantMarkers()` (`.active`, `.selected`, `.disabled`, …). Excluding them
is not only about suppressing their own findings — leaving them in would skew the
majority every other element is judged against.

`normalizeValue` / `normalizeFontFamily` collapse equivalent spellings of the
same computed value so a font stack quoted two ways is not reported as drift.

Groups under three peers produce a `skipped{reason: insufficient_peers}` rather
than a verdict: two elements have no majority and a verdict would be a coin flip.

## Skips are results, not silence

`auditResult` carries `findings` and `skipped` side by side (`finding.go`). A
category that could not run reports why. Producing no findings because nothing
ran is not a clean page, and reporting the two identically is how a tool starts
claiming success it did not earn.

## Spec conflict is reported, not resolved

When the caller's `spec` disagrees with the page's own `:root` tokens, `spec.go`
emits a finding rather than silently preferring one. One of the two is stale, and
picking a winner quietly hides which.
