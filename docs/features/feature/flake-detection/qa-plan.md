---
doc_type: qa-plan
feature_id: feature-flake-detection
last_reviewed: 2026-08-04
---

# Continuous Flake Detection — QA Plan

## Automated Coverage

- `scripts/ci/run-flake-detection.test.mjs` verifies deterministic planning, artifact fields, failure preservation, replay commands, bounded output, retry classification, and telemetry opt-out.
- `internal/flakerepro/runner_test.go` verifies attempt validation, immutable original evidence, deterministic perturbations, cancellation, and verdict aggregation.

## Required Scenarios

1. Run the same seed twice and compare command plans.
2. Make an original command fail and later retries pass; the campaign must still fail and classify the instability.
3. Reproduce a failure consistently; the artifact must retain every attempt and report reproduction.
4. Remove or prevent the artifact; the workflow must fail rather than upload incomplete evidence.
5. Verify every child process receives production telemetry opt-out.
6. Cancel between attempts and confirm no additional attempt starts.

## Release Gate

Run `npm run docs:check:strict`, the JavaScript flake-runner tests, the Go `internal/flakerepro` tests, and the workflow syntax checks used by CI.
