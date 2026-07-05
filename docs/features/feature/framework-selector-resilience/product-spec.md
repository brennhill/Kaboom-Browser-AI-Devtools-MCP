---
doc_type: product-spec
feature_id: feature-framework-selector-resilience
status: shipped
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Framework Selector Resilience Product Spec

## Problem

Automation that passes on static test pages can still fail on modern framework apps due to:
- hydration/event-binding races
- overlay interception
- DOM remount churn invalidating handles
- delayed async UI states
- virtualized/lazy targets that appear only after scroll

## Product Requirement

Provide smoke coverage that uses real framework runtimes and continuously validates these failure modes.

## Shipped Behavior

Module `29-framework-selector-resilience.sh` now runs against:
1. React fixture
2. Vue fixture
3. Svelte fixture
4. Next.js fixture

For each framework, smoke coverage verifies:
1. Framework detection via `analyze(page_structure)`
2. Hydration readiness gating before interaction
3. Consent/overlay dismissal path
4. Async delayed content discovery and action (`Async Save`)
5. Virtualized/lazy deep target discovery and action (`Framework Target 80`)
6. Route remount churn with stale `element_id` recovery
7. Selector churn resilience across 3 refresh cycles

## Non-Goals

1. Full browser-framework compatibility certification across every framework ecosystem variant.
2. Replacing dedicated E2E suites for product features.

## User Value

This reduces false confidence from simplistic fixtures and catches real-world selector/action regressions earlier, before users experience failures in modern SPA frameworks.
