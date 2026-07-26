---
doc_type: feature_index
feature_id: feature-noise-filtering
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-26
code_paths:
  - cmd/browser-agent/internal/toolconfigure/noise_actions.go
  - cmd/browser-agent/noise_autorun.go
  - cmd/browser-agent/tools_configure_audit_log.go
  - cmd/browser-agent/tools_configure_sessions.go
  - internal/noise/noise.go
  - internal/noise/noise_builtin.go
  - internal/noise/noise_detect.go
  - internal/noise/noise_filter.go
  - internal/noise/noise_persistence.go
  - internal/noise/noise_rules.go
  - internal/noise/noise_stats.go
test_paths:
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

Add concrete implementation and test links here as this feature evolves.
