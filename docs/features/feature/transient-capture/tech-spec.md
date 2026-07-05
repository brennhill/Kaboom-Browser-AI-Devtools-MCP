---
doc_type: tech-spec
feature_id: feature-transient-capture
status: shipped
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Transient Capture Tech Spec

## Architecture
- Content-side detection logic: `src/lib/transient-capture.ts`
- Daemon-side async wiring and retrieval: `cmd/browser-agent/tools_async_transient.go`
- Observe integration tests: `internal/tools/observe/handlers_transients_test.go`

## Constraints
- Snapshot extraction must be lightweight and bounded.
- Captures should include correlation and timing metadata.
- Replay/observe consumers must see consistent event ordering.
