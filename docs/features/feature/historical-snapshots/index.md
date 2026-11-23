---
doc_type: feature_index
feature_id: feature-historical-snapshots
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-08-06
code_paths:
  - internal/platform/types/snapshot.go
  - internal/analysis/snapshots/snapshot-manager.go
  - internal/analysis/snapshots/runtime_reader.go
  - internal/analysis/snapshots/comparison.go
  - internal/analysis/snapshots/snapdiff/types.go
  - internal/analysis/snapshots/snapdiff/errors.go
  - internal/analysis/snapshots/snapdiff/network.go
  - internal/analysis/snapshots/snapdiff/performance.go
test_paths:
  - internal/analysis/snapshots/snapshot_manager_test.go
  - internal/analysis/snapshots/sessions_test.go
  - internal/analysis/snapshots/sessions_compare_regression_test.go
  - internal/analysis/snapshots/comparison_test.go
  - internal/analysis/snapshots/snapdiff/errors_test.go
  - internal/analysis/snapshots/snapdiff/network_test.go
  - internal/analysis/snapshots/snapdiff/performance_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Historical Snapshots

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/protocol/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/protocol/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/historical-snapshots`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_HISTORICAL_SNAPSHOTS_001
- FEATURE_HISTORICAL_SNAPSHOTS_002
- FEATURE_HISTORICAL_SNAPSHOTS_003

## Code and Tests

- Snapshot contract types:
  - `internal/platform/types/snapshot.go`
  - `internal/analysis/snapshots/snapshot-manager.go`
- `snapshot-manager.go` owns the bounded snapshot aggregate, its capture-reader
  boundary, name invariants, list representation, and CRUD helpers. Handler and
  comparison behavior remain separate, while duplicate handler and reader
  test suites have been folded into their canonical owners.
- Snapshot diff engine:
  - `internal/analysis/snapshots/snapdiff/`
- Tests:
  - `internal/analysis/snapshots/snapshot_manager_test.go`
  - `internal/analysis/snapshots/snapdiff/`
