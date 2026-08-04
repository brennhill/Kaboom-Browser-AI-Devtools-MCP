---
doc_type: qa-plan
feature_id: feature-workflow-verification
last_reviewed: 2026-08-04
---

# Workflow Verification — QA Plan

## Automated Coverage

`internal/workflowverify/runner_test.go` uses a deterministic executor to cover validation, ordered execution, invariant failures, evidence filtering, cancellation, cleanup ordering, cleanup failures, and verdict selection.

## Required Scenarios

1. Reject malformed definitions before the executor receives a call.
2. Execute valid preconditions and steps in declaration order.
3. Fail an early invariant and verify later forward steps do not run.
4. Return mixed diagnostic channels but retain only evidence with the workflow correlation ID.
5. Cancel during forward execution and verify all cleanup still runs in reverse order under its bounded context.
6. Fail multiple cleanup steps and verify each is attempted while the primary failure remains unchanged.
7. Fail cleanup after otherwise successful execution and expect `FAIL`.
8. Compare repeated runs with the same deterministic executor and expect identical reports apart from controlled timestamps.

## Release Gate

Run `go test ./internal/workflowverify ./internal/verification` and the repository race and coverage gates.
