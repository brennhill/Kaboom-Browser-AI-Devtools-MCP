---
doc_type: feature_index
feature_id: feature-query-service
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-08-04
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
  - internal/capture/capture.go
  - internal/capture/sync.go
  - cmd/browser-agent/internal/asynccommand/handler.go
  - cmd/browser-agent/internal/asyncresult/normalization.go
  - src/types/global.d.ts
  - src/types/capture/
  - src/types/runtime/
  - src/types/wire/
  - src/types/utils.ts
  - src/types/runtime-messages.ts
  - src/types/runtime/queries.ts
  - src/background/pending-queries.ts
  - src/background/commands/helpers.ts
  - src/background/commands/interact.ts
  - src/background/exec/browser-actions.ts
  - src/background/exec/upload-handler.ts
  - src/background/orchestration/connection-monitor.ts
  - src/background/orchestration/stream-runtime.ts
  - scripts/contracts/check-architecture-boundaries.cjs
  - scripts/validate-architecture.sh
test_paths:
  - internal/mcp/response_test.go
  - internal/queries/dispatcher_test.go
  - internal/queries/commands_test.go
  - internal/queries/command_trace_test.go
  - internal/queries/expire_signal_test.go
  - internal/queries/result_ownership_test.go
  - internal/queries/no_facade_test.go
  - internal/capture/no_facade_test.go
  - internal/capture/sync_handler_owner_test.go
  - internal/capture/query_commands_test.go
  - cmd/browser-agent/internal/asyncresult/normalization_test.go
  - cmd/browser-agent/tools_async_formatting_test.go
  - cmd/browser-agent/tools_async_timeout_test.go
  - cmd/browser-agent/tools_core_sync_test.go
  - cmd/browser-agent/tools_pending_query_enqueue_test.go
  - tests/extension/contracts/no-compatibility-facades.test.js
  - scripts/contracts/check-architecture-boundaries.test.cjs
  - tests/extension/contracts/tooling-contracts.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Query Service

Command completion owns an injected wait operation. Lifecycle tests coordinate
entry and release with channels, allowing disconnect races to be reproduced
without sleeps while production remains wired to the query dispatcher's
notification-based wait.

Command waiters subscribe to the current notification generation and then
recheck lifecycle state before blocking. This prevents a completion or expiry
between the initial read and channel snapshot from becoming a missed wakeup.
The expiration/completion race suite runs from explicit barriers with no
wall-clock sleeps.

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

Pending commands expose their fixed capacity, active count, and oldest age to
health diagnostics. Saturation rejects new work explicitly; it never evicts an
accepted unresolved command to make room for disposable history.

- FEATURE_QUERY_SERVICE_001
- FEATURE_QUERY_SERVICE_002
- FEATURE_QUERY_SERVICE_003

## Code and Tests

- Query response assembly:
  - `internal/mcp/response.go` — marshal helpers plus the canonical `Succeed`/`SucceedText`/`Fail`/`ParseArgs` vocabulary
  - `internal/mcp/response_content.go` — image and warning content blocks
  - `internal/mcp/response_clamp.go` — JSON-aware payload clamping
- Command lifecycle updates accept only `pending`, `complete`, `error`, `timeout`, `expired`, or `cancelled`; noncanonical status text is treated as protocol drift and recorded as an error.
- Synchronous result consumers may bind waits to a caller context; cancellation
  and deadlines wake the canonical query condition through event-driven
  notifications instead of periodic polling, while preserving one-time result
  consumption.
- Raw JSON parameters and results are copied when they enter dispatcher-owned
  state and when command snapshots leave it. Callers and extension pollers
  cannot mutate queued commands or retained lifecycle history through shared
  slice backing arrays.
- Live command lifecycle storage keeps every pending command until it reaches a
  terminal state or expires. Completed and failed commands then share one
  five-entry terminal-history ring, preventing long browser sessions from
  accumulating stale dispatch records. Event recordings remain complete because
  `RecordingManager` owns and persists recorded actions independently of this
  diagnostic ring.
- Query IDs include a per-dispatcher process/time/sequence prefix, so a restarted
  daemon cannot reuse an ID that an attached extension still remembers for
  duplicate-delivery protection.
- Query and command state is accessed through the canonical
  `Capture.Queries()` owner. The former Capture forwarding layer and test-only
  pending-query facade have been deleted; disconnect-aware queue reconciliation
  is owned by `capture.SyncHandler` because it composes extension liveness with
  query expiry. `Capture` retains no sync forwarding methods.
- `internal/asynccommand.Handler` owns queue admission, accessibility queries,
  connectivity-aware waiting, terminal response enrichment, and outcome
  recording as one lifecycle. Callers receive its functions explicitly; the
  composition root exposes no parallel command-completion methods or provider
  interfaces.
- Tests:
  - `internal/mcp/response_test.go`
  - `internal/queries/no_facade_test.go` and `internal/capture/no_facade_test.go` prevent compatibility-only command lifecycle APIs from returning.
