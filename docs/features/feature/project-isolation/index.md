---
doc_type: feature_index
feature_id: feature-project-isolation
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-08-03
code_paths:
  - internal/state/paths.go
  - internal/persistence/persistence_store.go
test_paths:
  - internal/state/paths_test.go
  - internal/state/paths_coverage_test.go
  - internal/state/no_facade_test.go
  - internal/persistence/persistence_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Project Isolation

## TL;DR

- Status: proposed
- Tool: configure
- Mode/Action: multi-tenancy
- Location: `docs/features/feature/project-isolation`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_PROJECT_ISOLATION_001
- FEATURE_PROJECT_ISOLATION_002
- FEATURE_PROJECT_ISOLATION_003

## Code and Tests

Shared runtime state resolves through `internal/state` and honors
`KABOOM_STATE_DIR` as its explicit isolation boundary. The package exposes only
canonical paths; callers do not read, migrate, or delete historical locations.
Content-addressed QA artifacts use the canonical `evidence` directory beneath
that same root, so test isolation and user-state isolation apply consistently.
Persistence's asynchronous flush runs through the shared panic-contained
goroutine launcher, keeping a background storage failure from terminating the
daemon.
