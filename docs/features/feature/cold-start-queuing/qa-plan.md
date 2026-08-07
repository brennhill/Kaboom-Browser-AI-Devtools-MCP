---
doc_type: qa-plan
feature_id: feature-cold-start-queuing
status: implemented
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Cold-Start Queuing QA Plan

## Automated Coverage
- `cmd/browser-agent/internal/toolguard/guards_test.go`
- `cmd/browser-agent/internal/asynccommand/handler_test.go`

## Required Scenarios
1. Extension connects before timeout: command proceeds.
2. Timeout path returns retryable structured error.
3. `background:true` path skips blocking gate.
4. `coldStartTimeout=0` path fast-fails without wait.
