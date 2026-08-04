---
doc_type: qa-plan
feature_id: feature-operational-observability
last_reviewed: 2026-08-04
---

# Operational Observability — QA Plan

## Automated Coverage

- `internal/incident/store_test.go` and `support_test.go` cover lifecycle, capacity, generation rejection, projections, preview invalidation, redaction, and file permissions.
- `internal/statediag/collector_test.go` covers bounded redacted state diagnostics.
- `internal/telemetry/contract_compliance_test.go` and `beacon_test.go` cover the privacy allowlist, delivery outcomes, deduplication, queue pressure, panic recovery, and shutdown.
- `cmd/browser-agent/tools_configure_support_test.go` covers the public support workflow.
- Packaged recovery checks live in `tests/cli/contracts/packaged-recovery-uat.test.cjs` and `scripts/tests/release/cat-34-packaged-corruption-recovery.sh`.

## Required Scenarios

1. Record, retry, recover, and exhaust incidents; verify Doctor history and telemetry projections agree on fixed classifications.
2. Submit stale-generation transitions and verify current health is unchanged.
3. Saturate stores and queues; verify bounded memory and accurate dropped counts.
4. Panic and reject in the telemetry transport; verify the worker survives and delivery state is honest.
5. Preview support evidence, mutate incident state, and reject the stale export token.
6. Scan telemetry and support artifacts for URLs, paths, content, credentials, correlations, and raw evidence.
7. Start packaged builds with corrupt state and verify startup, fallback, Doctor visibility, and redacted logs.

## Release Gate

Run the listed Go, JavaScript, architecture, packaged recovery, privacy, and strict documentation tests before release.
