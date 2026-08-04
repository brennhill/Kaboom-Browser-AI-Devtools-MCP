---
doc_type: feature_index
feature_id: feature-hook-eval-rig
status: implemented
feature_type: feature
owners: []
last_reviewed: 2026-08-04
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

A deterministic evaluation framework for measuring the real-world impact of kaboom-hooks on AI coding sessions. Functional fixture contracts run independently of scheduler timing, while a serial production-performance evaluation exercises the real repository and enforces each declared latency budget. This keeps unit results stable under suite load without weakening the real hook SLO. Three tiers of testing cover unit-level hook evals (synthetic inputs, known-good outputs), integration evals (controlled codebases with known dependency graphs), and live session metrics (real usage data aggregated across sessions).

Fixture execution receives one explicit runtime containing the hook runner and
latency measurement boundary. Production uses a wall-clock measurement;
contract tests inject exact elapsed durations, proving latency enforcement and
contract-mode exclusion without sleeping.

The rig answers: "Do these hooks actually make AI coding better, and by how much?"
