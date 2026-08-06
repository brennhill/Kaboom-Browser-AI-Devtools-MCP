---
status: shipped
scope: feature/app-telemetry
ai-priority: medium
tags: [telemetry, privacy, reliability]
relates-to: [index.md, tech-spec.md, qa-plan.md, ../../../core/quality/app-metrics.md]
last-verified: 2026-08-04
doc_type: product-spec
feature_id: feature-app-telemetry
last_reviewed: 2026-08-04
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# App Telemetry — Product Specification

## Purpose

App telemetry gives the Kaboom maintainers anonymous product-usage and
reliability signals without collecting browser content or making telemetry a
dependency of any user operation. The canonical event and field definitions
are in the [app telemetry contract](../../../core/quality/app-metrics.md).

## User and Product Outcomes

- Maintainers can measure daily/monthly active installs, crash-free installs
  and sessions, failure-free commands, tool adoption, session depth, latency,
  error rates, and asynchronous outcomes.
- A stable anonymous install identifier distinguishes installations without
  identifying a person.
- A short-lived session identifier groups activity and rotates after
  inactivity.
- Users can disable all outbound telemetry with `KABOOM_TELEMETRY=off`.
- Telemetry failure, saturation, or an unavailable ingest service never blocks
  an MCP tool call or daemon lifecycle operation.

## Privacy Boundary

Telemetry may contain the fields defined by the canonical contract: anonymous
install and session identifiers, application version, platform, release
channel, MCP client name, tool family/name, outcome, aggregate latency, error
classification, and aggregate counts.

It must not include page contents, captured console output, network bodies,
URLs, user-entered text, credentials, or other browser telemetry. Install state
is stored locally in Kaboom's state directory. Event delivery is the only
outbound transmission in this feature.

## Functional Requirements

1. Each installation has a stable 12-character hexadecimal install ID.
2. Each activity session has a 16-character hexadecimal session ID and rotates
   after 30 minutes of inactivity.
3. A tool invocation emits a `tool_call`; the first invocation for an install
   also emits `first_tool_call`.
4. Active sessions emit `session_start` and `session_end` at the lifecycle
   boundaries defined in the canonical contract. Session-end rows include
   bounded call/error counts and a derived success/error outcome.
5. Active tool and asynchronous outcome counters are summarized every five
   minutes; idle windows emit no summary.
6. Known application failures emit normalized `app_error` events.
7. Delivery is asynchronous, uses a two-second HTTP timeout, and caps
   concurrent beacon sends at 50.
8. `KABOOM_TELEMETRY=off` is case-insensitive and suppresses every beacon.

## Non-Goals

- User identity, cross-device correlation, or behavioral advertising.
- Uploading captured browser data.
- Guaranteed delivery, retry queues, or durable event buffering.
- Exposing telemetry as an MCP tool or writing telemetry diagnostics to MCP
  stdout.

## Acceptance Criteria

- The event envelopes and payloads conform to
  [`docs/core/quality/app-metrics.md`](../../../core/quality/app-metrics.md).
- Opt-out causes no HTTP request.
- Network failure and concurrency saturation remain best-effort and nonblocking.
- Install/session state and aggregate counters remain race-safe.
- Session rotation, shutdown, first-use, idle-window, and summary-reset behavior
  are covered by deterministic automated tests.

## Related Documents

- [Technical specification](tech-spec.md)
- [QA plan](qa-plan.md)
- [Feature index](index.md)
