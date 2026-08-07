---
doc_type: feature_index
feature_id: feature-batch-sequences
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-07
code_paths:
  - cmd/browser-agent/internal/replay/contract.go
  - cmd/browser-agent/internal/sequencehandler/handler.go
  - cmd/browser-agent/internal/sequencehandler/replay.go
  - cmd/browser-agent/internal/sequencehandler/contract.go
  - cmd/browser-agent/internal/toolinteract/action_owners.go
  - cmd/browser-agent/tools_interact_dispatch.go
  - cmd/browser-agent/tools_configure.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher.go
  - internal/recording/actionlog/recorder.go
  - internal/tools/interact/workflow.go
  - internal/schema/interact/actions.go
  - internal/schema/interact/properties_output_batch.go
  - internal/tools/configure/capabilities/modespecs_interact.go
test_paths:
  - tests/architecture/user-state-loaders.test.cjs
  - cmd/browser-agent/internal/toolconfigure/dispatcher_test.go
  - cmd/browser-agent/internal/replay/contract_test.go
  - cmd/browser-agent/internal/sequencehandler/handler_test.go
  - cmd/browser-agent/internal/sequencehandler/contract_test.go
  - cmd/browser-agent/tools_interact_batch_test.go
  - internal/recording/actionlog/recorder_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Batch Sequences

## TL;DR
- Status: shipped
- Tools: `interact`, `configure`
- Actions: `batch`, `save_sequence`, `replay_sequence`
- Step results use the canonical `internal/replay.StepResult` contract directly.

## Specs
- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Design Reference: [design-spec.md](./design-spec.md)

## Canonical Note
Batch execution and reusable configure sequences share replay primitives and one
concurrency lock. Saved-sequence persistence, CRUD, and replay orchestration stay
together in `internal/sequencehandler`; interactive batch orchestration remains
with the interact feature. Configure actions and saved interact steps both use
the canonical `what` discriminator. Tool composition constructs one
`sequencehandler.Handler`, and all five configure sequence actions route to it
directly; no ToolHandler forwarding methods or per-request handler factory
remain. Batch and replay audit entries use the same
`internal/recording/actionlog.Recorder` as direct interact actions.
Malformed or unreadable saved sequences are isolated from valid siblings and
omitted from listings. The handler reports the recovery through System Doctor
without retaining the saved payload.
Read, list, write, and delete failures now share that same redacted Doctor
incident; raw storage errors are never echoed through MCP. Canonical state-fault
tests also prove corruption, partial writes, cancellation, and restart reloads.
The sequence handler owns its persistence namespace, validation limits, name
pattern, and saved wire models directly. It no longer imports configure merely
to reach sequence contracts.
