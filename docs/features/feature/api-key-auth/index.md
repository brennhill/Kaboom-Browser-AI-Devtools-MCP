---
doc_type: feature_index
feature_id: feature-api-key-auth
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-06
code_paths:
  - cmd/browser-agent/internal/httpguard/apikey.go
  - cmd/browser-agent/internal/httpguard/middleware.go
test_paths:
  - cmd/browser-agent/internal/httpguard/apikey_test.go
  - cmd/browser-agent/internal/httpguard/middleware_test.go
  - cmd/browser-agent/http_helpers_unit_test.go
last_verified_version: 0.8.1
last_verified_date: 2026-03-29
---

# Api Key Auth

## TL;DR

- Status: shipped
- Tool: configure
- Mode/Action: request validation
- Location: `docs/features/feature/api-key-auth`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_API_KEY_AUTH_001
- FEATURE_API_KEY_AUTH_002
- FEATURE_API_KEY_AUTH_003

## Code and Tests

- Auth middleware: `cmd/browser-agent/internal/httpguard/apikey.go`
- CORS allowlist for auth header: `cmd/browser-agent/internal/httpguard/middleware.go`
- Tests: `cmd/browser-agent/internal/httpguard/apikey_test.go`, `cmd/browser-agent/http_helpers_unit_test.go`
