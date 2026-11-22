---
doc_type: tech-spec
feature_id: feature-transient-capture
status: shipped
last_reviewed: 2026-11-21
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Transient Capture Tech Spec

## Architecture
- Content-side detection logic: `src/lib/page/transient-capture.ts`
- Daemon-side async result enrichment: `internal/tools/asyncresult/transients.go` (`AttachTransientElements`, bounded by `MaxTransientsPerResult`)
- Observe retrieval: `observe({what: "transients"})` via `GetTransients` in `internal/tools/observe/session/session.go`, dispatched from `internal/tools/observe/handler/dispatcher.go`
- Observe integration tests: `internal/tools/observe/session/session_transients_test.go`

## Constraints
- Snapshot extraction must be lightweight and bounded.
- Captures should include correlation and timing metadata.
- Replay/observe consumers must see consistent event ordering.
