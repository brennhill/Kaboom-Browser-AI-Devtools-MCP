---
doc_type: qa-plan
feature_id: feature-observe
status: shipped
owners: []
last_reviewed: 2026-07-05
links:
  product: ./product-spec.md
  tech: ./tech-spec.md
  qa: ./qa-plan.md
  feature_index: ./index.md
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Observe QA Plan (TARGET)

## Automated Coverage
- `cmd/browser-agent/internal/toolobserve/dispatcher_commands_test.go`
- `cmd/browser-agent/tools_observe_blackbox_test.go`
- `cmd/browser-agent/tools_observe_audit_test.go`
- `extension/background/commands/observe.fullpage.test.js`

## Required Scenarios
1. Enum contract
- Every `what` value from schema dispatches to a handler.

2. Pagination contract
- Cursor-based navigation works for logs.
- `restart_on_eviction` behavior is verified.

3. Command-result contract
- Pending, complete, error, expired, timeout states are all surfaced.
- Correlation lookup failures return structured terminal guidance.

4. Filtering correctness
- URL/status/method/level filters affect only intended modes.

5. Screenshot contract
- `observe(what:"screenshot")` returns capture metadata or structured timeout/error.
- `observe(what:"screenshot", full_page:true)` captures nested scroll-container content when available, restores container styles after capture, and degrades cleanly to viewport capture on CDP failure.

## Manual UAT
1. Call `configure(what:"health")`.
2. Call `observe(what:"logs")` with and without cursor options.
3. Queue an async command and verify `observe(what:"command_result", correlation_id)`.
4. Disconnect extension and verify warning + guidance surfaces.
