---
doc_type: feature_index
feature_id: feature-error-bundling
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-08-05
code_paths:
  - src/lib/ai-context/ai-context-parsing.ts
  - src/lib/ai-context/ai-context-enrichment.ts
  - src/lib/page/exceptions.ts
  - src/inject/api.ts
  - internal/capture/bodystore/store.go
test_paths:
  - tests/extension/ai-context/ai-context-fixture.js
  - tests/extension/ai-context/ai-context-frameworks.test.js
  - tests/extension/ai-context/ai-context-pipeline.test.js
  - tests/extension/ai-context/ai-context.test.js
  - tests/extension/ai-context/ai-context-parsing.test.js
  - tests/extension/ai-context/ai-context-enrichment.test.js
  - tests/extension/contracts/no-compatibility-facades.test.js
  - internal/capture/bodystore/store_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Error Bundling

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/error-bundling`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_ERROR_BUNDLING_001
- FEATURE_ERROR_BUNDLING_002
- FEATURE_ERROR_BUNDLING_003

## Code and Tests

Add concrete implementation and test links here as this feature evolves.

Chrome and Firefox stack formats delegate matched-frame validation and
construction to one canonical decoder; format-specific wrappers own only their
regular expressions.
