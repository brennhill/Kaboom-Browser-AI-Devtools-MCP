---
doc_type: feature_index
feature_id: feature-query-service
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-26
code_paths:
  - internal/mcp/response_json.go
  - internal/mcp/response_builders.go
  - internal/queries/dispatcher.go
  - internal/queries/dispatcher_commands.go
  - internal/queries/dispatcher_queries.go
  - internal/queries/dispatcher_results.go
  - internal/queries/dispatcher_trace.go
  - internal/queries/types.go
  - src/types/index.ts
  - src/types/messages.ts
  - src/types/runtime-messages.ts
  - src/types/runtime/queries.ts
test_paths:
  - internal/mcp/response_test.go
  - internal/queries/dispatcher_test.go
  - internal/queries/commands_test.go
  - internal/queries/command_trace_test.go
  - internal/queries/expire_signal_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Query Service

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/query-service`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_QUERY_SERVICE_001
- FEATURE_QUERY_SERVICE_002
- FEATURE_QUERY_SERVICE_003

## Code and Tests

- Query response assembly:
  - `internal/mcp/response_json.go`
  - `internal/mcp/response_builders.go`
- Tests:
  - `internal/mcp/response_test.go`
