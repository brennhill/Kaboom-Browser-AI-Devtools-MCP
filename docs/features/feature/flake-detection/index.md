---
doc_type: feature_index
feature_id: feature-flake-detection
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-03
code_paths:
  - scripts/ci/run-flake-detection.mjs
  - .github/workflows/flake-detection.yml
  - internal/flakerepro/runner.go
test_paths:
  - scripts/ci/run-flake-detection.test.mjs
  - internal/flakerepro/runner_test.go
---

# Continuous Flake Detection

Kaboom runs a scheduled shuffled, repeated, race-enabled test campaign under
explicit resource pressure. The same runner is available locally:

```bash
KABOOM_FLAKE_SEED=12345 node scripts/ci/run-flake-detection.mjs
```

The machine-readable artifact records the seed, exact Go package and JavaScript
file order, command arguments, concurrency settings, timing, bounded redacted
output, original failures, and reproduction attempts. Its `replay_command`
recreates the campaign with one command.

Every original failing command is retained immutably and rerun twice with the
same seed, order, and pressure. Retries classify the failure as reproduced,
intermittent, or flaky; they never change the campaign exit code from failure to
success. Production telemetry is disabled throughout the campaign.

The scheduled GitHub workflow runs twice weekly and supports manual dispatch
with an explicit seed, run count, and concurrency. Evidence uploads run with
`if: always()` and fail when the result artifact is missing, so infrastructure
failures cannot silently erase the replay context.
