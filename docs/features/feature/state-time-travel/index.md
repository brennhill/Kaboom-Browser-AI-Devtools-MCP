---
doc_type: feature_index
feature_id: feature-state-time-travel
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - cmd/browser-agent/internal/toolinteract/interactstate/state.go
  - cmd/browser-agent/tools_interact_dispatch.go
  - internal/schema/interact/actions.go
  - src/inject/state.ts
test_paths:
  - cmd/browser-agent/internal/toolinteract/interactstate/state_test.go
  - cmd/browser-agent/tools_interact_state_test.go
  - internal/schema/interact/schema_test.go
  - tests/extension/pilot-state.test.js
  - tests/extension/no-compatibility-facades.test.js
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
  - `src/inject/state.ts`
- Public `interact` action routing:
  - `cmd/browser-agent/tools_interact_dispatch.go`
- Canonical public action schemas:
  - `internal/schema/interact/actions.go`
- Tests:
  - `cmd/browser-agent/internal/toolinteract/interactstate/state_test.go`
  - `cmd/browser-agent/tools_interact_state_test.go`
  - `internal/schema/interact/schema_test.go`
  - `tests/extension/pilot-state.test.js`
  - `tests/extension/no-compatibility-facades.test.js`
