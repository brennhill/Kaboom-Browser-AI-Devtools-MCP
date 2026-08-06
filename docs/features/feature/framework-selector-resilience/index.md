---
doc_type: feature_index
feature_id: feature-framework-selector-resilience
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-27
code_paths:
  - scripts/smoke-tests/29-framework-selector-resilience.sh
  - scripts/smoke-tests/build-framework-fixtures.mjs
  - scripts/uat/runners/smoke-test.sh
  - package.json
  - scripts/smoke-tests/framework-fixtures/react-entry.jsx
  - scripts/smoke-tests/framework-fixtures/vue-entry.js
  - scripts/smoke-tests/framework-fixtures/SmokeSvelteApp.svelte
  - scripts/smoke-tests/framework-fixtures/next-app/pages/index.jsx
  - scripts/smoke-tests/framework-fixtures/README.md
test_paths:
  - scripts/smoke-tests/29-framework-selector-resilience.sh
  - scripts/smoke-tests/29-framework-selector-resilience.sh
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Framework Selector Resilience

## TL;DR
- Status: shipped
- Surface: smoke module `29-framework-selector-resilience.sh`
- Scope: real-framework fixture coverage for hard automation failure modes

## Specs
- Product: `product-spec.md`
- Tech: `tech-spec.md`
- QA: `qa-plan.md`
- Flow Map: `flow-map.md`

## Canonical Note
This feature makes fixture coverage a first-class capability: Kaboom validates automation resilience against real React, Vue, Svelte, and Next.js pages that model hydration races, overlays, remount churn, async delayed UI, and virtualized content.

The parity gate supports explicit repeat budgets:
- `FRAMEWORK_RESILIENCE_FULL_REPEATS` (full scenario repeats per framework)
- `FRAMEWORK_SELECTOR_REFRESH_CYCLES` (refresh loops per scenario)
