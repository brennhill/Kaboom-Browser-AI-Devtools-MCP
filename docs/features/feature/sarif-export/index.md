---
doc_type: feature_index
feature_id: feature-sarif-export
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-11-21
code_paths:
  - internal/tools/generate/handler/dispatcher.go
  - internal/tools/generate/handler/deps.go
  - internal/tools/generate/handler/artifacts_sarif_impl.go
  - internal/tools/mcp/response.go
  - internal/analysis/export/sarif/export.go
  - internal/analysis/export/sarif/conversion.go
  - internal/analysis/export/sarif/file.go
  - internal/analysis/export/sarif/types.go
test_paths:
  - internal/tools/generate/handler/handlers_coverage_test.go
  - internal/analysis/export/sarif/export_test.go
  - internal/analysis/export/sarif/document_test.go
  - internal/analysis/export/sarif/file_test.go
  - internal/analysis/export/sarif/unit_test.go
  - internal/analysis/export/sarif/coverage_test.go
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

- MCP adapter: `internal/tools/generate/handler/dispatcher.go`
- Composition: `internal/tools/generate/handler/deps.go` explicitly
  supplies connectivity and accessibility operations; no catch-all host is retained.
- Generate handler: `internal/tools/generate/handler/artifacts_sarif_impl.go`
- Export implementation and tests: `internal/analysis/export/sarif/`
- SARIF owns its schema, conversion, safe file output, and golden evidence as a
  ten-file package. The generate handler supplies the Kaboom version explicitly;
  report generation no longer depends on mutable linker-injected package state.
