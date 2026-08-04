---
doc_type: qa-plan
feature_id: feature-flaky-reproduction
last_reviewed: 2026-08-04
---

# Deterministic Flaky-Failure Reproduction — QA Plan

## Automated Coverage

- `internal/flakerepro/runner_test.go` covers validation, attempt bounds, correlation IDs, original-evidence immutability, cancellation, and each verdict class.
- `internal/workflowverify/runner_test.go` covers the upstream failure and evidence semantics consumed by reproduction.

## Required Scenarios

1. Reject empty and over-limit plans without invoking the executor.
2. Reject each unknown cache, lifecycle, and pressure value.
3. Run identical plans against deterministic executors and compare complete reports.
4. Mix passing and failing baseline attempts and expect `FLAKY`.
5. Reproduce every baseline attempt and expect `FAIL`.
6. Reproduce only under a declared perturbation and expose that correlation.
7. Cancel between attempts and expect `BLOCKED` with no later executor call.
8. Attempt executor-side mutation and verify original evidence is unchanged.

## Release Gate

Run `go test ./internal/flakerepro ./internal/workflowverify` and the repository race-enabled Go test gate.
