---
doc_type: feature_index
feature_id: feature-flaky-reproduction
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-03
code_paths:
  - internal/flakerepro/runner.go
  - internal/workflowverify/runner.go
test_paths:
  - internal/flakerepro/runner_test.go
  - internal/workflowverify/runner_test.go
---

# Deterministic Flaky-Failure Reproduction

Kaboom reproduces the smallest relevant failed workflow segment with an explicit,
bounded attempt plan. Every attempt declares its latency, CPU pressure, cache
state, and tab lifecycle transition. There are no correctness sleeps or implicit
random perturbations.

The original failed workflow report is deep-copied before execution and remains
part of every result. Attempts receive child correlation IDs such as
`repro-123/retry-001`, retaining their relationship to the original evidence.
The executor cannot mutate or replace the original failure.

Reports separate baseline reproduction, non-reproduction, and reproduction
correlated with an explicit environment perturbation. They include counts and
rates for all three outcomes. Mixed results are `FLAKY`, consistent reproduction
is `FAIL`, no reproduction is `UNVERIFIED`, and cancellation is `BLOCKED`.
A passing retry can therefore never erase or turn the original failure green.

Plans allow at most 20 attempts. Cache state is one of `preserve`, `cold`, or
`warm`; tab lifecycle is `none`, `reload`, `reconnect`, or `navigate`; pressure
values are validated before execution. Cancellation is checked between attempts
and stops further work without a delay loop.
