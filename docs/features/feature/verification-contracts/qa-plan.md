---
doc_type: qa-plan
feature_id: feature-verification-contracts
last_reviewed: 2026-08-04
---

# Verification Contracts — QA Plan

## Automated Coverage

- `internal/verification/contract_test.go` covers schema validation and verdict aggregation.
- `internal/verification/evidence_test.go` covers redaction, hashing, provenance, authenticity, and freshness.
- `internal/verification/store_test.go` covers atomic persistence, permissions, and restart lookup.
- `internal/state/paths_test.go` covers deterministic state paths.
- `cmd/browser-agent/internal/toolanalyze/verificationhandler/handler_test.go` covers define/evaluate request handling.

## Required Scenarios

1. Reject unsupported schema versions, duplicate assertions, and missing required fields.
2. Evaluate complete passing assertions with authentic fresh evidence as `PASS`.
3. Omit a result or required evidence and expect `UNVERIFIED`.
4. Modify persisted evidence and expect `UNVERIFIED` after re-hashing.
5. Age evidence beyond the configured window and expect `UNVERIFIED`.
6. Submit provenance outside the five canonical tools and reject it.
7. Restart the store and resolve an unchanged artifact by hash.
8. Verify file permissions and scan persisted bytes for unredacted sensitive fixtures.

## Release Gate

Run `go test ./internal/verification ./internal/state ./cmd/browser-agent/internal/toolanalyze/verificationhandler` plus wire-drift and schema contract checks.
