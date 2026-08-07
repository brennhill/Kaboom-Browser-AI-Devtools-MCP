---
doc_type: feature_index
feature_id: feature-buffer-clearing
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-07
code_paths:
  - cmd/browser-agent/internal/toolconfigure/clear.go
  - cmd/browser-agent/tools_configure.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher.go
  - internal/capture/bodystore/store.go
  - internal/capture/actionstore/store.go
  - internal/capture/wsconn/store.go
test_paths:
  - internal/capture/resetter/resetter_test.go
  - internal/capture/bodystore/store_test.go
  - internal/capture/actionstore/store_test.go
  - internal/capture/wsconn/store_test.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher_test.go
  - cmd/browser-agent/internal/toolconfigure/handlers_coverage_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Buffer Clearing

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/protocol/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/protocol/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/buffer-clearing`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_BUFFER_CLEARING_001
- FEATURE_BUFFER_CLEARING_002
- FEATURE_BUFFER_CLEARING_003

## Code and Tests

`internal/toolconfigure/clear.go` owns request parsing and the clearing policy
for capture, log, push inbox, and annotation stores. The root configure registry
supplies those stores explicitly through `ClearTargets`, including the
canonical `resetter.Resetter` for coordinated full-state resets. `Capture`
has no `ClearAll` forwarding method.
Enhanced actions are cleared directly through the canonical
`capture.Telemetry().Actions()` owner; no telemetry compatibility facade is
retained.
WebSocket events and derived connection status are cleared atomically through
`capture.Telemetry().WebSockets().Clear()`.
