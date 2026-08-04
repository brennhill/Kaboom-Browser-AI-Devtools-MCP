---
doc_type: product-spec
feature_id: feature-flake-detection
last_reviewed: 2026-08-04
---

# Continuous Flake Detection — Product Specification

## Outcome

Maintainers can reproduce intermittent CI failures from a machine-readable artifact instead of rerunning an opaque job until it fails again.

## Requirements

- Run Go and JavaScript tests repeatedly with an explicit seed, shuffled order, race detection, and bounded resource pressure.
- Record the exact commands, ordering, concurrency, duration, bounded redacted output, and initial failure without changing it during retries.
- Retry each failed command twice under the original conditions and classify it as reproduced, intermittent, or flaky.
- Preserve a failing campaign exit status even when a retry passes.
- Disable production telemetry for every campaign process.
- Upload the result artifact even when the campaign or artifact validation fails.
- Support scheduled and manually dispatched GitHub runs and local replay from one recorded command.

## Boundaries

This feature detects and preserves test-suite flakiness. It does not conceal failures, automatically quarantine tests, or mutate the test suite.

## Acceptance

A seeded campaign produces the same command plan, every failure retains replay evidence, missing evidence fails the workflow, and no retry can turn the campaign green.
