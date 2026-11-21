---
doc_type: feature_index
feature_id: feature-query-service
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-11-21
code_paths:
  - internal/mcp/response.go
  - internal/mcp/response_content.go
  - internal/mcp/response_clamp.go
  - internal/mcp/errors.go
  - internal/browsercommand/internal/dispatch/dispatcher.go
  - internal/browsercommand/internal/dispatch/dispatcher_commands.go
  - internal/browsercommand/internal/dispatch/dispatcher_queries.go
  - internal/browsercommand/internal/dispatch/dispatcher_results.go
  - internal/browsercommand/internal/dispatch/dispatcher_trace.go
  - internal/queries/types.go
  - internal/browsercommand/owner.go
  - internal/browsercommand/result.go
  - internal/browsercommand/exchange.go
  - internal/browsercommand/views.go
  - internal/browsercommand/commandtest/commandtest.go
  - internal/app/ports/ports.go
  - scripts/contracts/layering/main.go
  - scripts/contracts/layering/command_owner.go
  - cmd/browser-agent/internal/mediaapi/handler.go
  - cmd/browser-agent/internal/screenrec/save.go
  - cmd/browser-agent/internal/toolanalyze/annotationanalysis/handler.go
  - cmd/browser-agent/internal/toolconfigure/qafixture/transport/command.go
  - cmd/browser-agent/internal/toolobserve/dispatcher.go
  - cmd/browser-agent/internal/toolruntime/interact_wiring.go
  - internal/tools/observe/idbquery/execute.go
  - internal/capture/capture.go
  - internal/capture/syncruntime/handler.go
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
  - scripts/quality/verification/validate-architecture.sh
test_paths:
  - internal/mcp/response_test.go
  - internal/mcp/errors_test.go
  - internal/browsercommand/internal/dispatch/dispatcher_test.go
  - internal/browsercommand/internal/dispatch/commands_test.go
  - internal/browsercommand/internal/dispatch/command_trace_test.go
  - internal/browsercommand/internal/dispatch/expire_signal_test.go
  - internal/browsercommand/owner_test.go
  - internal/browsercommand/local_test.go
  - cmd/browser-agent/internal/toolconfigure/qafixture/transport/command_test.go
  - cmd/browser-agent/internal/toolinteract/interactbatch/batch_test.go
  - internal/capture/syncruntime/sync_command_lifecycle_test.go
  - internal/capture/syncruntime/sync_test.go
  - scripts/contracts/layering/main_test.go
  - cmd/browser-agent/composition_test.go
  - cmd/browser-agent/internal/screenrec/screenrec_test.go
  - internal/capture/syncruntime/extension_state_unit_test.go
  - internal/capture/async_queue_integration_test.go
  - internal/capture/no_facade_test.go
  - internal/capture/syncruntime/integrationtest/sync_handler_owner_test.go
  - internal/capture/query_commands_test.go
  - cmd/browser-agent/internal/asyncresult/normalization_test.go
  - cmd/browser-agent/internal/asynccommand/formatting_test.go
  - cmd/browser-agent/internal/asynccommand/handler_test.go
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
Tool-level synchronous completion tests launch the waiter and deliver results
through dispatcher state or the pending-query notification. They exercise both
race orderings without delaying the completion goroutine.
Analyze integration tests use the command owner's injected completion boundary
after real queue admission, eliminating scheduler-dependent observation of
short-lived pending entries while retaining the production queue path.

Command waiters subscribe to the current notification generation and then
recheck lifecycle state before blocking. This prevents a completion or expiry
between the initial read and channel snapshot from becoming a missed wakeup.
The expiration/completion race suite runs from explicit barriers with no
wall-clock sleeps.

Dispatcher cleanup has one owned stop channel and one completion channel.
`Close` is concurrent-safe and returns only after the periodic worker exits;
there is no nullable stop-function lifecycle or cross-package goroutine-count
test. Result timeout, cancellation, wakeup, TTL, and cleanup ownership are all
verified in this package.

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/protocol/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/protocol/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
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
- Browser-command lifecycle owner (`internal/browsercommand`, kaboom-jdm5.2):
  `Owner` wraps `QueryDispatcher` state and is the only path the transports use
  for queueing (`Enqueue`), long-poll delivery (`AwaitPending`), results
  (`SubmitResult` / `ApplySyncExchange`), expiry (`ExpireAll`), MCP disconnect
  (`FailDisconnected`) and missing-progress reconciliation. `/sync`
  (`syncruntime.Handler`), `/query-result` (`httpingest.HandleQueryResult`) and
  MCP waits (`asynccommand.Handler`) are adapters over it. Each `ResultSource`
  has one validation policy: `sync` results are accepted only inside
  `ApplySyncExchange` while the connection owner's `GenerationGate` confirms the
  exchange generation is current (results, ack, disconnect expiry and progress
  reconciliation all run inside the gate, so superseded polls cannot mutate
  current-generation commands); `query_result` is validated by identity and
  client scope (its wire contract carries no generation); `daemon` transitions
  need a correlation ID and are either a failure (`error` with an error) or the
  completion of a daemon-local command registered with `RegisterLocal` (for
  example annotation waits). Connection identity, liveness and generation
  stay with `syncruntime.Runtime`. `scripts/contracts/layering` forbids the
  command module (`internal/browsercommand`, `internal/queries`) from importing
  `net/http`, `internal/mcp` or `internal/capture/**`.
- Owner-only dispatcher access (kaboom-jdm5.5): every production caller —
  interact (`toolinteract.Commands`, batch `WaitCommand`), observe
  (`observecore.Deps.Commands`, storage/IndexedDB/waterfall), analyze annotation
  waits (`annotationanalysis.Commands`), media/recording uploads
  (`ports.ResultSink` in `internal/app/ports`, as `query_result`), QA fixtures,
  health/doctor/operational traces (`Snapshot`, `PendingCommands`,
  `CompletedCommands`, `FailedCommands`, `RecentTraces`) — goes through
  `browsercommand.Owner` or a consumer interface it satisfies. `queries.QueryRunner`
  is the shared "enqueue, then wait" port. The layering `command-owner` rule
  (`scripts/contracts/layering/command_owner.go`) fails any production
  `X.Queries()` call and any import of the dispatcher outside
  `internal/browsercommand/**`. The dispatcher itself lives in
  `internal/browsercommand/internal/dispatch`, so the Go compiler keeps every
  other package off it; `browsercommand.NewOwner()` constructs it and
  `Owner.Close()` stops its cleanup. `Capture.Queries()` was deleted. Tests drive
  the extension side through `internal/browsercommand/commandtest`, which uses only
  the owner's public result-source API (`query_result` submissions and a gated
  `ApplySyncExchange` acknowledgment). `ports.ResultSink` lives in
  `internal/app/ports` because the ownership rule rejects a producer-owned
  interface in `browsercommand` and `queries` cannot import `browsercommand`.
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
- Queue capacity and concurrent admission are tested inside `internal/browsercommand/internal/dispatch`
  with an explicit start barrier and exact accepted/rejected counts. The former
  Capture-level polling/jitter simulation and duplicate wall-clock expiration
  loop were deleted; Capture retains only end-to-end composition coverage.
- Query and command state is accessed through the canonical
  `Capture.Commands()` owner (`browsercommand.Owner`). The former Capture forwarding layer and test-only
  pending-query facade have been deleted; disconnect-aware queue reconciliation
  is owned by `capture.SyncHandler` because it composes extension liveness with
  query expiry. `Capture` retains no sync forwarding methods.
- Test callers also consume `GetPendingQueries` directly. The former exported
  `GetLastPendingQuery` test accessor was a production compatibility surface;
  every caller now derives the latest detached snapshot in test code, and the
  query architecture gate prevents that facade from returning.
- `internal/asynccommand.Handler` owns queue admission, accessibility queries,
  connectivity-aware waiting, terminal response enrichment, and outcome
  recording as one lifecycle. Callers receive its functions explicitly; the
  composition root exposes no parallel command-completion methods or provider
  interfaces.
- Tests:
  - `internal/mcp/response_test.go`
  - `internal/browsercommand/internal/dispatch/dispatcher_test.go` and `internal/capture/no_facade_test.go` prevent compatibility-only command lifecycle APIs from returning.
