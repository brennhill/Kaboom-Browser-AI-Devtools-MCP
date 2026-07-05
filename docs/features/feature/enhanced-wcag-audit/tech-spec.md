---
feature: enhanced-wcag-audit
status: proposed
doc_type: tech-spec
feature_id: feature-enhanced-wcag-audit
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Enhanced WCAG Audit

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

Enhanced WCAG Audit extends `observe({what: "accessibility"})` with additional checks beyond axe-core's built-in ruleset. Runs as post-processing step after axe-core audit completes. Adds manual checks for: color contrast (deeper analysis), keyboard navigation (focus trap detection), ARIA validity (relationship verification), and form accessibility (label associations).

## Key Components

**Color Contrast Analyzer**: Uses computed styles to check text/background contrast ratios. Validates WCAG AA (4.5:1 normal, 3:1 large text) and AAA (7:1 normal, 4.5:1 large). Cross-references with axe-core results to avoid duplicates.

**Keyboard Navigation Tester**: Detects focus traps (elements that capture focus and prevent tab navigation). Uses synthetic keyboard events to walk through tab order. Flags if focus gets stuck or cycles infinitely.

**ARIA Relationship Validator**: Checks `aria-labelledby`, `aria-describedby`, `aria-controls`, `aria-owns` point to valid element IDs. Flags broken references.

**Form Accessibility Checker**: Validates all inputs have associated labels (explicit `<label for>` or implicit wrapper). Checks required field indicators (aria-required or required attribute). Validates error message associations (aria-invalid + aria-describedby).

**Finding Categorization**: Each issue tagged with: WCAG level (A, AA, AAA), severity (critical, high, medium, low), affected elements (selectors), and remediation guidance.

## Data Flows

```
AI calls observe({what: "accessibility", enhanced: true})
  |
  v
Extension runs axe-core audit (existing behavior)
  |
  v
Post-processing: enhanced checks
  -> Color contrast analysis on text elements
  -> Keyboard navigation simulation
  -> ARIA relationship validation
  -> Form accessibility checks
  |
  v
Merge findings with axe-core results
  |
  v
Return combined audit report
```

## Implementation Strategy

**Extension files**:
- `extension/lib/enhanced-wcag.js` (new): Post-axe-core checks
- `extension/inject.js` (modified): Invoke enhanced checks if `enhanced: true`

**Server files**:
- `cmd/browser-agent/queries.go`: Pass `enhanced` flag to extension

**Trade-offs**:
- Opt-in via `enhanced: true` flag (not always-on) to avoid slowing down standard a11y audits
- Keyboard navigation test limited to 100 tab stops to prevent infinite loops
- Color contrast only checks visible text (not hidden or off-screen)

## Edge Cases & Assumptions

- **axe-core not loaded**: Enhanced checks still run (don't depend on axe-core output)
- **Computed styles unavailable**: Contrast check skipped for affected elements
- **Focus trap test gets stuck**: Timeout after 100 tab stops, flag as potential trap

## Risks & Mitigations

**Risk**: Keyboard navigation test hangs on infinite focus loop.
**Mitigation**: Hard limit of 100 tab stops. Test aborts if exceeded and flags finding.

**Risk**: Enhanced checks double execution time.
**Mitigation**: Opt-in only. Default `observe({what: "accessibility"})` unchanged (fast path).

## Dependencies

- axe-core (existing)
- Computed styles API
- Keyboard event simulation

## Performance Considerations

| Metric | Target |
|--------|--------|
| Enhanced checks execution time | < 500ms |
| Memory impact | < 2MB |
| Total audit time (axe + enhanced) | < 3 seconds |

## Security Considerations

- Read-only DOM access
- Keyboard simulation contained to tab navigation (no form submission or clicks)
- No network requests initiated
