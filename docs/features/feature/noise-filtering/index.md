---
doc_type: feature_index
feature_id: feature-noise-filtering
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - cmd/browser-agent/internal/toolconfigure/noise_actions.go
  - internal/mcp/response.go
  - cmd/browser-agent/internal/noiseautorun/autorun.go
  - cmd/browser-agent/internal/toolconfigure/auditlog/handler.go
  - cmd/browser-agent/tools_configure.go
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
  - cmd/browser-agent/internal/noiseautorun/autorun_test.go
  - cmd/browser-agent/noise_first_connect_test.go
  - cmd/browser-agent/internal/toolconfigure/auditlog/handler_test.go
  - internal/noise/noise_test.go
  - internal/noise/noise_detect_test.go
  - internal/noise/noise_edge_test.go
  - internal/noise/noise_persistence_test.go
  - internal/noise/noise_validation_test.go
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
