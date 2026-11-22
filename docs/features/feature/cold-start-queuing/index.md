---
doc_type: feature_index
feature_id: feature-cold-start-queuing
status: implemented
feature_type: feature
owners: []
last_reviewed: 2026-11-21
code_paths:
  - cmd/browser-agent/internal/toolruntime/tools_core.go
  - internal/tools/guard/guards.go
  - internal/tools/asynccommand/handler.go
  - internal/sync/runtime/runtime.go
test_paths:
  - internal/tools/guard/guards_test.go
  - internal/lifecycle/bridge/bridge_startup_test.go
  - internal/sync/runtime/readiness_gate_test.go
  - internal/tools/guard/guards_test.go
  - internal/tools/asynccommand/handler_test.go
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
The bounded default is owned by `toolguard`, which applies the policy. Capture
only owns the live connection transition and wait primitive; it no longer
exports command-dispatch timeout policy.
Tests construct the full capture/guard dependency pair through shared fixtures so readiness diagnostics exercise the production object graph.
Extension connection changes close and rotate a generation notification under
the extension-state lock. Readiness waiters snapshot state and that channel
atomically, then select on connection, cancellation, or one bounded timer; no
poll interval or scheduler sleep participates in command readiness.
Async timeout adapter tests likewise coordinate command enqueue and completion
through the query dispatcher; elapsed wall time is reserved for the timeout
contract itself, not used to schedule successful completion.
Cold-start gate tests race an explicit connection transition against the
readiness waiter and require a successful result. They do not delay connection
to approximate waiter registration or assert scheduler-dependent elapsed bands.
