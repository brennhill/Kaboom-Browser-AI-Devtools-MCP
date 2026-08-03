---
doc_type: feature_index
feature_id: feature-performance-audit
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-08-03
code_paths:
  - internal/performance/diff.go
  - internal/performance/types.go
  - internal/capture/accessors.go
  - internal/capture/model.go
  - src/lib/analysis/perf-snapshot.ts
  - src/lib/analysis/performance.ts
test_paths:
  - internal/performance/diff_test.go
  - internal/performance/diff_resource_test.go
  - internal/performance/diff_summary_test.go
  - internal/performance/wire_performance_test.go
  - internal/capture/accessor_unit_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Performance Audit

## TL;DR

- Status: proposed
- Tool: generate
- Mode/Action: performance_audit
- Location: `docs/features/feature/performance-audit`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_PERFORMANCE_AUDIT_001
- FEATURE_PERFORMANCE_AUDIT_002
- FEATURE_PERFORMANCE_AUDIT_003

## Code and Tests

Performance snapshots and pre-action baselines now use deterministic oldest-first
single-pass eviction. Both stores expose size, capacity, cumulative drops, and
oldest age through the common health resource-pressure contract.
