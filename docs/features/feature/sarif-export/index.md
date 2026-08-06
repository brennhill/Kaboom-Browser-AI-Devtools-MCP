---
doc_type: feature_index
feature_id: feature-sarif-export
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-06
code_paths:
  - cmd/browser-agent/internal/toolgenerate/dispatcher.go
  - cmd/browser-agent/internal/toolgenerate/deps.go
  - cmd/browser-agent/internal/toolgenerate/artifacts_sarif_impl.go
  - internal/mcp/response.go
  - internal/export/sarif/export.go
  - internal/export/sarif/conversion.go
  - internal/export/sarif/file.go
  - internal/export/sarif/types.go
test_paths:
  - cmd/browser-agent/internal/toolgenerate/handlers_coverage_test.go
  - cmd/browser-agent/tools_generate_audit_test.go
  - internal/export/sarif/export_test.go
  - internal/export/sarif/document_test.go
  - internal/export/sarif/file_test.go
  - internal/export/sarif/unit_test.go
  - internal/export/sarif/coverage_test.go
  - scripts/contracts/goarchitecturetests/contracts_test.go
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
- Composition: `cmd/browser-agent/internal/toolgenerate/deps.go` explicitly
  supplies connectivity and accessibility operations; no catch-all host is retained.
- Generate handler: `cmd/browser-agent/internal/toolgenerate/artifacts_sarif_impl.go`
- Export implementation and tests: `internal/export/sarif/`
- SARIF owns its schema, conversion, safe file output, and golden evidence as a
  ten-file package. The generate handler supplies the Kaboom version explicitly;
  report generation no longer depends on mutable linker-injected package state.
