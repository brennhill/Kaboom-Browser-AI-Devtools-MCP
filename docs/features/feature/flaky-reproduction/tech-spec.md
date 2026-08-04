---
doc_type: tech-spec
feature_id: feature-flaky-reproduction
last_reviewed: 2026-08-04
---

# Deterministic Flaky-Failure Reproduction — Technical Specification

## Ownership

`internal/flakerepro/runner.go` owns plan validation, bounded execution, correlation, classification, and summary calculation. `internal/workflowverify/runner.go` supplies workflow failures that can be reproduced without coupling the generic reproduction package to browser actions.

## Model

An attempt has explicit environmental dimensions and executes through an injected executor. The runner deep-copies original workflow evidence, assigns a child correlation ID, checks cancellation before each attempt, and records the executor result. Classification is a pure aggregation over recorded outcomes.

## Invariants

- Attempt count is between 1 and 20.
- Cache state is `preserve`, `cold`, or `warm`.
- Tab lifecycle is `none`, `reload`, `reconnect`, or `navigate`.
- Original failure data cannot be replaced by retry output.
- Perturbed and unperturbed reproduction rates are reported separately.
- No wall-clock sleep is part of correctness.

## Change Boundary

Runner model, validation, verdict rules, and tests move together. Browser-specific perturbation execution remains outside this package.
