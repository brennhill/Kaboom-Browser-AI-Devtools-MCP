---
doc_type: product-spec
feature_id: feature-workflow-verification
last_reviewed: 2026-08-04
---

# Workflow Verification — Product Specification

## Outcome

Developers can verify a complete user workflow—not merely its final page—and receive the earliest actionable failure with correlated local evidence and reliable cleanup.

## Requirements

- Define preconditions, ordered executable steps, per-step invariants, and explicit cleanup.
- Require workflow and correlation IDs, described steps, unique invariant IDs, and at least one precondition.
- Stop forward execution at the first broken step or invariant and preserve it as the primary diagnosis.
- Collect only diagnostic evidence carrying the workflow correlation ID.
- Keep DOM, network, console, Doctor, and screenshot evidence in separate channels.
- Run every cleanup step in reverse order under a fresh bounded context, including after cancellation or partial navigation.
- Report cleanup failures without replacing an earlier root cause.
- Treat malformed definitions as errors and runtime faults as verdict data.

## Acceptance

The report identifies the earliest workflow failure, contains only correlated evidence, attempts all cleanup exactly once in reverse order, and cannot report success when cleanup fails.
