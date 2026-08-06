---
doc_type: qa-plan
feature_id: feature-framework-selector-resilience
status: shipped
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Framework Selector Resilience QA Plan

## Automated Coverage

1. `scripts/uat/runners/smoke-test.sh --only 29`
2. `scripts/smoke-tests/framework/29-framework-selector-resilience.sh`

## Required Scenarios

1. Build pipeline emits all four framework fixtures.
2. `analyze(page_structure)` detects each expected framework.
3. Hydration gate blocks actions until `#hydrated-ready`.
4. Overlay dismissal path (`Accept Cookies`) is reliable.
5. Async delayed content appears and is actionable.
6. Virtualized/lazy deep target appears after scroll and is actionable.
7. Stale `element_id` still resolves after route remount churn.
8. Selector churn changes per refresh while interactions remain successful across 3 cycles.

## Failure Triage Checklist

1. Confirm fixture build output exists under `cmd/browser-agent/internal/testpages/pages/frameworks/`.
2. Check diagnostics for first failing framework and step.
3. Confirm framework detection payload from `analyze(page_structure)`.
4. Validate that fixture semantic contract IDs/text still exist.
5. Re-run single module with `CI=1 bash scripts/uat/runners/smoke-test.sh 7890 --only 29`.
