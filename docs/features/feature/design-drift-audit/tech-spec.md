---
doc_type: tech-spec
feature_id: feature-design-drift-audit
status: shipped
owners: []
last_reviewed: 2026-09-05
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
  → buildAuditResult(auditInputs)                finding.go
      ├─ reconcileAcrossCategories                (drop contradicted guesses)
      ├─ collapseShorthandDuplicates              (four longhands → one edit)
      └─ findingWindow.page                       (cap and page each section)
  → auditResult{ findings, skipped, totals }     finding.go
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
mode arguments (`selector`, `categories`, `spec`, `limit`, `offset`);
`internal/tools/configure/capabilities/modespecs_analyze.go` exposes it in the
capability matrix and `cmd/browser-agent/internal/cli/parser/observe_analyze.go`
gives each one a CLI flag.

A schema property that the mode's own params struct does not name is accepted
and discarded: `mcp.ParseArgs` is a plain `json.Unmarshal`, which ignores unknown
keys. `limit` was in the schema and absent from `auditParams` for exactly that
reason, so a caller following the response clamp's own advice got no error and
no effect. Every design_audit schema property must appear in `auditParams`.

## The response is bounded, and the bound is complete

`findingWindow` caps each section at `limit` findings (`defaultFindingsPerSection`
= `maxFindingsPerSection` = 50, mirroring `pageissues.pageIssuesPerSectionCap`)
starting at `offset`. `normalizeWindow` clamps a caller's `limit` down to the cap
rather than honouring it upward, because three sections of 50 findings measure
about 63KB against a 100KB clamp and a wider window would put silent truncation
back within reach of one call.

The envelope reports `total_findings` (the whole census), `returned_findings`,
and `next_offset`, and each section reports `total`/`returned`/`offset`/
`has_more`. Unbounded, 200 elements measured 588KB, of which
`mcp.ClampResponseSize` kept 46KB — the clamp cuts the JSON mid-string, so the
discarded findings were unrecoverable and the caller could not tell.

`collapseShorthandDuplicates` folds the four byte-identical findings one
`padding: 15px` produces into a single `padding` finding. Only a complete,
uniform group collapses; `uniformShorthandGroups` keys on the whole verdict, so a
`padding: 15px 16px` drifting on two sides keeps its longhands and no message
claims something false about the other two.

## Reconciliation across categories

`reconcileAcrossCategories` runs before the sections are built.
`gapProducingMargin` maps `gap-vertical` → `margin-top` and `gap-horizontal` →
`margin-left`, which is what makes a spacing finding and a token finding about
the same element and the same observed value findings about the same bytes rather
than a coincidence of equal numbers — grouping on the value alone would fold a
`font-size: 14px` outlier into a `gap-vertical: 14px` one.

The measured gap survives; the token near-miss is dropped and its expectation is
appended to the survivor's evidence. Both analyzers read the same probe, so a
declared `spec` makes both verdicts `declared` at once and the precedence can
never invert.

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
10 is not 3 of 4 — and is reported separately from severity. That band applies to
the **inferred** path only. A declared violation is always `confidenceHigh`:
grading a stated rule by how many gaps share a modal value reported the page's
uniformity as doubt about the caller's own rule, and an agent filtering to
high-confidence errors saw none of the declared spacing errors at all.

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

**The minimum guards inference, not the spec.** `analyzeConsistency` consults
`spec.declaresAnyAuditedProperty()` before skipping, and
`consistencyFindingsForProperty` resolves precedence before counting peers, so a
declared rule is judged per element at any group size. `analyzeSpacing` does the
same for `spec.declaresSpacing()`, dropping its flow and gap minimums to 2 and 1
— a declared scale states which gaps are legal, so one gap is judgeable. Checking
the peer count first made a rule the caller explicitly supplied unenforceable on
every pair, which is precisely where inference has nothing to offer either.

## Skips are results, not silence

`auditResult` carries `findings` and `skipped` side by side (`finding.go`). A
category that could not run reports why. Producing no findings because nothing
ran is not a clean page, and reporting the two identically is how a tool starts
claiming success it did not earn.

## Spec conflict is reported, not resolved

When the caller's `spec` disagrees with the page's own `:root` tokens, `spec.go`
emits a finding rather than silently preferring one. One of the two is stale, and
picking a winner quietly hides which.
