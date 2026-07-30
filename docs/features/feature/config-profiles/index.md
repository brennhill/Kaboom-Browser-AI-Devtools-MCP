---
doc_type: feature_index
feature_id: feature-config-profiles
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-30
code_paths:
  - cmd/browser-agent/tools_core.go
  - cmd/browser-agent/tools_configure.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher.go
  - cmd/browser-agent/internal/toolcatalog/catalog.go
  - cmd/browser-agent/internal/toolconfigure/deps.go
  - cmd/browser-agent/internal/toolconfigure/session.go
  - cmd/browser-agent/internal/summarypref/cache.go
  - internal/statediag/collector.go
  - internal/session/snapshot-manager.go
  - internal/tools/configure/audit.go
  - internal/tools/configure/boundaries.go
  - internal/tools/configure/rewrite.go
  - internal/tools/configure/capabilities/capabilities.go
  - internal/tools/configure/capabilities/schema.go
  - internal/tools/configure/capabilities/modespecs.go
test_paths:
  - cmd/browser-agent/internal/toolconfigure/dispatcher_test.go
  - cmd/browser-agent/internal/toolcatalog/catalog_test.go
  - cmd/browser-agent/lint_hardening_test.go
  - cmd/browser-agent/internal/summarypref/cache_test.go
  - internal/statediag/collector_test.go
  - cmd/browser-agent/tools_configure_handler_test.go
  - cmd/browser-agent/tools_configure_persistence_actions_test.go
  - cmd/browser-agent/tools_configure_capabilities_test.go
  - cmd/browser-agent/tools_configure_jitter_test.go
  - cmd/browser-agent/tools_configure_noise_test.go
  - cmd/browser-agent/tools_interface_check_test.go
  - cmd/browser-agent/internal/toolconfigure/handlers_coverage_test.go
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

Configure mode registration and unknown-mode handling are owned by the
immutable `internal/toolconfigure.Dispatcher`. The composition boundary supplies
final handlers with explicit dependencies; `ToolHandler` exposes no configure
forwarding methods.
Capability examples resolve through the same `internal/toolcatalog.Catalog`
that owns executable modules and input schemas.

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
- Session summary-response preference caching and invalidation:
  - `cmd/browser-agent/internal/summarypref/cache.go`
  - composition passes `Cache.Invalidate` directly; no ToolHandler preference
    forwarding layer remains
- Shared configure argument normalization/parsing:
  - `internal/tools/configure/boundaries.go`
  - `internal/tools/configure/rewrite.go`
- The unused `internal/tools/configure` host declaration has been deleted;
  configure dependencies are defined only at boundaries that consume them.
- Configure-local noise, capability, security, telemetry, and jitter handlers
  receive one explicit function-field dependency value composed in
  `tools_configure.go`. Tutorial receives its own three-signal value. The
  former broad host interfaces and twelve configure-only ToolHandler adapters
  are deleted and structurally prohibited; analyze-owned shared boundaries are
  unchanged.
- Test-boundary start/end state and synchronization are owned together by
  `configure.BoundaryHandler`. The root `ToolHandler` router calls that owner
  directly and retains no boundary mutex, map, or forwarding methods.
- Tests:
  - `cmd/browser-agent/tools_configure_handler_test.go`
  - `cmd/browser-agent/tools_configure_persistence_actions_test.go`
  - `cmd/browser-agent/tools_configure_session_test.go`
  - `internal/tools/configure/boundaries_test.go`
  - `internal/tools/configure/rewrite_test.go`
