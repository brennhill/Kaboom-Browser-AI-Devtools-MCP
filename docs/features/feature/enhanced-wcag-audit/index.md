---
doc_type: feature_index
feature_id: feature-enhanced-wcag-audit
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-08-03
code_paths:
  - internal/a11ysummary/summary.go
  - internal/tools/observe/page_state.go
  - src/types/capture/accessibility.ts
test_paths:
  - internal/a11ysummary/summary_test.go
  - cmd/browser-agent/tools_observe_analysis_test.go
  - internal/tools/observe/page_state_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Enhanced Wcag Audit

## TL;DR

- Accessibility summary counts reject unsigned values that cannot fit the host
  integer rather than wrapping them into negative counts.

- Status: proposed
- Tool: observe
- Mode/Action: accessibility
- Location: `docs/features/feature/enhanced-wcag-audit`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_ENHANCED_WCAG_AUDIT_001
- FEATURE_ENHANCED_WCAG_AUDIT_002
- FEATURE_ENHANCED_WCAG_AUDIT_003

## Code and Tests

Accessibility summaries use one canonical count contract: `violations`,
`passes`, `incomplete`, and `inapplicable`. Legacy `*_count` compatibility
fields are neither accepted nor emitted. Numeric counts are normalized from
every JSON-compatible integer and floating representation, including
`json.Number` and numeric strings; malformed values fall back to the canonical
top-level audit arrays.
