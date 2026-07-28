---
doc_type: feature_index
feature_id: feature-query-service
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - internal/mcp/response.go
  - internal/mcp/response_content.go
  - internal/mcp/response_clamp.go
  - internal/queries/dispatcher.go
  - internal/queries/dispatcher_commands.go
  - internal/queries/dispatcher_queries.go
  - internal/queries/dispatcher_results.go
  - internal/queries/dispatcher_trace.go
  - internal/queries/types.go
  - internal/capture/query_dispatcher.go
  - cmd/browser-agent/tools_async_completion.go
  - cmd/browser-agent/internal/asyncresult/normalization.go
  - src/types/index.ts
  - src/types/global.d.ts
  - src/types/runtime-messages.ts
  - src/types/runtime/queries.ts
  - src/background/pending-queries.ts
  - src/background/commands/helpers.ts
  - src/background/commands/interact.ts
  - src/background/exec/browser-actions.ts
  - src/background/exec/upload-handler.ts
  - src/background/index.ts
test_paths:
  - internal/mcp/response_test.go
  - internal/queries/dispatcher_test.go
  - internal/queries/commands_test.go
  - internal/queries/command_trace_test.go
  - internal/queries/expire_signal_test.go
  - internal/queries/no_facade_test.go
  - internal/capture/no_facade_test.go
  - internal/capture/query_commands_test.go
  - cmd/browser-agent/internal/asyncresult/normalization_test.go
  - tests/extension/no-compatibility-facades.test.js
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
  - `internal/mcp/response.go` — marshal helpers plus the canonical `Succeed`/`SucceedText`/`Fail`/`ParseArgs` vocabulary
  - `internal/mcp/response_content.go` — image and warning content blocks
  - `internal/mcp/response_clamp.go` — JSON-aware payload clamping
- Tests:
  - `internal/mcp/response_test.go`
  - `internal/queries/no_facade_test.go` and `internal/capture/no_facade_test.go` prevent compatibility-only command lifecycle APIs from returning.
