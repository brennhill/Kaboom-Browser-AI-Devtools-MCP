---
doc_type: tech-spec
feature_id: feature-verification-contracts
last_reviewed: 2026-08-04
---

# Verification Contracts — Technical Specification

## Boundaries

- `internal/verification/contract.go` owns schema validation and verdict evaluation.
- `internal/verification/evidence.go` owns evidence normalization, redaction, hashing, authenticity, and freshness checks.
- `internal/verification/store.go` owns atomic local persistence and lookup.
- `internal/state/paths.go` owns the state location.
- `cmd/browser-agent/internal/toolanalyze/verificationhandler` implements the action boundary.
- `internal/schema/analyze.go` and the analyze mode specification expose the public contract.

## Invariants

- Schema version `1` is explicit.
- Contract and assertion identifiers are required and assertion IDs are unique.
- Evidence hashes address the redacted bytes actually stored.
- Evaluation revalidates content hashes and age; lookup success alone is insufficient.
- The default freshness window is 24 hours unless the contract supplies `max_age_seconds`.
- Missing or invalid proof never degrades to success.
- Writes are atomic and owner-only.

## Change Boundary

Public schema, handler decoding, domain validation, persistence, and tests must change atomically when the contract evolves.
