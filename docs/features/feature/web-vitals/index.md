---
doc_type: feature_index
feature_id: feature-web-vitals
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-04
code_paths:
  - src/lib/analysis/perf-snapshot.ts
  - src/lib/analysis/vitals-attribution.ts
  - internal/performance/wire_performance.go
  - internal/performance/types.go
  - internal/tools/observe/session.go
test_paths:
  - tests/extension/performance/web-vitals.test.js
  - internal/performance/wire_performance_test.go
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# Web Vitals

## TL;DR

- Status: shipped
- Tool: observe
- Mode/Action: vitals
- Location: `docs/features/feature/web-vitals`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_WEB_VITALS_001
- FEATURE_WEB_VITALS_002
- FEATURE_WEB_VITALS_003

## Code and Tests

Kaboom reports bounded element descriptors and phase timings for LCP and INP,
bounded shifting-node evidence for CLS, and long-task attribution with an
explicit availability status. Element text, values, arbitrary attributes, and
resource URLs are never included in attribution payloads.
