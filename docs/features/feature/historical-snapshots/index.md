---
doc_type: feature_index
feature_id: feature-historical-snapshots
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-05
code_paths:
  - internal/types/snapshot.go
  - internal/session/types.go
  - internal/session/snapshot-manager.go
test_paths:
  - internal/session/snapshot_manager_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Historical Snapshots

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/historical-snapshots`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Flow Map: [flow-map.md](./flow-map.md)

## Requirement IDs

- FEATURE_HISTORICAL_SNAPSHOTS_001
- FEATURE_HISTORICAL_SNAPSHOTS_002
- FEATURE_HISTORICAL_SNAPSHOTS_003

## Code and Tests

- Snapshot contract types:
  - `internal/types/snapshot.go`
  - `internal/session/types.go`
  - `internal/session/snapshot-manager.go`
- Tests:
  - `internal/session/snapshot_manager_test.go`
