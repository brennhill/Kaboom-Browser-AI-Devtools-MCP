---
doc_type: feature_index
feature_id: feature-cold-start-queuing
status: implemented
feature_type: feature
owners: []
last_reviewed: 2026-07-27
code_paths:
  - cmd/browser-agent/tools_core.go
  - cmd/browser-agent/tools_async_completion.go
  - internal/capture/extension_state.go
test_paths:
  - cmd/browser-agent/tools_coldstart_gate_test.go
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
