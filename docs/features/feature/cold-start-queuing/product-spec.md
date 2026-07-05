---
doc_type: product-spec
feature_id: feature-cold-start-queuing
status: implemented
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Cold-Start Queuing Product Spec

## Purpose
Avoid false `no_data` failures when clients issue commands before extension sync has established connectivity.

## Requirements
- `COLD_START_PROD_001`: Gate waits up to configured timeout for extension connectivity.
- `COLD_START_PROD_002`: Timeout failures are retryable with explicit retry hints.
- `COLD_START_PROD_003`: Background mode bypasses blocking wait and returns queued response.
- `COLD_START_PROD_004`: Test environments can disable wait for deterministic fast tests.
