---
doc_type: feature_index
feature_id: feature-rate-limiting
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-06
code_paths:
  - internal/circuit/breaker.go
  - internal/lifecycle/observer.go
  - internal/capture/capture.go
  - internal/capture/httpingest/handlers.go
test_paths:
  - internal/capture/httpingest/handlers_test.go
  - internal/circuit/breaker_test.go
  - internal/lifecycle/observer_test.go
  - internal/capture/coverage_gaps_test.go
  - internal/capture/no_facade_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Rate Limiting

The extension-facing rate-limit HTTP boundary is owned directly by
`httpingest.Handlers`; `Capture` has no forwarding handler methods.

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

The circuit breaker publishes state transitions through the canonical
`lifecycle.Observer` owned by Capture. Runtime subscribers and capture
publishers use `Capture.Lifecycle()` directly; Capture does not duplicate the
observer's subscribe or emit API.

Lifecycle callback tests await the emitted transition itself. They never delay
the scheduler and infer that the asynchronous callback probably ran.
