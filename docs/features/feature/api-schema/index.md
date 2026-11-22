---
doc_type: feature_index
feature_id: feature-api-schema
status: superseded
feature_type: feature
owners: []
last_reviewed: 2026-09-05
code_paths:
  - internal/tools/configure/handler/capabilities.go
  - internal/tools/configure/handler/deps.go
  - internal/composition/toolruntime/tools_configure.go
  - internal/tools/configure/handler/dispatcher.go
  - internal/tools/catalog/catalog.go
  - internal/composition/toolruntime/tools_core.go
  - cmd/browser-agent/server.go
  - internal/analysis/apicontract/contract.go
  - internal/analysis/apicontract/runtime/handler.go
  - internal/analysis/apicontract/report.go
  - internal/analysis/apicontract/endpoint.go
  - internal/analysis/apicontract/learning.go
  - internal/analysis/apicontract/validation.go
  - internal/analysis/apicontract/violations.go
  - internal/tools/schema/schema.go
  - internal/tools/schema/observe.go
  - internal/tools/schema/analyze.go
  - internal/tools/schema/generate.go
  - internal/tools/schema/configure/tool.go
  - internal/tools/schema/configure/properties.go
  - internal/tools/schema/configure/properties_core.go
  - internal/tools/schema/configure/properties_runtime.go
  - internal/tools/schema/interact/tool.go
  - internal/tools/schema/interact/actions.go
  - internal/tools/schema/interact/properties.go
  - internal/tools/schema/interact/properties_core.go
  - internal/tools/schema/interact/properties_dispatch.go
  - internal/tools/schema/interact/properties_form_wait.go
  - internal/tools/schema/interact/properties_output_batch.go
  - internal/tools/schema/interact/properties_targeting.go
  - internal/tools/mcp/response_clamp.go
  - scripts/contracts/response/snapshot-tool-responses.cjs
test_paths:
  - internal/tools/configure/handler/dispatcher_test.go
  - internal/tools/catalog/catalog_test.go
  - scripts/contracts/goarchitecturetests/contracts_test.go
  - internal/tools/configure/handler/handlers_coverage_test.go
  - internal/analysis/apicontract/runtime/handler_test.go
  - internal/analysis/apicontract/contract_test.go
  - internal/analysis/apicontract/contract_actions_test.go
  - internal/analysis/apicontract/contract_analyze_test.go
  - internal/analysis/apicontract/contract_report_test.go
  - internal/tools/schema/invariants_test.go
  - internal/tools/schema/interact/schema_test.go
  - cmd/browser-agent/composition_test.go
  - internal/tools/mcp/response_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **Removed 2026-07-26 — dead code, never reachable.**
>
> API schema inference was never wired to an MCP surface. The package had zero importers outside its own tests and was absent from `go list -deps ./cmd/browser-agent/...`, so no schema was ever inferred at runtime. Removed rather than left as `status: shipped`, which it never was.
>
> *Evidence correction, same day:* this note originally cited `InferSchema`/`RecordRequest`
> as "existing nowhere else." Those symbols never existed under those names — the real
> entry points were `NewSchemaStore`/`Observe`/`BuildSchema`, and a grep returning zero for
> a name that was never used proves nothing. The unreachability conclusion was verified
> independently at symbol level and stands; the cited evidence did not.

Note: the API *contract* checking in `internal/analysis/apicontract` is live and unaffected; only the schema-inference half was dead.

The live contract engine is independent of MCP state. Its incremental offset,
clear lifecycle, and response projection are owned by the one-way
`internal/analysis/apicontract/runtime` adapter; callers import that canonical
owner directly, with no compatibility facade. Core shape/observation edge cases
live beside the core contract tests, and both packages enforce the ten-file
ownership boundary.


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

API contract tests construct canonical `internal/platform/types.NetworkBody` values
directly; no test-local type alias masks the wire-contract owner.
API contract violations expose one discriminator, `violation_type`; the former
duplicate `type` response field is not emitted.
First-observation retention tests compare the exact captured timestamp across
subsequent observations; they do not sleep merely to make wall-clock values
different.

Observed-traffic API schema inference (`observe {what: "api"}`):

- Schema inference and OpenAPI emission: `internal/analysis/apischema/`
- Contract learning and violation reporting: `internal/analysis/apicontract/`
- MCP tool-list adapter: `cmd/browser-agent/tools_core.go`
- OpenAPI HTTP route: `cmd/browser-agent/server.go`

MCP tool schemas (the `tools/list` contract, `internal/tools/schema`):

- tools/list assembly: `internal/tools/schema/schema.go` (`AllTools`, in tools/list order)
- Single-file tool schemas: `internal/tools/schema/observe.go`, `internal/tools/schema/analyze.go`, `internal/tools/schema/generate.go`
- Configure tool schema + property groups: `internal/tools/schema/configure/`
- Interact tool schema, property groups, and canonical action registry: `internal/tools/schema/interact/`
  (`interact.ActionSpecs` is the single source of truth consumed by `internal/tools/configure`
  for `describe_capabilities` mode specs)
- Daemon delegation: `cmd/browser-agent/tools_core.go`
- Claude-API schema invariants (no top-level/nested combiners, valid JSON round-trip): `internal/tools/schema/invariants_test.go`
- Interact enum/alias/registry invariants: `internal/tools/schema/interact/schema_test.go`
- Schema/runtime dispatch parity: `cmd/browser-agent/composition_test.go`
