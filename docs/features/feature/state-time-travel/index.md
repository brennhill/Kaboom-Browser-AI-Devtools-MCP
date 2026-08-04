---
doc_type: feature_index
feature_id: feature-state-time-travel
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-30
code_paths:
  - cmd/browser-agent/internal/toolinteract/interactstate/state.go
  - cmd/browser-agent/tools_core.go
  - cmd/browser-agent/tools_interact_dispatch.go
  - internal/recording/actionlog/recorder.go
  - internal/schema/interact/actions.go
  - internal/statediag/collector.go
  - src/inject/message-handlers.ts
  - src/inject/settings.ts
  - src/inject/state.ts
test_paths:
  - cmd/browser-agent/lint_hardening_test.go
  - cmd/browser-agent/internal/toolinteract/interactstate/state_test.go
  - internal/recording/actionlog/recorder_test.go
  - cmd/browser-agent/tools_interact_gate_test.go
  - cmd/browser-agent/tools_interact_helpers_test.go
  - cmd/browser-agent/tools_interact_state_test.go
  - internal/schema/interact/schema_test.go
  - tests/extension/pilot/pilot-state.test.js
  - tests/extension/injection/inject-settings.test.js
  - tests/extension/contracts/no-compatibility-facades.test.js
  - tests/architecture/user-state-loaders.test.cjs
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# State Time Travel

## TL;DR

- Status: proposed
- Tool: observe
- Mode/Action: history
- Location: `docs/features/feature/state-time-travel`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_STATE_TIME_TRAVEL_001
- FEATURE_STATE_TIME_TRAVEL_002
- FEATURE_STATE_TIME_TRAVEL_003

## Code and Tests

- State capture, persistence, restore, listing, and deletion:
  - `cmd/browser-agent/internal/toolinteract/interactstate/state.go`
  - `src/inject/message-handlers.ts`
  - `src/inject/settings.ts`
  - `src/inject/state.ts`
- Inject-context state results return through the canonical nonce-authenticated
  response boundary. Unauthenticated page messages cannot satisfy a pending
  capture or restore request.
- Save, load, and delete share one request-validation boundary for canonical
  snapshot naming and session-store readiness.
- The handler package tests install a package-owned temporary state root, so
  recursive and restricted-home runs never touch the user's `~/.kaboom`.
- Public `interact` action routing:
  - `cmd/browser-agent/tools_interact_dispatch.go`
- Composition owns the canonical state handler directly:
  - `cmd/browser-agent/tools_core.go`
  - dispatch and state-focused tests access `stateInteractHandler` without an
    unchanged-return accessor facade
- Save, load, and delete audit actions route through the canonical
  `internal/recording/actionlog.Recorder`; the composition root has no parallel
  action-recording methods.
- Snapshot read and schema failures never restore partial state. They retain a
  safe list entry and report the redacted `page_snapshot_state` recovery to
  System Doctor.
- State-capture tests share one extension-response driver that waits on the
  query dispatcher's notification barrier, validates the expected capture
  command, and propagates JSON encoding failures. Form, storage, redaction,
  legacy-shape, and execution-failure scenarios no longer infer readiness from
  wall-clock sleeps.
- Canonical public action schemas:
  - `internal/schema/interact/actions.go`
- Tests:
  - `cmd/browser-agent/lint_hardening_test.go`
  - `cmd/browser-agent/internal/toolinteract/interactstate/state_test.go`
  - `cmd/browser-agent/tools_interact_gate_test.go`
  - `cmd/browser-agent/tools_interact_helpers_test.go`
  - `cmd/browser-agent/tools_interact_state_test.go`
  - `internal/schema/interact/schema_test.go`
  - `tests/extension/pilot/pilot-state.test.js`
  - `tests/extension/injection/inject-settings.test.js`
  - `tests/extension/contracts/no-compatibility-facades.test.js`
