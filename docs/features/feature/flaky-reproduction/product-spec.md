---
doc_type: product-spec
feature_id: feature-flaky-reproduction
last_reviewed: 2026-08-04
---

# Deterministic Flaky-Failure Reproduction — Product Specification

## Outcome

Maintainers can determine whether a failed workflow is consistently broken, intermittent, or correlated with a declared environmental condition without losing the original evidence.

## Requirements

- Accept an explicit plan of at most 20 attempts.
- Declare latency, CPU pressure, cache state, and tab lifecycle for every attempt.
- Reject unsupported pressure, cache, and lifecycle values before execution.
- Preserve the original failed report and derive child correlation IDs for attempts.
- Distinguish baseline reproduction from reproduction correlated with a perturbation.
- Report `FAIL`, `FLAKY`, `UNVERIFIED`, or `BLOCKED` from all outcomes; never infer `PASS` from a successful retry.
- Stop deterministically when cancellation is observed between attempts.

## Boundaries

The runner describes and executes controlled reproduction attempts. It does not insert correctness sleeps, invent random pressure, or edit the workflow under test.

## Acceptance

Given the same plan and executor results, the runner returns the same attempts, counts, rates, correlations, and verdict while retaining the unchanged original failure.
