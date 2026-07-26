---
doc_type: feature_index
feature_id: feature-api-schema
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-26
code_paths:
  - cmd/browser-agent/openapi.go
  - internal/analysis/api_contract.go
  - internal/analysis/api_contract_analysis.go
  - internal/analysis/api_contract_endpoint.go
  - internal/analysis/api_contract_learning.go
  - internal/analysis/api_contract_validation.go
  - internal/analysis/api_contract_violations.go
  - internal/analysis/apischema/schema.go
  - internal/analysis/apischema/builder.go
  - internal/analysis/apischema/openapi.go
  - internal/analysis/apischema/infer.go
  - internal/analysis/apischema/path.go
  - internal/analysis/apischema/observe_http.go
  - internal/analysis/apischema/observe_ws.go
  - internal/schema/observe.go
test_paths:
  - internal/analysis/api_contract_test.go
  - internal/analysis/branch_coverage_test.go
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
