---
doc_type: tech-spec
feature_id: feature-batch-sequences
status: proposed
last_reviewed: 2026-07-27
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Batch Sequences Tech Spec

## Architecture
- Core batch executor: `cmd/browser-agent/internal/toolinteract/action_owners.go`
- Sequence configure boundary: `cmd/browser-agent/tools_configure.go`
- Sequence persistence handlers: `cmd/browser-agent/internal/sequencehandler/handler.go`
- Replay orchestration: `cmd/browser-agent/internal/sequencehandler/replay.go`

## Contract Notes
- Batch step schema is part of interact tool schema (`internal/schema/interact/properties_output_batch.go`).
- Replay should reuse batch execution internals rather than reimplementing per-step behavior.

## Reliability Constraints
- Max step limits must be enforced server-side.
- Nested batch/replay deadlocks must be prevented via replay mutex and validation guards.
- Result payloads should remain bounded and avoid large binary output embedding.
