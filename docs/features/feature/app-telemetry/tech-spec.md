---
status: shipped
scope: feature/app-telemetry
ai-priority: medium
tags: [telemetry, architecture, privacy]
relates-to: [index.md, product-spec.md, qa-plan.md, ../../../core/app-metrics.md]
last-verified: 2026-07-29
doc_type: tech-spec
feature_id: feature-app-telemetry
last_reviewed: 2026-07-29
last_verified_version: 0.8.8
last_verified_date: 2026-07-29
---

# App Telemetry — Technical Specification

## Module Boundaries

The `internal/telemetry` package is divided by behavior that changes together:

| Module | Responsibility |
|---|---|
| `beacon.go` | Envelope construction, error classification, opt-out, bounded asynchronous HTTP delivery |
| `install_id.go` | Local install ID persistence and once-per-install first-tool marker |
| `session.go` | Session ID generation, inactivity rotation, and rotation callbacks |
| `usage_counter.go` | Thread-safe per-tool aggregation and session event construction |
| `usage_beacon.go` | Five-minute summary scheduling and shutdown handling |

These are direct modules, not compatibility facades. Callers use the canonical
telemetry APIs and the package owns its synchronization and local state.

## Data Flow

```text
tool or lifecycle event
        |
        v
UsageTracker / AppError
        |
        +--> session and install identity
        |
        +--> canonical event envelope
        |
        v
bounded fire-and-forget HTTP POST
        |
        v
https://t.gokaboom.dev/v1/event
```

`UsageTracker.RecordToolCall` updates counters under a mutex, touches the
session, emits lifecycle events when required, and emits the per-call event.
`SwapAndReset` atomically replaces the aggregate maps so reporting cannot lose
increments that arrive after the swap.

## Identity and Lifecycle

- The install ID is random, persisted at the installation root
  (`~/.kaboom/install_id`), cached after loading, and represented as 12
  lowercase hexadecimal characters. Runtime-state overrides never relocate it:
  project isolation and UAT must not manufacture new analytics identities.
- First creation publishes a fully written candidate atomically. Concurrent
  daemon starts therefore converge on the same persisted ID instead of caching
  different candidates while overwriting one another.
- The first-tool marker is persisted per install so `first_tool_call` is
  emitted once for that install.
- Session IDs are random 16-character lowercase hexadecimal values.
- `TouchSession` rotates a session after 30 minutes of inactivity and invokes
  the registered session-end callback.
- `GetSessionID` reads or creates identity but does not itself extend activity.
- Session duration uses an injectable clock in `UsageTracker`, keeping lifecycle
  tests deterministic.

## Delivery

`fireBeacon` checks opt-out before serialization or network access. It marshals
the payload once, then attempts to acquire a slot from a 50-entry semaphore.
When capacity is exhausted, the event is dropped. Sends run in a safe
background goroutine through a shared `http.Client` with a two-second timeout.
Response bodies are drained and closed for connection reuse. Only `202
Accepted` counts as accepted delivery; every other HTTP response is rejected,
and transport failures are counted separately. `/health` and `/diagnostics`
publish these payload-free outcome counters.

Telemetry sends use the network only; they do not log to stdout. MCP stdout
remains reserved for JSON-RPC protocol messages.

## Event Construction

The shared envelope supplies `event`, application version, OS/architecture,
install ID, session ID, and the MCP client name when known. Structured events
add the timestamp and release channel. Tool keys are split into `family:name`
for the canonical `family`, `name`, and `tool` fields.

Only the daemon owns event construction. The extension and shell
install/uninstall scripts never post directly to the ingest endpoint. Unknown
event names are rejected before serialization, so a producer cannot silently
create a legacy envelope.

The five-minute reporting loop swaps aggregates only when activity exists.
Context cancellation emits `session_end` with reason `shutdown` and terminates
the loop.

## Concurrency and Failure Properties

- Package state and tracker aggregates are mutex-protected.
- Installation identity is invariant across upgrade, runtime-state overrides,
  and concurrent daemon starts.
- Delivery concurrency is bounded without blocking producers.
- Empty snapshots and idle reporting windows do not emit events.
- Caller-supplied app-error properties cannot overwrite canonical error fields.
- Opt-out, HTTP failure, malformed internal payloads, and saturated delivery
  are fail-open with respect to Kaboom operation.

## Contracts

The authoritative wire contract is
[`docs/core/app-metrics.md`](../../../core/app-metrics.md). Changes to event
names, envelope fields, outcome values, session reasons, or summary shapes must
update that contract and the contract-compliance tests in the same change.

## Related Documents

- [Product specification](product-spec.md)
- [QA plan](qa-plan.md)
- [Feature index](index.md)
