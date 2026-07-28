---
doc_type: feature_index
feature_id: feature-ci-infrastructure
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - cmd/browser-agent/internal/ciapi/handlers.go
  - cmd/browser-agent/internal/ciapi/snapshot.go
  - cmd/browser-agent/internal/ciapi/types.go
  - cmd/browser-agent/server.go
test_paths:
  - internal/capture/state_resetter_owner_test.go
  - cmd/browser-agent/internal/ciapi/snapshot_test.go
  - cmd/browser-agent/ci_test.go
  - cmd/browser-agent/ci_unit_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Ci Infrastructure

## TL;DR

- Status: proposed
- Tool: observe, configure, interact
- Mode/Action: ci-cd, autonomous-repair, snapshots
- Location: `docs/features/feature/ci-infrastructure`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_CI_INFRASTRUCTURE_001
- FEATURE_CI_INFRASTRUCTURE_002
- FEATURE_CI_INFRASTRUCTURE_003

## Code and Tests

- HTTP endpoint implementation: `cmd/browser-agent/internal/ciapi/`
- Route registration: `cmd/browser-agent/server.go`
- Snapshot, clear, and test-boundary endpoint behavior:
  `cmd/browser-agent/internal/ciapi/handlers.go`
- The clear route receives `capture.StateResetter` explicitly; snapshot and
  test-boundary handlers retain only the narrower capture read surface.
- Snapshot filtering, statistics, and payload contracts:
  `cmd/browser-agent/internal/ciapi/snapshot.go`,
  `cmd/browser-agent/internal/ciapi/types.go`
- Characterization and route tests:
  `cmd/browser-agent/internal/ciapi/snapshot_test.go`,
  `cmd/browser-agent/ci_test.go`, `cmd/browser-agent/ci_unit_test.go`
