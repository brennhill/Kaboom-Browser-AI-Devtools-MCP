---
doc_type: feature_index
feature_id: feature-har-export
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-11-21
code_paths:
  - internal/tools/generate/handler/dispatcher.go
  - internal/tools/generate/handler/deps.go
  - internal/tools/generate/handler/artifacts_har_impl.go
  - internal/tools/mcp/response.go
  - internal/observation/export/har/export.go
  - internal/observation/export/har/conversion.go
  - internal/observation/export/har/filters.go
  - internal/observation/export/har/helpers.go
test_paths:
  - internal/tools/generate/handler/handlers_coverage_test.go
  - internal/observation/export/har/export_test.go
  - internal/observation/export/har/coverage_test.go
  - scripts/contracts/goarchitecturetests/contracts_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Har Export

## TL;DR

- Status: shipped
- Tool: generate
- Mode/Action: har
- Location: `docs/features/feature/har-export`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_HAR_EXPORT_001
- FEATURE_HAR_EXPORT_002
- FEATURE_HAR_EXPORT_003

## Code and Tests

- MCP adapter: `internal/tools/generate/handler/dispatcher.go`
- Composition: `internal/tools/generate/handler/deps.go` receives the
  canonical capture owner and version value directly, without ToolHandler getters.
- Generate handler: `internal/tools/generate/handler/artifacts_har_impl.go`
- Export implementation: `internal/observation/export/har/`
- HAR serialization, conversion, filtering, path handling, and their tests are
  one dependency-free package. It has no dependency on SARIF concerns and no
  root export facade.
