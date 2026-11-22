---
doc_type: feature_index
feature_id: feature-batch-sequences
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-09-05
code_paths:
  - internal/recording/replay/contract.go
  - internal/recording/sequence/handler.go
  - internal/recording/sequence/replay.go
  - internal/recording/sequence/contract.go
  - internal/tools/interact/handler/interactbatch/batch.go
  - internal/composition/toolruntime/tools_interact_dispatch.go
  - internal/composition/toolruntime/tools_configure.go
  - internal/tools/configure/handler/dispatcher.go
  - internal/recording/actionlog/recorder.go
  - internal/tools/interact/workflow.go
  - internal/tools/schema/interact/actions.go
  - internal/tools/schema/interact/properties_output_batch.go
  - internal/tools/configure/capabilities/modespecs_interact.go
test_paths:
  - tests/architecture/user-state-loaders.test.cjs
  - internal/tools/configure/handler/dispatcher_test.go
  - internal/recording/replay/contract_test.go
  - internal/recording/sequence/handler_test.go
  - internal/recording/sequence/contract_test.go
  - internal/tools/interact/handler/interactbatch/batch_test.go
  - internal/tools/schema/interact/schema_test.go
  - internal/recording/actionlog/recorder_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Batch Sequences

## TL;DR
- Status: shipped
- Tools: `interact`, `configure`
- Actions: `batch`, `save_sequence`, `replay_sequence`
- Step results use the canonical `internal/recording/replay.StepResult` contract directly.

## Specs
- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Design Reference: [design-spec.md](./design-spec.md)

## Canonical Note
Batch execution and reusable configure sequences share replay primitives and one
runtime-scoped concurrency lock. Saved-sequence persistence, CRUD, and replay orchestration stay
together in `internal/recording/sequence`; interactive batch orchestration remains
in the dedicated `interactbatch` owner. The former package-global lock and
unused action-runtime dependency are deleted. Configure actions and saved interact steps both use
the canonical `what` discriminator. Tool composition constructs one
`sequencehandler.Handler`, and all five configure sequence actions route to it
directly; no ToolHandler forwarding methods or per-request handler factory
remain. Batch and replay audit entries use the same
`internal/recording/actionlog.Recorder` as direct interact actions.
Malformed or unreadable saved sequences are isolated from valid siblings and
omitted from listings. The handler reports the recovery through System Doctor
without retaining the saved payload.
Replay response parsing, including structured, plain-text, empty, and malformed
errors, is tested directly beside the canonical replay contract.
Read, list, write, and delete failures now share that same redacted Doctor
incident; raw storage errors are never echoed through MCP. Canonical state-fault
tests also prove corruption, partial writes, cancellation, and restart reloads.
The sequence handler owns its persistence namespace, validation limits, name
pattern, and saved wire models directly. It no longer imports configure merely
to reach sequence contracts.
