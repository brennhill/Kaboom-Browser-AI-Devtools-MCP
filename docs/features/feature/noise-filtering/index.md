---
doc_type: feature_index
feature_id: feature-noise-filtering
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-07
code_paths:
  - cmd/browser-agent/internal/health/doctor_live_checks.go
  - cmd/browser-agent/internal/toolconfigure/noise_actions.go
  - cmd/browser-agent/internal/toolconfigure/deps.go
  - internal/mcp/response.go
  - cmd/browser-agent/internal/noiseautorun/autorun.go
  - cmd/browser-agent/internal/toolconfigure/auditlog/handler.go
  - cmd/browser-agent/tools_configure.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher.go
  - cmd/browser-agent/tools_core.go
  - cmd/browser-agent/internal/toolconfigure/session.go
  - internal/noise/noise.go
  - internal/noise/noise_builtin.go
  - internal/noise/noise_detect.go
  - internal/noise/noise_rules.go
  - internal/util/media.go
test_paths:
  - cmd/browser-agent/noise_doctor_test.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher_test.go
  - scripts/contracts/goarchitecturetests/contracts_test.go
  - cmd/browser-agent/internal/toolconfigure/handlers_coverage_test.go
  - cmd/browser-agent/internal/toolconfigure/handlers_coverage_test.go
  - cmd/browser-agent/internal/noiseautorun/autorun_test.go
  - cmd/browser-agent/noise_first_connect_test.go
  - cmd/browser-agent/internal/toolconfigure/auditlog/handler_test.go
  - internal/noise/noise_builtin_matching_test.go
  - internal/noise/noise_rule_management_test.go
  - internal/noise/noise_detect_test.go
  - internal/noise/noise_edge_test.go
  - internal/noise/noise_persistence_test.go
  - internal/noise/noise_validation_test.go
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
`cmd/browser-agent/internal/noiseautorun`. The root handler retains only its
required `mcp.NoiseFilterer` method and initialization wiring. Automatic URL
classification imports the canonical `internal/util` path normalizer directly;
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
