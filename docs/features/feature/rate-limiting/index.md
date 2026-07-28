---
doc_type: feature_index
feature_id: feature-rate-limiting
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - internal/circuit/breaker.go
  - internal/lifecycle/observer.go
  - internal/capture/capture.go
  - internal/capture/handlers.go
test_paths:
  - internal/circuit/breaker_test.go
  - internal/lifecycle/observer_test.go
  - internal/capture/coverage_gaps_test.go
  - internal/capture/no_facade_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Rate Limiting

## TL;DR

- Status: shipped
- Tool: configure
- Mode/Action: throttling
- Location: `docs/features/feature/rate-limiting`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_RATE_LIMITING_001
- FEATURE_RATE_LIMITING_002
- FEATURE_RATE_LIMITING_003

## Code and Tests

Add concrete implementation and test links here as this feature evolves.
