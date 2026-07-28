---
doc_type: feature_index
feature_id: feature-persistent-memory
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - cmd/browser-agent/internal/toolconfigure/session.go
  - internal/tools/configure/capabilities/modespecs_configure.go
test_paths:
  - cmd/browser-agent/tools_configure_handler_test.go
  - cmd/browser-agent/tools_configure_persistence_actions_test.go
  - cmd/browser-agent/tools_configure_capabilities_test.go
  - cmd/browser-agent/tools_configure_audit_test.go
  - cmd/browser-agent/tools_stdio_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Persistent Memory

## TL;DR

- Status: shipped
- Tool: configure
- Mode/Action: store, load, record_event
- Location: `docs/features/feature/persistent-memory`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_PERSISTENT_MEMORY_001
- FEATURE_PERSISTENT_MEMORY_002
- FEATURE_PERSISTENT_MEMORY_003

## Code and Tests

- `cmd/browser-agent/internal/toolconfigure/session.go` owns the canonical store request contract: `store_action`, `namespace`, `key`, and `data`.
- `internal/tools/configure/capabilities/modespecs_configure.go` exposes the same canonical parameters in capability metadata.
- `cmd/browser-agent/tools_configure_persistence_actions_test.go` and `cmd/browser-agent/tools_configure_capabilities_test.go` cover store behavior and its advertised contract.
- `cmd/browser-agent/tools_configure_audit_test.go` and `cmd/browser-agent/tools_stdio_test.go` exercise the canonical store request through action and stdout-purity gates.
