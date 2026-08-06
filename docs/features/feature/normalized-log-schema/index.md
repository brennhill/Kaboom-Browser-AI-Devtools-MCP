---
doc_type: feature_index
feature_id: feature-normalized-log-schema
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-08-06
code_paths:
  - internal/types/wire_log.go
  - internal/capture/logstore/store.go
  - internal/capture/pressure/stats.go
  - src/types/wire/wire-extension-log.ts
test_paths:
  - internal/capture/logstore/store_test.go
  - internal/capture/logstore/diagnostic_test.go
  - internal/capture/syncruntime/sync_test_helpers_test.go
  - scripts/contracts/sync-wire-generated.test.cjs
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# Normalized Log Schema

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/protocol/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/protocol/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/normalized-log-schema`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_NORMALIZED_LOG_SCHEMA_001
- FEATURE_NORMALIZED_LOG_SCHEMA_002
- FEATURE_NORMALIZED_LOG_SCHEMA_003

## Code and Tests

Extension diagnostic entries have one Go wire owner. The generated TypeScript
shape is embedded in the canonical `/sync` graph, while the capture store owns
normalization, redaction, and retention after ingestion. A shared JSON fixture
must round-trip through Go and match the generated extension feature contract.

The normalized log store is a direct package boundary rather than a root
compatibility facade. Its bounded retention reports the shared resource
pressure contract, including capacity, cumulative drops, and oldest-entry age.
