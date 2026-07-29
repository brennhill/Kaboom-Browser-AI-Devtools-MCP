---
doc_type: feature_index
feature_id: feature-custom-event-api
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-29
code_paths:
  - src/inject/api.ts
  - src/inject.ts
  - scripts/build/bundle-content.js
test_paths:
  - tests/extension/injection/inject-context-api-actions.test.js
  - tests/extension/injection/inject-v5-wiring.test.js
  - tests/extension/performance/performance-marks.test.js
  - tests/extension/network-http/network-waterfall.test.js
  - tests/extension/contracts/no-compatibility-facades.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Custom Event Api

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/custom-event-api`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_CUSTOM_EVENT_API_001
- FEATURE_CUSTOM_EVENT_API_002
- FEATURE_CUSTOM_EVENT_API_003

## Code and Tests

The Kaboom developer API is exposed through `window.__kaboom`, and the injected bundle version contract is defined through the `__KABOOM_VERSION__` build symbol in `scripts/build/bundle-content.js`.
`src/inject.ts` owns runtime installation only; API consumers import
`src/inject/api.ts` directly, with no bundle compatibility facade.
