---
doc_type: feature_index
feature_id: feature-batch-sequences
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-26
code_paths:
  - cmd/browser-agent/internal/toolinteract/interact_batch.go
  - cmd/browser-agent/tools_interact_dispatch.go
  - cmd/browser-agent/tools_configure_sequence.go
  - cmd/browser-agent/tools_configure_sequence_replay.go
  - cmd/browser-agent/tools_configure_sequence_replay_steps.go
  - cmd/browser-agent/tools_configure_sequence_types.go
  - internal/schema/interact/actions.go
  - internal/schema/interact/properties_output_batch.go
  - internal/tools/configure/mode_specs_interact.go
test_paths:
  - cmd/browser-agent/tools_interact_batch_test.go
  - cmd/browser-agent/tools_configure_sequence_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Batch Sequences

## TL;DR
- Status: shipped
- Tools: `interact`, `configure`
- Actions: `batch`, `save_sequence`, `replay_sequence`

## Specs
- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Design Reference: [design-spec.md](./design-spec.md)

## Canonical Note
Batch execution and reusable configure sequences share step semantics; sequence replay should remain a thin layer over the core batch runner to keep behavior DRY and predictable.
