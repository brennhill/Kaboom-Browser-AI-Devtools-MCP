---
doc_type: feature_index
feature_id: feature-performance-budget
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - internal/performance/types.go
  - internal/performance/diff.go
  - internal/performance/wire_performance.go
test_paths:
  - internal/performance/no_facade_test.go
  - internal/performance/diff_test.go
  - internal/performance/diff_resource_test.go
  - internal/performance/diff_summary_test.go
  - internal/performance/wire_performance_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Performance Budget

## TL;DR

- Status: shipped
- Tool: configure, observe
- Mode/Action: health, performance
- Location: `docs/features/feature/performance-budget`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_PERFORMANCE_BUDGET_001
- FEATURE_PERFORMANCE_BUDGET_002
- FEATURE_PERFORMANCE_BUDGET_003

## Code and Tests

Performance snapshots, timings, baselines, and regressions use the canonical
`Performance*` contracts directly. The former non-stuttering aliases were
deleted so the package has one explicit API surface.
