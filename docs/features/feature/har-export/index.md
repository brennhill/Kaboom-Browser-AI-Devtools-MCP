---
doc_type: feature_index
feature_id: feature-har-export
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - cmd/browser-agent/internal/toolgenerate/dispatcher.go
  - cmd/browser-agent/internal/toolgenerate/deps.go
  - cmd/browser-agent/internal/toolgenerate/artifacts_har_impl.go
  - internal/mcp/response.go
  - internal/export/export_har.go
  - internal/export/export_sarif.go
test_paths:
  - cmd/browser-agent/internal/toolgenerate/handlers_coverage_test.go
  - cmd/browser-agent/tools_generate_har_test.go
  - cmd/browser-agent/lint_hardening_test.go
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

- MCP adapter: `cmd/browser-agent/internal/toolgenerate/dispatcher.go`
- Composition: `cmd/browser-agent/internal/toolgenerate/deps.go` receives the
  canonical capture owner and version value directly, without ToolHandler getters.
- Generate handler: `cmd/browser-agent/internal/toolgenerate/artifacts_har_impl.go`
- Export implementation: `internal/export/export_har.go`
