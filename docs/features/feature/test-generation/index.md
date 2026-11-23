---
doc_type: feature_index
feature_id: feature-test-generation
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-11-21
code_paths:
  - internal/tools/generate/testgen/handler.go
  - internal/tools/generate/testgen/provider_adapter.go
  - internal/tools/generate/testgen/classify.go
  - internal/tools/generate/testgen/heal.go
  - internal/tools/generate/testgen/generate.go
  - internal/tools/generate/handler/dispatcher.go
  - internal/tools/generate/handler/deps.go
  - internal/tools/generate/handler/artifacts_test_impl.go
  - internal/tools/mcp/response.go
  - internal/testgen/generate.go
  - internal/recording/reproduction/reproduction_locators.go
  - internal/observation/store/bodies/store.go
  - internal/observation/store/actions/store.go
  - internal/platform/types/wire_log.go
  - internal/testgen/helpers.go
  - internal/testgen/classify.go
  - internal/testgen/types.go
  - internal/testgen/heal/batch.go
  - internal/testgen/heal/paths.go
  - internal/testgen/heal/repair.go
  - internal/testgen/heal/selectors.go
  - internal/testgen/heal/summary.go
  - internal/testgen/heal/types.go
  - internal/tools/schema/generate.go
test_paths:
  - internal/tools/generate/csp_golden_test.go
  - internal/tools/generate/testdata/csp-moderate.golden.json
  - internal/tools/generate/handler/handlers_coverage_test.go
  - scripts/contracts/goarchitecturetests/contracts_test.go
  - internal/tools/generate/testgen/entrypoints_test.go
  - internal/tools/generate/testgen/context_test.go
  - internal/tools/generate/testgen/heal_test.go
  - internal/tools/generate/testgen/classify_test.go
  - internal/tools/generate/testgen/testhelpers_test.go
  - internal/testgen/generate_test.go
  - internal/observation/store/bodies/store_test.go
  - internal/observation/store/actions/store_test.go
  - internal/testgen/helpers_test.go
  - internal/testgen/classify_test.go
  - internal/testgen/heal/heal_test.go
  - internal/tools/schema/invariants_test.go
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
- Operator guides: [quick start](./guides/quick-start.md), [UAT](./guides/uat-guide.md), and [validation](./guides/validation-guide.md)
- Historical planning and reviews: [status](./history/STATUS.md) and [review](./history/REVIEW.md)

## Requirement IDs

- FEATURE_TEST_GENERATION_001
- FEATURE_TEST_GENERATION_002
- FEATURE_TEST_GENERATION_003

## Code and Tests

- Sub-handler wiring: `internal/tools/generate/testgen/handler.go`
- Tool composition and tests use the owned `testGenHandler` directly; the
  unchanged-return ToolHandler accessor has been deleted.
- Generate handlers receive an explicit immutable dependency bundle composed
  from the capture, annotation, version, connectivity, and accessibility owners;
  `ToolHandler` is not a generate dependency interface.
- The context-test sub-handler likewise receives only log, action, and network
  snapshot functions. It has no capture-provider or log-reader host interface.
- Context dispatch: `internal/tools/generate/testgen/generate.go`
- Canonical contracts: `internal/testgen/types.go`, `internal/testgen/heal/types.go`
- Provider delegation: `internal/tools/generate/testgen/provider_adapter.go`
- Heal and classify handlers: `internal/tools/generate/testgen/heal.go`, `internal/tools/generate/testgen/classify.go`
- Test generation and failure classification engine: `internal/testgen/`
- Selector healing engine (self-contained subpackage): `internal/testgen/heal/`
- Generate tool schema contract: `internal/tools/schema/generate.go`
- Core behavior tests: `internal/tools/generate/testgen/context_test.go`, `internal/tools/generate/testgen/heal_test.go`, `internal/tools/generate/testgen/classify_test.go`
- Context generation and its script-output helpers deliberately share
  `context_test.go`: they consume the same generated-test contract and fixtures,
  and the package has an executable ten-file ownership boundary.
- Schema invariants: `internal/tools/schema/invariants_test.go`

## Three-locator emission

`generate({type:'test_from_context'})` emits through `internal/recording/reproduction`, so every generated step
now carries the accessibility and coordinate locators alongside its selector, and states the
environment the recording session pinned. See [Session to test](../session-to-test/index.md).
