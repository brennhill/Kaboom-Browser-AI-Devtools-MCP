---
doc_type: feature_index
feature_id: feature-state-time-travel
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-05
code_paths:
  - cmd/browser-agent/tools_interact_state_handler.go
  - cmd/browser-agent/tools_interact_state_capture.go
  - cmd/browser-agent/tools_interact_state_save_load.go
  - cmd/browser-agent/tools_interact_state_list_delete.go
test_paths:
  - cmd/browser-agent/tools_interact_state_test.go
  - cmd/browser-agent/tools_interact_gate_test.go
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

- State sub-handler wiring:
  - `cmd/browser-agent/tools_interact_state_handler.go`
- State capture and restore queueing:
  - `cmd/browser-agent/tools_interact_state_capture.go`
- Save/load handlers:
  - `cmd/browser-agent/tools_interact_state_save_load.go`
- List/delete handlers:
  - `cmd/browser-agent/tools_interact_state_list_delete.go`
- Tests:
  - `cmd/browser-agent/tools_interact_state_test.go`
  - `cmd/browser-agent/tools_interact_gate_test.go`
