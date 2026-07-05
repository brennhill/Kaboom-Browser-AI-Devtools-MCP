---
doc_type: feature_index
feature_id: feature-hook-eval-rig
status: implemented
feature_type: feature
owners: []
last_reviewed: 2026-07-05
code_paths:
  - internal/hook/eval/eval.go
  - internal/hook/eval/testdata/
test_paths:
  - internal/hook/eval/eval_test.go
---

# Hook Eval Rig

| Field         | Value                                   |
|---------------|-----------------------------------------|
| **Status**    | implemented (Tier 1)                    |
| **Binary**    | kaboom-hooks                          |
| **Command**   | `kaboom-hooks eval`                   |
| **Purpose**   | Measure token savings, accuracy, and redundancy elimination |
| **Parent**    | [Quality Gates](../quality-gates/index.md) |

## Specs

- [Product Spec](./product-spec.md)
- [Tech Spec](./tech-spec.md)

## Summary

A deterministic evaluation framework for measuring the real-world impact of kaboom-hooks on AI coding sessions. Three tiers of testing: unit-level hook evals (synthetic inputs, known-good outputs), integration evals (controlled codebases with known dependency graphs), and live session metrics (real usage data aggregated across sessions).

The rig answers: "Do these hooks actually make AI coding better, and by how much?"
