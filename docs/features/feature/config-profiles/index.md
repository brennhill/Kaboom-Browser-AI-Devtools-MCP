---
doc_type: feature_index
feature_id: feature-config-profiles
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-26
code_paths:
  - cmd/browser-agent/tools_configure.go
  - cmd/browser-agent/tools_configure_registry.go
  - cmd/browser-agent/tools_configure_session_handler.go
  - cmd/browser-agent/tools_configure_state_impl.go
  - cmd/browser-agent/tools_configure_sessions.go
  - internal/tools/configure/audit.go
  - internal/tools/configure/boundaries.go
  - internal/tools/configure/rewrite.go
  - internal/tools/configure/capabilities/capabilities.go
  - internal/tools/configure/capabilities/schema.go
  - internal/tools/configure/capabilities/modespecs.go
test_paths:
  - cmd/browser-agent/tools_configure_handler_test.go
  - cmd/browser-agent/tools_configure_session_test.go
  - internal/tools/configure/audit_test.go
  - internal/tools/configure/boundaries_test.go
  - internal/tools/configure/rewrite_test.go
  - internal/tools/configure/capabilities/capabilities_test.go
  - internal/tools/configure/capabilities/modespecs_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Config Profiles

## TL;DR

- Status: proposed
- Tool: configure
- Mode/Action: profiles
- Location: `docs/features/feature/config-profiles`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_CONFIG_PROFILES_001
- FEATURE_CONFIG_PROFILES_002
- FEATURE_CONFIG_PROFILES_003

## Code and Tests

- Configure dispatch and action registry:
  - `cmd/browser-agent/tools_configure.go`
  - `cmd/browser-agent/tools_configure_registry.go`
- Session/store sub-handler and implementations:
  - `cmd/browser-agent/tools_configure_session_handler.go`
  - `cmd/browser-agent/tools_configure_state_impl.go`
  - `cmd/browser-agent/tools_configure_sessions.go`
- Shared configure argument normalization/parsing:
  - `internal/tools/configure/boundaries.go`
  - `internal/tools/configure/rewrite.go`
- Tests:
  - `cmd/browser-agent/tools_configure_handler_test.go`
  - `cmd/browser-agent/tools_configure_session_test.go`
  - `internal/tools/configure/boundaries_test.go`
  - `internal/tools/configure/rewrite_test.go`
