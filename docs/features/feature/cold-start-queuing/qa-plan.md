---
doc_type: qa-plan
feature_id: feature-cold-start-queuing
status: implemented
last_reviewed: 2026-11-21
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Cold-Start Queuing QA Plan

## Automated Coverage
- `internal/tools/guard/guards_test.go`
- `internal/tools/asynccommand/handler_test.go`

## Required Scenarios
1. Extension connects before timeout: command proceeds.
2. Timeout path returns retryable structured error.
3. `background:true` path skips blocking gate.
4. `coldStartTimeout=0` path fast-fails without wait.
