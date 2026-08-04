---
status: shipped
scope: feature/app-telemetry
ai-priority: medium
tags: [telemetry, testing, privacy]
relates-to: [index.md, product-spec.md, tech-spec.md, ../../../core/app-metrics.md]
last-verified: 2026-08-04
doc_type: qa-plan
feature_id: feature-app-telemetry
last_reviewed: 2026-08-04
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# App Telemetry — QA Plan

## Test Strategy

Tests use temporary state directories, injected clocks, local HTTP test
servers, configurable short reporting intervals, and package test hooks.
Telemetry tests must not contact the production endpoint. Tests in this package
run serially because endpoint, identity, session, and delivery controls are
package-level state.

## Coverage Map

| Area | Primary tests | Required evidence |
|---|---|---|
| Transport and envelope | `beacon_test.go` | Fire-and-forget behavior, timeout tolerance, bounded concurrency, response cleanup, required IDs and version |
| Canonical payload contract | `contract_compliance_test.go` | Required/omitted fields, error normalization, authoritative fields, concurrent lifecycle behavior |
| Install identity | `install_id_test.go` | Generation, persistence, state-directory selection, whitespace handling, read-failure recovery |
| Session identity | `install_id_test.go` | Format, stability, timeout rotation, activity refresh, concurrent access |
| Aggregation | `usage_counter_test.go` | Counts, errors, latency, async outcomes, atomic swap/reset, concurrent increments, session depth |
| Reporting loop | `e2e_usage_summary_test.go` | Active-window emission, idle skip, opt-out, cancellation |
| End-to-end events | `e2e_reporting_test.go` | Success/error tool calls, first use, app errors, slow ingest, JSON round trips |
| End-to-end sessions | `e2e_session_test.go` | Start/end reasons, timeout, shutdown, idle behavior, full lifecycle, opt-out |
| End-to-end summaries | `e2e_usage_summary_test.go` | Full payload, empty omission, nil snapshots, accumulation and reset |

## Required Scenarios

### Privacy and Opt-Out

1. Set `KABOOM_TELEMETRY=off` using lower- and mixed-case values.
2. Exercise individual events, summaries, and session lifecycle events.
3. Assert that no HTTP request reaches the local test server.
4. Inspect representative payloads and assert that only canonical contract
   fields are present; browser content and credentials must be absent.

### Identity and Sessions

1. Start with an empty temporary state directory.
2. Assert the install ID is 12-character hex and persists across state reload.
3. Assert the session ID is 16-character hex and stable during activity.
4. Advance the controlled activity time past 30 minutes.
5. Assert one timeout end event and one post-timeout start event are produced.
6. Assert concurrent tool calls do not duplicate `session_start`.

### Tool Calls and Aggregation

1. Record successful and failed calls for multiple tool families with controlled
   latencies.
2. Assert per-call events contain canonical identity, outcome, and latency.
3. Swap the aggregate snapshot and verify counts, error counts, average/max
   latency, and asynchronous outcomes.
4. Assert a second swap is empty and concurrent swaps plus increments preserve
   the total count.

### Reporting and Shutdown

1. Run the reporting loop with a short test interval.
2. Assert active windows emit one `usage_summary` and idle windows emit none.
3. Cancel the context and assert the loop exits and emits the appropriate
   shutdown session end when a session is active.

### Delivery Failure

1. Use an unreachable endpoint and a deliberately slow local endpoint.
2. Assert event-producing calls return without waiting for HTTP completion.
3. Saturate the delivery semaphore and assert excess events are dropped without
   blocking or leaking capacity.

## Commands

```bash
go test ./internal/telemetry
go test -race ./internal/telemetry
npm run docs:check:strict
```

The repository-wide completion gate remains `make test`; telemetry changes must
also preserve the MCP stdout invariant because telemetry is never permitted to
write protocol-external output there.

## Exit Criteria

- All telemetry package tests pass normally and with the race detector.
- Strict feature-document validation resolves every path and required document.
- The canonical contract and implementation agree on event names and fields.
- No test performs an external telemetry transmission.
- Opt-out and failure paths prove that telemetry cannot disrupt product use.

## Related Documents

- [Product specification](product-spec.md)
- [Technical specification](tech-spec.md)
- [Feature index](index.md)
