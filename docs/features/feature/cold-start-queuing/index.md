---
doc_type: feature_index
feature_id: feature-cold-start-queuing
status: implemented
feature_type: feature
owners: []
last_reviewed: 2026-08-04
code_paths:
  - cmd/browser-agent/tools_core.go
  - cmd/browser-agent/internal/asynccommand/handler.go
  - internal/capture/extension_state.go
test_paths:
  - internal/capture/readiness_gate_test.go
  - cmd/browser-agent/tools_coldstart_gate_test.go
  - cmd/browser-agent/tools_async_timeout_test.go
  - cmd/browser-agent/tools_core_sync_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Cold-Start Queuing

## TL;DR
- Status: implemented
- Scope: extension-readiness gating before tool execution
- Default timeout: 5s

## Specs
- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Canonical Note
Readiness wait occurs once at the gate; async/background paths return queued immediately and should not double-block.
Tests construct the full capture/guard dependency pair through shared fixtures so readiness diagnostics exercise the production object graph.
Extension connection changes close and rotate a generation notification under
the extension-state lock. Readiness waiters snapshot state and that channel
atomically, then select on connection, cancellation, or one bounded timer; no
poll interval or scheduler sleep participates in command readiness.
Async timeout adapter tests likewise coordinate command enqueue and completion
through the query dispatcher; elapsed wall time is reserved for the timeout
contract itself, not used to schedule successful completion.
