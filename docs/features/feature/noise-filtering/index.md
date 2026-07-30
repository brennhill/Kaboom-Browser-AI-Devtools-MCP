---
doc_type: feature_index
feature_id: feature-noise-filtering
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-30
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
  - internal/noise/noise_filter.go
  - internal/noise/noise_persistence.go
  - internal/noise/noise_rules.go
  - internal/noise/noise_stats.go
  - internal/util/url.go
test_paths:
  - cmd/browser-agent/noise_doctor_test.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher_test.go
  - cmd/browser-agent/lint_hardening_test.go
  - cmd/browser-agent/internal/toolconfigure/handlers_coverage_test.go
  - cmd/browser-agent/tools_configure_noise_test.go
  - cmd/browser-agent/internal/noiseautorun/autorun_test.go
  - cmd/browser-agent/noise_first_connect_test.go
  - cmd/browser-agent/internal/toolconfigure/auditlog/handler_test.go
  - internal/noise/noise_builtin_matching_test.go
  - internal/noise/noise_rule_management_test.go
  - internal/noise/noise_detect_test.go
  - internal/noise/noise_edge_test.go
  - internal/noise/noise_persistence_test.go
  - internal/noise/noise_validation_test.go
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

Automatic detection scheduling, navigation debouncing, first-connect lifecycle
wiring, and telemetry adaptation are owned together by
`cmd/browser-agent/internal/noiseautorun`. The root handler retains only its
required `mcp.NoiseFilterer` method and initialization wiring. Automatic URL
classification imports the canonical `internal/util` path normalizer directly;
there is no capture-layer pass-through.
Configure noise handlers receive their owner callbacks through the explicit
configure dependency value; no noise-specific ToolHandler adapter surface
remains.

Persisted rule loading is fail-safe: malformed or unsupported saved data cannot
block startup, built-in defaults remain active, and both the HTTP and MCP System
Doctor surfaces report an actionable `noise_rule_state` warning. The diagnostic
does not include raw persisted content.
