---
doc_type: feature_index
feature_id: feature-performance-audit
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-08-07
code_paths:
  - internal/performance/diff.go
  - internal/performance/types.go
  - internal/capture/perfstore/store.go
  - src/lib/analysis/perf-snapshot.ts
  - src/lib/analysis/performance.ts
test_paths:
  - internal/performance/diff_test.go
  - internal/performance/diff_resource_test.go
  - internal/performance/diff_summary_test.go
  - internal/performance/wire_performance_test.go
  - internal/performance/no_facade_test.go
  - internal/capture/perfstore/store_test.go
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

The focused `internal/capture/perfstore` owner retains performance snapshots,
chronological repeated-run samples, and pre-action baselines. Its injected clock
makes oldest-age and eviction behavior deterministic in tests. All three stores
use oldest-first single-pass eviction and expose size, capacity, cumulative
drops, and oldest age through the common health resource-pressure contract.
Nested maps, slices, pointer metrics, resource lists, and user-timing entries
are copied at both write and read boundaries, so comparison callers cannot
mutate retained baselines outside the store lock.
Post-navigation comparisons wait on a store-owned generation channel. New
snapshots wake bounded, cancellation-aware consumers immediately; command
results no longer poll the store or sleep while waiting for an after snapshot.
Package documentation now lives with the canonical performance domain model in
`types.go`. The standalone documentation-only file was deleted, and the package
boundary test holds the change-coupled owner set to ten files.
