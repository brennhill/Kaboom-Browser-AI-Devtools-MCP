---
doc_type: feature_index
feature_id: feature-ttl-retention
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-04-22
code_paths:
  - cmd/browser-agent/screenshot_cleanup.go
test_paths:
  - cmd/browser-agent/screenshot_cleanup_test.go
last_verified_version: 0.8.5
last_verified_date: 2026-04-22
---

# Ttl Retention

## TL;DR

- Status: shipped
- Tool: configure
- Mode/Action: data TTL
- Location: `docs/features/feature/ttl-retention`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_TTL_RETENTION_001
- FEATURE_TTL_RETENTION_002
- FEATURE_TTL_RETENTION_003

## Code and Tests

- **Screenshot disk retention** (`cmd/browser-agent/screenshot_cleanup.go`) — background goroutine launched from `runMCPMode` that sweeps `state.ScreenshotsDir()` hourly and removes `.jpg`/`.jpeg`/`.png` files with ModTime older than 72 hours. Subdirectories and non-image files are left alone. Tests in `cmd/browser-agent/screenshot_cleanup_test.go` pin retention boundary, extension matching, and missing-directory no-op behavior.
