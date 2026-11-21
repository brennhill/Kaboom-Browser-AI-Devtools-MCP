---
doc_type: qa-plan
feature_id: feature-csp-safe-execution
status: implemented
last_reviewed: 2026-11-21
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# CSP-Safe Execution QA Plan

## Automated Coverage
- `tests/extension/capture/csp-safe-integration.test.js`
- `tests/extension/injection/execute-js.test.js`
- `extension/background/exec/__tests__/query-execution-serialization.test.js`

## Required Scenarios
1. MAIN world success path.
2. CSP-blocked fallback to ISOLATED/structured path.
3. Unsupported structured expression returns parse/validation error.
4. Host-object serialization returns meaningful JSON, not empty objects.
