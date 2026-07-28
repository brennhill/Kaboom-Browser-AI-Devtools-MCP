---
doc_type: feature_index
feature_id: feature-config-profiles
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - cmd/browser-agent/tools_configure.go
  - cmd/browser-agent/internal/toolconfigure/session.go
  - internal/session/snapshot-manager.go
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
- Session/store sub-handler and implementations:
  - `cmd/browser-agent/internal/toolconfigure/session.go`
- Shared configure argument normalization/parsing:
  - `internal/tools/configure/boundaries.go`
  - `internal/tools/configure/rewrite.go`
- Test-boundary start/end state and synchronization are owned together by
  `configure.BoundaryHandler`. The root `ToolHandler` router calls that owner
  directly and retains no boundary mutex, map, or forwarding methods.
- Tests:
  - `cmd/browser-agent/tools_configure_handler_test.go`
  - `cmd/browser-agent/tools_configure_session_test.go`
  - `internal/tools/configure/boundaries_test.go`
  - `internal/tools/configure/rewrite_test.go`
