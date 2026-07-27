---
doc_type: feature_index
feature_id: feature-test-generation
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - cmd/browser-agent/internal/testgenhandler/handler.go
  - cmd/browser-agent/internal/testgenhandler/provider_adapter.go
  - cmd/browser-agent/internal/testgenhandler/classify.go
  - cmd/browser-agent/internal/testgenhandler/heal.go
  - cmd/browser-agent/internal/testgenhandler/generate.go
  - cmd/browser-agent/internal/toolgenerate/dispatcher.go
  - cmd/browser-agent/internal/toolgenerate/artifacts_test_impl.go
  - internal/mcp/response.go
  - internal/testgen/generate.go
  - internal/testgen/helpers.go
  - internal/testgen/classify.go
  - internal/testgen/types.go
  - internal/testgen/heal/batch.go
  - internal/testgen/heal/paths.go
  - internal/testgen/heal/repair.go
  - internal/testgen/heal/selectors.go
  - internal/testgen/heal/summary.go
  - internal/testgen/heal/types.go
  - internal/schema/generate.go
test_paths:
  - cmd/browser-agent/internal/toolgenerate/handlers_coverage_test.go
  - cmd/browser-agent/tools_generate_handler_test.go
  - cmd/browser-agent/tools_generate_validation_test.go
  - cmd/browser-agent/internal/testgenhandler/context_test.go
  - cmd/browser-agent/internal/testgenhandler/generate_test.go
  - cmd/browser-agent/internal/testgenhandler/heal_test.go
  - cmd/browser-agent/internal/testgenhandler/classify_test.go
  - internal/testgen/generate_test.go
  - internal/testgen/helpers_test.go
  - internal/testgen/classify_test.go
  - internal/testgen/heal/heal_test.go
  - internal/schema/invariants_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Test Generation

## TL;DR

- Status: proposed
- Tool: generate
- Mode/Action: [test_from_context, test_heal, test_classify]
- Location: `docs/features/feature/test-generation`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_TEST_GENERATION_001
- FEATURE_TEST_GENERATION_002
- FEATURE_TEST_GENERATION_003

## Code and Tests

- Sub-handler wiring: `cmd/browser-agent/internal/testgenhandler/handler.go`
- Context dispatch: `cmd/browser-agent/internal/testgenhandler/generate.go`
- Canonical contracts: `internal/testgen/types.go`, `internal/testgen/heal/types.go`
- Provider delegation: `cmd/browser-agent/internal/testgenhandler/provider_adapter.go`
- Heal and classify handlers: `cmd/browser-agent/internal/testgenhandler/heal.go`, `cmd/browser-agent/internal/testgenhandler/classify.go`
- Test generation and failure classification engine: `internal/testgen/`
- Selector healing engine (self-contained subpackage): `internal/testgen/heal/`
- Generate tool schema contract: `internal/schema/generate.go`
- Core behavior tests: `cmd/browser-agent/internal/testgenhandler/context_test.go`, `cmd/browser-agent/internal/testgenhandler/generate_test.go`, `cmd/browser-agent/internal/testgenhandler/heal_test.go`, `cmd/browser-agent/internal/testgenhandler/classify_test.go`
- Schema invariants: `internal/schema/invariants_test.go`
