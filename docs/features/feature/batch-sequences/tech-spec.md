---
doc_type: tech-spec
feature_id: feature-batch-sequences
status: proposed
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Batch Sequences Tech Spec

## Architecture
- Core batch executor: `cmd/browser-agent/tools_interact_batch.go`
- Sequence persistence + dispatch: `cmd/browser-agent/tools_configure_sequence.go`
- Replay orchestration: `cmd/browser-agent/tools_configure_sequence_replay.go`
- Replay step execution helpers: `cmd/browser-agent/tools_configure_sequence_replay_steps.go`

## Contract Notes
- Batch step schema is part of interact tool schema (`internal/schema/interact/properties_output_batch.go`).
- Replay should reuse batch execution internals rather than reimplementing per-step behavior.

## Reliability Constraints
- Max step limits must be enforced server-side.
- Nested batch/replay deadlocks must be prevented via replay mutex and validation guards.
- Result payloads should remain bounded and avoid large binary output embedding.
