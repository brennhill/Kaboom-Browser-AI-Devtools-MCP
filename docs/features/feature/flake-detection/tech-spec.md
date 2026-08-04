---
doc_type: tech-spec
feature_id: feature-flake-detection
last_reviewed: 2026-08-04
---

# Continuous Flake Detection — Technical Specification

## Ownership

`scripts/ci/run-flake-detection.mjs` owns campaign planning, process execution, bounded output capture, reproduction, classification, and artifact serialization. `.github/workflows/flake-detection.yml` owns scheduling and evidence upload. `internal/flakerepro` supplies the deterministic Go reproduction model also used by workflow verification.

## Contracts

- The environment seed is the only source of campaign ordering.
- Original command results are immutable inputs to reproduction.
- Reproduction attempts reuse the seed, order, and declared pressure.
- Output is bounded and redacted before it enters the artifact.
- `replay_command` contains everything required to recreate the campaign locally.
- Artifact absence is a workflow error, including after an earlier command failure.

## Failure Semantics

Process launch errors, timeouts, non-zero exits, and missing artifacts remain failures. Retries add diagnostic classification only. Telemetry opt-out is inherited by every child process.

## Change Boundary

Campaign behavior and its JavaScript tests change together. Changes to reusable reproduction verdicts or attempt semantics also update `internal/flakerepro` and its Go tests.
