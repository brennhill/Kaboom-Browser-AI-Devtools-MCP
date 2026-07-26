---
doc_type: feature_index
feature_id: feature-api-schema
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-26
code_paths:
  - cmd/browser-agent/openapi.go
  - internal/analysis/apicontract/contract.go
  - internal/analysis/apicontract/report.go
  - internal/analysis/apicontract/endpoint.go
  - internal/analysis/apicontract/learning.go
  - internal/analysis/apicontract/validation.go
  - internal/analysis/apicontract/violations.go
  - internal/analysis/apischema/schema.go
  - internal/analysis/apischema/builder.go
  - internal/analysis/apischema/openapi.go
  - internal/analysis/apischema/infer.go
  - internal/analysis/apischema/path.go
  - internal/analysis/apischema/observe_http.go
  - internal/analysis/apischema/observe_ws.go
  - internal/schema/observe.go
test_paths:
  - internal/analysis/apicontract/contract_test.go
  - internal/analysis/apicontract/branch_coverage_test.go
  - internal/analysis/apischema/builder_test.go
  - internal/analysis/apischema/infer_test.go
  - internal/analysis/apischema/observe_http_test.go
  - internal/analysis/apischema/openapi_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Api Schema

## TL;DR

- Status: shipped
- Tool: observe
- Mode/Action: api
- Location: `docs/features/feature/api-schema`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_API_SCHEMA_001
- FEATURE_API_SCHEMA_002
- FEATURE_API_SCHEMA_003

## Code and Tests

Add concrete implementation and test links here as this feature evolves.
