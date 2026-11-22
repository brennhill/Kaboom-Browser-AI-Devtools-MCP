---
doc_type: feature_index
feature_id: feature-noise-filtering
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-11-21
code_paths:
  - internal/tools/commandhealth/doctor_live_checks.go
  - internal/tools/configure/handler/noise_actions.go
  - internal/tools/configure/handler/deps.go
  - internal/tools/mcp/response.go
  - internal/observation/noise/autorun/autorun.go
  - internal/tools/configure/handler/auditlog/handler.go
  - internal/composition/toolruntime/tools_configure.go
  - internal/tools/configure/handler/dispatcher.go
  - internal/composition/toolruntime/tools_core.go
  - internal/tools/configure/handler/session.go
  - internal/observation/noise/noise.go
  - internal/observation/noise/noise_builtin.go
  - internal/observation/noise/noise_detect.go
  - internal/observation/noise/noise_rules.go
  - internal/platform/util/media.go
test_paths:
  - internal/lifecycle/doctor/support/projections_test.go
  - internal/tools/configure/handler/dispatcher_test.go
  - scripts/contracts/goarchitecturetests/contracts_test.go
  - internal/tools/configure/handler/handlers_coverage_test.go
  - internal/tools/configure/handler/handlers_coverage_test.go
  - internal/observation/noise/autorun/autorun_test.go
  - internal/observation/noise/autorun/autorun_test.go
  - internal/tools/configure/handler/auditlog/handler_test.go
  - internal/observation/noise/noise_builtin_matching_test.go
  - internal/observation/noise/noise_rule_management_test.go
  - internal/observation/noise/noise_detect_test.go
  - internal/observation/noise/noise_edge_test.go
  - internal/observation/noise/noise_persistence_test.go
  - internal/observation/noise/noise_validation_test.go
  - tests/architecture/user-state-loaders.test.cjs
  - tests/architecture/user-state-loaders.json
  - scripts/tests/capture/cat-20-noise-persistence.sh
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Noise Filtering

## TL;DR

- Status: shipped
- Tool: configure
- Mode/Action: noise_rule, dismiss
- Location: `docs/features/feature/noise-filtering`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_NOISE_FILTERING_001
- FEATURE_NOISE_FILTERING_002
- FEATURE_NOISE_FILTERING_003

## Code and Tests

Noise persistence depends on its own minimal two-method store boundary rather
than the concrete session-store implementation. Canonical state-fault tests
prove read, write, quota, corruption, partial-write, cancellation, sync-stage,
and restart behavior; failures retain built-in or in-memory rules and create a
redacted `noise_rule_state` Doctor incident.

The package has two runtime ownership units: `noise.go` owns configuration and
hot-path matching, while `noise_rules.go` owns the complete mutable rule
lifecycle, including statistics and persistence. Keeping CRUD and its durable
side effects together prevents partial rule migrations and bounds the package
at ten files without compatibility wrappers.

Automatic detection scheduling, navigation debouncing, first-connect lifecycle
wiring, and telemetry adaptation are owned together by
`internal/observation/noise/autorun`. The root handler retains only its
required `mcp.NoiseFilterer` method and initialization wiring. Automatic URL
classification imports the canonical `internal/platform/util` path normalizer directly;
there is no capture-layer pass-through.
Runner time, dispatch, and delayed execution cross one private runtime boundary.
Tests drive callbacks and first-connect timers explicitly, while production uses
panic-safe goroutines and timers. A detector panic clears pending state through
a deferred lifecycle transition, so future navigation detection cannot remain
permanently wedged.
Composition coverage awaits the first-connect callback itself and emits repeated
connection events without spacing them on the wall clock. The former no-assertion
"emits log" test was deleted rather than retained as dormant coverage.
Configure noise handlers receive their owner callbacks through the explicit
configure dependency value; no noise-specific ToolHandler adapter surface
remains.

Persisted rule loading is fail-safe: malformed or unsupported saved data cannot
block startup, built-in defaults remain active, and both the HTTP and MCP System
Doctor surfaces report an actionable `noise_rule_state` warning. The diagnostic
does not include raw persisted content.
The architecture inventory names `noise_rules.go` as the canonical persisted
state reader, so moving persistence ownership cannot silently disable fallback
and Doctor enforcement.
