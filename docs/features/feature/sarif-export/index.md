---
doc_type: feature_index
feature_id: feature-sarif-export
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - cmd/browser-agent/internal/toolgenerate/dispatcher.go
  - cmd/browser-agent/internal/toolgenerate/artifacts_sarif_impl.go
  - internal/mcp/response.go
  - internal/export/export_sarif.go
  - internal/export/export_sarif_file.go
test_paths:
  - cmd/browser-agent/internal/toolgenerate/handlers_coverage_test.go
  - cmd/browser-agent/tools_generate_audit_test.go
  - internal/export/export_sarif_test.go
  - internal/export/export_sarif_unit_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Sarif Export

## TL;DR

- Status: shipped
- Tool: generate
- Mode/Action: sarif
- Location: `docs/features/feature/sarif-export`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_SARIF_EXPORT_001
- FEATURE_SARIF_EXPORT_002
- FEATURE_SARIF_EXPORT_003

## Code and Tests

- MCP adapter: `cmd/browser-agent/internal/toolgenerate/dispatcher.go`
- Generate handler: `cmd/browser-agent/internal/toolgenerate/artifacts_sarif_impl.go`
- Export implementation and tests: `internal/export/`
