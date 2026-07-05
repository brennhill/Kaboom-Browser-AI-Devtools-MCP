---
doc_type: product-spec
feature_id: feature-csp-safe-execution
status: implemented
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# CSP-Safe Execution Product Spec

## Purpose
Keep `execute_js` functional on CSP-restricted pages by using deterministic fallback routing instead of hard failure.

## Requirements
- `CSP_SAFE_PROD_001`: attempt MAIN-world execution when allowed.
- `CSP_SAFE_PROD_002`: fallback to ISOLATED world when MAIN eval path is blocked.
- `CSP_SAFE_PROD_003`: fallback to structured executor when eval paths fail but supported expression subset is sufficient.
- `CSP_SAFE_PROD_004`: responses include execution mode metadata for debugging and triage.
