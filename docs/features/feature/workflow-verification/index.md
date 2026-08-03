---
doc_type: feature_index
feature_id: feature-workflow-verification
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-03
code_paths:
  - internal/workflowverify/runner.go
  - internal/verification/contract.go
  - internal/verification/evidence.go
test_paths:
  - internal/workflowverify/runner_test.go
---

# Workflow Verification

Kaboom verifies multi-step user workflows through an ordered runner boundary.
A workflow declares its preconditions, executable steps, per-step invariants,
and cleanup steps. The executor boundary is where the canonical five tools
supply browser actions, checks, diagnostics, and recovery operations.

The runner stops at the first broken invariant or failed step, retains that
failure as the primary diagnosis, and requests a bounded diagnostic bundle.
Only evidence carrying the workflow correlation ID is retained. Bundles have
separate DOM, network, console, Doctor, and screenshot channels so a final-page
symptom does not hide the earlier transition that failed.

Cleanup always runs in reverse declaration order under a fresh bounded context,
including when navigation is partial, the extension reconnects, or the caller
is cancelled. Every cleanup step is attempted; cleanup failures are reported
without replacing an earlier root cause. A cleanup-only failure changes an
otherwise passing workflow to `FAIL`.

Runtime faults are verdict data, while malformed workflow definitions are
errors. Definitions require a workflow ID, correlation ID, preconditions,
described steps with uniquely identified invariants, and explicit cleanup.
