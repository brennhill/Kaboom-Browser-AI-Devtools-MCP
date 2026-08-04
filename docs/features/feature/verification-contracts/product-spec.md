---
doc_type: product-spec
feature_id: feature-verification-contracts
last_reviewed: 2026-08-04
---

# Verification Contracts — Product Specification

## Outcome

Developers can define explicit acceptance assertions, attach trustworthy local evidence, and receive an honest verdict that cannot pass when evidence is missing, stale, or modified.

## Requirements

- Define and evaluate schema-versioned contracts through `analyze(what="verification")`.
- Require unique assertion IDs, descriptions, and any required evidence kinds.
- Limit verdicts to `PASS`, `FAIL`, `BLOCKED`, `UNVERIFIED`, and `FLAKY`.
- Return `UNVERIFIED` for missing assertion results or required evidence.
- Redact evidence before content addressing and persistence.
- Persist artifacts atomically with owner-only permissions and resolve them after daemon restart.
- Re-hash and freshness-check referenced evidence during evaluation.
- Accept provenance only from Kaboom's five canonical tools.

## Privacy

Evidence and contracts remain local. Evidence content is redacted before storage and never becomes product telemetry.

## Acceptance

An evaluation can report `PASS` only when every required result and evidence reference is present, authentic, fresh, and bound to the evaluated assertion.
