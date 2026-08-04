---
doc_type: tech-spec
feature_id: feature-workflow-verification
last_reviewed: 2026-08-04
---

# Workflow Verification — Technical Specification

## Ownership

`internal/workflowverify/runner.go` owns definition validation, ordered execution, invariant evaluation, bounded diagnostic collection, primary-failure preservation, cleanup, and final verdict. `internal/verification` owns the reusable assertion and evidence contracts consumed by the runner.

## Execution Model

The runner accepts an injected executor so browser operations remain outside the domain package. It validates the complete definition before invoking external behavior, evaluates preconditions, processes steps in order, and stops forward progress at the first runtime failure. Cleanup uses a new bounded context and iterates in reverse declaration order without short-circuiting.

## Invariants

- Runtime results cannot replace an earlier primary failure.
- Diagnostic evidence must match the workflow correlation ID.
- Each evidence channel remains distinct in the report.
- Caller cancellation does not cancel cleanup immediately.
- Cleanup-only failure changes an otherwise passing result to `FAIL`.
- Definition errors and runtime verdicts remain separate API outcomes.

## Change Boundary

Runner types, executor boundary, verdict semantics, and `runner_test.go` change together. Browser-specific actions remain adapters to the five canonical tools.
