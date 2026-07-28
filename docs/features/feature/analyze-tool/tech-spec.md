---
feature: analyze-tool
status: shipped
version: 0.7.12
doc_type: tech-spec
feature_id: feature-analyze-tool
last_reviewed: 2026-07-27
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Analyze Tech Spec

## Dispatcher and Handler Topology
- Dispatch entrypoint: `(*analyzedispatch.Dispatcher).Handle` in `cmd/browser-agent/internal/toolanalyze/analyzedispatch/dispatcher.go`.
- Mode routing table: the private analyze mode registry in `cmd/browser-agent/internal/toolanalyze/analyzedispatch/dispatcher.go`.
- Annotation handlers: `cmd/browser-agent/internal/toolanalyze/annotationanalysis/handler.go`.
- API validation runtime and handlers: `internal/analysis/apicontract/runtime_handler.go`.
- Async command-result polling: `cmd/browser-agent/internal/toolobserve/dispatcher.go`.

## Query-Type Mapping
- `dom` -> pending query type `dom`
- `accessibility` -> pending query type `a11y`
- `page_summary` -> pending query type `execute`
- `link_health` -> pending query type `link_health`

## Server-Only Flows
- `link_validation` performs server-side URL checks (no extension dependency).
- `api_validation` performs contract analysis over captured network bodies.
- `security_audit` and `third_party_audit` consume capture buffers and policy filters.

## Async and Sync-by-Default
- Async-capable handlers register a command correlation ID in the command tracker.
- `MaybeWaitForCommand` provides sync-by-default behavior and falls back to `still_processing`.
- `observe({what:"command_result", correlation_id})` is the canonical retrieval path.

## Annotation Waiter and Flush Recovery
- `analyze({what:"annotations", background:false})` now uses a two-stage wait path:
  1. bounded blocking wait for new annotations (`timeout_ms`, default 15s, max 10m),
  2. fallback to `ann_*` async waiter + command-result polling if the block window expires.
- Normal completion path:
  1. Draw-mode completion stores session data in `internal/annotation`.
  2. Store callback completes matching waiters via `capture.ApplyCommandResult(correlationID, "complete", payload, "")`.
  3. `observe(command_result)` returns final command output.
- Recovery path (`#412`):
  - `analyze({what:"annotations", operation:"flush", correlation_id:"ann_*"})`
  - Removes the pending waiter (`TakeWaiter`) to prevent duplicate later completion.
  - Force-completes pending command quickly with currently available annotations.
  - Emits explicit `terminal_reason` values:
    - `completed` (normal storage callback path)
    - `flushed` (operator recovery flush with available data)
    - `abandoned` (flush with no captured annotation data)
- Repeated flush calls are idempotent: terminal commands are returned as-is.

## Schema Contract Notes
- Analyze schema source: `internal/schema/analyze.go`.
- `operation` currently supports:
  - `api_validation`: `analyze`, `report`, `clear`
  - `annotations`: `flush`

## Code Anchors
- `cmd/browser-agent/internal/toolanalyze/analyzedispatch/dispatcher.go`
- `cmd/browser-agent/internal/toolanalyze/annotationanalysis/handler.go`
- `cmd/browser-agent/internal/toolobserve/dispatcher.go`
- `cmd/browser-agent/tools_async_completion.go`
- `internal/annotation/store.go`
- `internal/annotation/store_results.go`
- `internal/annotation/store_wait.go`
- `internal/schema/analyze.go`
