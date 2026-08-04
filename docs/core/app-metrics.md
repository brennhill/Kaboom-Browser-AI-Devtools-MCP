# Kaboom App Telemetry Contract

Date: 2026-04-15

This document is the canonical telemetry contract for Kaboom app analytics sent to `POST /v1/event`.

Only the event types and fields defined here are part of the analytics contract. Unknown extra fields may be ignored by the ingest service and must not be used for analysis.

This product-usage envelope is the only exception to Kaboom's local-data
policy. It exists to measure anonymous active installs and product-command
usage. Producers must never include URLs, prompts, page content, file contents,
captured logs/network data, credentials, or personal data. Users can suppress
all delivery with `KABOOM_TELEMETRY=off`.

## Goals

This contract is designed to support:

- monthly active installs
- install-level drilldown
- tool and subtool usage
- first-use and activation analysis
- session depth and session duration
- per-tool latency and error rates
- async outcome analysis
- co-usage and workflow analysis
- product/runtime reliability analysis

## Endpoint

Kaboom sends app telemetry to:

```text
POST /v1/event
Content-Type: application/json
```

Production URL:

```text
https://t.gokaboom.dev/v1/event
```

Successful ingestion returns `202 Accepted`.

Malformed telemetry is fail-open:

- valid events write their normal normalized rows
- partially invalid `usage_summary` payloads salvage valid rows
- invalid payloads also write one `malformed` debug row
- the full malformed body is archived to R2 when available
- malformed payloads do not block ingestion with `400`

## Shared Envelope

Every event must include the shared envelope.

| Field | Type | Required | Example | Notes |
|-------|------|----------|---------|-------|
| `event` | string | yes | `tool_call` | Event type |
| `iid` | string | yes | `e41ce1f047c8` | Stable install ID |
| `sid` | string | yes | `8510f6ce8ca743c2` | Session ID, 16-character hex |
| `ts` | string | yes | `2026-04-15T08:10:00Z` | ISO-8601 UTC timestamp |
| `v` | string | yes | `0.8.6` | App version |
| `os` | string | yes | `darwin-arm64` | OS/platform identifier |
| `channel` | string | yes | `stable` | Release channel |
| `llm` | string | no | `claude-code` | MCP client name from initialize handshake |
| `screen` | string | no | `review` | Current visible surface |
| `workspace_bucket` | string | no | `2_5` | Approximate workload/project-size bucket |

Notes:

- Kaboom is the only producer. There is no `app` field.
- `iid` must remain stable for one install across launches and upgrades.
- `sid` must remain stable for one session and rotate on session boundaries.
- `sid` is normalized as lowercase hex by the ingest service.
- `ts` is the app event time. Analytics Engine write time must not be used as a substitute.

## Tool Identity

All tool-oriented events use the same identity model.

| Field | Type | Required | Example |
|-------|------|----------|---------|
| `family` | string | yes | `observe` |
| `name` | string | yes | `page` |
| `tool` | string | yes | `observe:page` |

Allowed families:

- `observe`
- `interact`
- `generate`
- `analyze`
- `configure`
- `ext`

Rules:

- `name` is open-ended but must be non-empty.
- `tool` must equal `family:name`.

## Event Types

The supported event set is:

- `tool_call`
- `first_tool_call`
- `session_start`
- `session_end`
- `usage_summary`
- `app_error`

In addition to those producer event types, the ingest service may write a storage-only row type named `malformed` for debugging invalid payloads.

### `tool_call`

Emit one event per meaningful tool invocation or command action.

| Field | Type | Required | Example | Notes |
|-------|------|----------|---------|-------|
| `family` | string | yes | `observe` | Tool family |
| `name` | string | yes | `page` | Tool/subtool name |
| `tool` | string | yes | `observe:page` | Combined tool key |
| `outcome` | string | yes | `success` | One of `success`, `error`, `cancelled`, `timeout`, `expired` |
| `latency_ms` | integer | no | `45` | End-to-end latency |
| `source` | string | no | `ui` | Runtime origin such as `ui`, `extension`, `mcp`, `background` |
| `async` | boolean | no | `true` | Whether the command was async |
| `async_outcome` | string | no | `timeout` | One of `complete`, `error`, `timeout`, `expired`, `cancelled` |

Example:

```json
{
  "event": "tool_call",
  "iid": "e41ce1f047c8",
  "sid": "8510f6ce8ca743c2",
  "ts": "2026-04-15T08:10:01Z",
  "v": "0.8.6",
  "os": "darwin-arm64",
  "channel": "stable",
  "family": "observe",
  "name": "page",
  "tool": "observe:page",
  "outcome": "success",
  "latency_ms": 45,
  "source": "ui"
}
```

### `first_tool_call`

Emit once per install, on the first tool call ever observed for that install.

| Field | Type | Required | Example |
|-------|------|----------|---------|
| `family` | string | yes | `observe` |
| `name` | string | yes | `page` |
| `tool` | string | yes | `observe:page` |

Example:

```json
{
  "event": "first_tool_call",
  "iid": "e41ce1f047c8",
  "sid": "8510f6ce8ca743c2",
  "ts": "2026-04-15T08:10:01Z",
  "v": "0.8.6",
  "os": "darwin-arm64",
  "channel": "stable",
  "family": "observe",
  "name": "page",
  "tool": "observe:page"
}
```

### `session_start`

Emit when a new session begins.

| Field | Type | Required | Example | Notes |
|-------|------|----------|---------|-------|
| `reason` | string | yes | `first_activity` | One of `first_activity`, `startup`, `post_timeout`, `resume` |

Example:

```json
{
  "event": "session_start",
  "iid": "e41ce1f047c8",
  "sid": "8510f6ce8ca743c2",
  "ts": "2026-04-15T08:10:00Z",
  "v": "0.8.6",
  "os": "darwin-arm64",
  "channel": "stable",
  "reason": "first_activity"
}
```

### `session_end`

Emit when a session closes or rotates.

| Field | Type | Required | Example | Notes |
|-------|------|----------|---------|-------|
| `reason` | string | yes | `timeout` | One of `timeout`, `shutdown`, `restart`, `crash`, `background` |
| `duration_s` | integer | yes | `1500` | Non-negative integer. `0` is valid for very short sessions. |
| `tool_calls` | integer | yes | `28` | Positive integer |
| `error_count` | integer | yes | `2` | Failed tool calls in this session |
| `outcome` | string | yes | `error` | `success` when `error_count` is zero, otherwise `error` |
| `active_window_m` | integer | no | `25` | Optional active minutes estimate |

Example:

```json
{
  "event": "session_end",
  "iid": "e41ce1f047c8",
  "sid": "8510f6ce8ca743c2",
  "ts": "2026-04-15T08:35:00Z",
  "v": "0.8.6",
  "os": "darwin-arm64",
  "channel": "stable",
  "reason": "shutdown",
  "duration_s": 1500,
  "tool_calls": 28,
  "error_count": 2,
  "outcome": "error"
}
```

### `usage_summary`

Emit a structured rollup every 5 minutes when there has been activity.

`usage_summary` is a rollup event. It is not a replacement for `tool_call`.

| Field | Type | Required | Example | Notes |
|-------|------|----------|---------|-------|
| `window_m` | integer | yes | `5` | Positive integer |
| `tool_stats` | array | yes | see below | One entry per tool in the window |
| `async_outcomes` | object | no | see below | Aggregate async outcome counts |

Each `tool_stats` entry:

| Field | Type | Required | Example |
|-------|------|----------|---------|
| `family` | string | yes | `observe` |
| `name` | string | yes | `page` |
| `tool` | string | yes | `observe:page` |
| `count` | integer | yes | `12` |
| `error_count` | integer | no | `1` |
| `latency_avg_ms` | integer | no | `45` |
| `latency_max_ms` | integer | no | `230` |

Allowed `async_outcomes` keys:

- `complete`
- `error`
- `timeout`
- `expired`
- `cancelled`

Example:

```json
{
  "event": "usage_summary",
  "iid": "e41ce1f047c8",
  "sid": "8510f6ce8ca743c2",
  "ts": "2026-04-15T08:35:00Z",
  "v": "0.8.6",
  "os": "darwin-arm64",
  "channel": "stable",
  "window_m": 5,
  "tool_stats": [
    {
      "family": "observe",
      "name": "page",
      "tool": "observe:page",
      "count": 12,
      "error_count": 0,
      "latency_avg_ms": 45,
      "latency_max_ms": 230
    },
    {
      "family": "interact",
      "name": "click",
      "tool": "interact:click",
      "count": 5,
      "error_count": 1,
      "latency_avg_ms": 1200,
      "latency_max_ms": 3500
    }
  ],
  "async_outcomes": {
    "complete": 7,
    "error": 1,
    "timeout": 1
  }
}
```

### `app_error`

Emit `app_error` for product/runtime failures that are not naturally modeled as a failed `tool_call`.

Use `tool_call` with `outcome = error` for normal user-invoked tool failures.

| Field | Type | Required | Example | Notes |
|-------|------|----------|---------|-------|
| `error_kind` | string | yes | `internal` | Broad error class |
| `error_code` | string | yes | `DAEMON_PANIC` | Stable short code |
| `severity` | string | yes | `fatal` | One of `warning`, `error`, `fatal` |
| `source` | string | no | `daemon` | Runtime origin |
| `retryable` | boolean | no | `true` | Whether the app could retry automatically |
| `outcome` | string | yes | `recovered` | `pending`, `recovered`, or `exhausted` |
| `attempt_bucket` | string | yes | `2_3` | `0`, `1`, `2_3`, or `4_plus` |
| `latency_bucket` | string | yes | `5s_30s` | `under_1s`, `1s_5s`, `5s_30s`, or `over_30s` |

Recommended `error_kind` values:

- `network`
- `auth`
- `validation`
- `timeout`
- `internal`
- `integration`
- `unknown`

Kaboom producers do not pass free-form values for these fields. Every runtime
failure uses a code from `internal/incident/registry.go`, which owns its source,
stage, severity, retryability, error kind, Doctor guidance, and
`bounded_product_metadata` privacy classification. Unknown codes are rejected
before an envelope is built. The current stable codes are:

`unclean_daemon_exit`, `daemon_restart_loop`,
`extension_reconnect_exhausted`, `tracked_tab_recovery_failed`,
`content_readiness_timeout`, `stale_command_result_rejected`,
`queue_saturated`, `state_recovery_failed`, `daemon_panic`,
`daemon_start_failed`, `tool_rate_limited`, `bridge_connection_error`,
`bridge_port_blocked`, `bridge_spawn_build_error`,
`bridge_spawn_start_error`, `bridge_spawn_timeout`, `bridge_exit_error`,
`extension_disconnect`, and `install_config_error`.

Example:

```json
{
  "event": "app_error",
  "iid": "e41ce1f047c8",
  "sid": "8510f6ce8ca743c2",
  "ts": "2026-04-15T08:11:00Z",
  "v": "0.8.6",
  "os": "darwin-arm64",
  "channel": "stable",
  "error_kind": "internal",
  "error_code": "DAEMON_PANIC",
  "severity": "fatal",
  "source": "daemon",
  "outcome": "pending",
  "attempt_bucket": "0",
  "latency_bucket": "under_1s"
}
```

## Emission Rules

### Install identity

- Generate `iid` once per install.
- Persist it at `~/.kaboom/install_id`, independently of configurable runtime
  state roots.
- Keep it stable across launches and upgrades.
- Automated tests must disable production telemetry and must not contribute
  install identities or activity rows.

### Session identity

- Generate a new `sid` when a session begins.
- Rotate it after 30 minutes of inactivity.
- Rotate it when a session ends because of timeout, shutdown, restart, crash, or backgrounding.

### Event emission

- The daemon is the only outbound producer. Extension and install/uninstall
  processes do not emit separate lifecycle envelopes.
- Emit `session_start` on the first activity in a new session.
- Emit `tool_call` for each meaningful tool invocation.
- Emit `first_tool_call` once per install ever.
- Emit `session_end` when the session closes.
- Emit `usage_summary` every 5 minutes when there has been activity in the window.
- Emit `app_error` only for runtime/product failures that are not one ordinary failed tool call.

### Delivery diagnostics

- Only `202 Accepted` confirms ingestion.
- Other HTTP statuses count as rejected delivery.
- Network errors, local drops, and opt-out/test suppressions are counted
  separately.
- `/health` and `/diagnostics` expose these aggregate counters under
  `telemetry_delivery`; they never expose event payloads or identities.

### Privacy

Do not send:

- prompts
- file contents
- project names
- URLs
- stack traces
- user identifiers
- anything that can identify a person or project

## Storage Model In Cloudflare Analytics Engine

The worker flattens incoming telemetry into normalized Analytics Engine rows.

Current row types:

- `tool_call`
- `first_tool_call`
- `session_start`
- `session_end`
- `tool_summary`
- `async_outcome`
- `app_error`
- `malformed`

Flattening rules:

- one `tool_call` row per `tool_call` event
- one `first_tool_call` row per `first_tool_call` event
- one `session_start` row per `session_start` event
- one `session_end` row per `session_end` event
- one `app_error` row per `app_error` event
- one `tool_summary` row per `usage_summary.tool_stats[]` entry
- one `async_outcome` row per `usage_summary.async_outcomes` key
- one `malformed` row for each invalid or partially invalid payload

Blob layout in `kaboomTelemetry`:

| Slot | Meaning |
|------|---------|
| `blob1` | app id (`kaboom`) |
| `blob2` | row type |
| `blob3` | event name |
| `blob4` | install id |
| `blob5` | session id |
| `blob6` | app version |
| `blob7` | os |
| `blob8` | tool, or raw payload preview for `malformed` rows |
| `blob9` | source or reason |
| `blob10` | family |
| `blob11` | name, or raw payload archive key for `malformed` rows |
| `blob12` | channel |
| `blob13` | llm (MCP client name) |
| `blob14` | outcome |
| `blob15` | async outcome, or `app_error.attempt_bucket` |
| `blob16` | error kind |
| `blob17` | error code |
| `blob18` | severity, or joined validation errors for `malformed` rows |
| `blob19` | screen, or `app_error.latency_bucket` |
| `blob20` | workspace bucket |

Double layout in `kaboomTelemetry`:

| Slot | Meaning |
|------|---------|
| `double1` | event time in ms since epoch |
| `double2` | count |
| `double3` | window minutes |
| `double4` | latency ms |
| `double5` | latency average ms |
| `double6` | latency max ms |
| `double7` | error count |
| `double8` | duration seconds |
| `double9` | tool calls |
| `double10` | active window minutes |
| `double11` | retryable (`1` or `0`) |

Index layout:

- `index1` stores `iid`

Notes:

- `tool_call`, `first_tool_call`, `session_start`, `session_end`, and `app_error` all write `count = 1`.
- `session_start.reason`, `session_end.reason`, and `app_error.source` are stored in `blob9`.
- `app_error.outcome` uses `blob14`, `attempt_bucket` uses `blob15`, and
  `latency_bucket` uses `blob19`; these fields are bounded enums and never carry
  correlation identifiers or local evidence.
- `ts` is stored as `double1` and should be used for v2 time filtering.
- `malformed` rows use `error_kind = malformed_payload`, `error_code = json_parse_failed`, `contract_validation_failed`, or `body_read_failed`, and `source = ingest`.
- `malformed` rows store only a preview in `blob8`; the full raw payload is archived in R2 under `telemetry-malformed/...` and the archive key is stored in `blob11` when archival succeeds.
- product analytics queries should exclude `malformed` rows by default.

## Analysis Mapping

| Question | Primary source |
|----------|----------------|
| Monthly active installs | distinct valid `iid` with activity in the trailing 30 days |
| Tool popularity | `tool_call` and `tool_summary` |
| Tool latency | `tool_call.latency_ms` and `tool_summary.latency_*` |
| Error rate by tool | `tool_call.outcome = error` and `tool_summary.error_count` |
| Async reliability | `tool_call.async_outcome` and `async_outcome` rows |
| First-use funnel | `first_tool_call` |
| Session depth | `session_end.tool_calls` |
| Session length | `session_end.duration_s` |
| Failure-free command rate | `tool_call` rows grouped by `outcome` |
| Failure-free completed session rate | `session_end` rows grouped by `outcome` |
| Daily/monthly active installs | distinct valid `iid` across accepted activity rows in the selected window |
| Crash-free active installs | active `iid` values with no fatal `app_error` or `UNCLEAN_DAEMON_EXIT` in the selected window |
| Crash-free active sessions | active `sid` values with no fatal `app_error`; report unclean-exit recovery sessions separately because abrupt termination cannot emit a final row |
| Install-level drilldown | all rows filtered by `iid` |
| Product/runtime failures and recovery effectiveness | `app_error`, grouped by `error_code`, `outcome`, `attempt_bucket`, and `latency_bucket` |

“Raw installs” means package-store/download or installer completion data and is
not inferred from telemetry identities. `first_tool_call` measures activated
installs; daily/monthly active installs measure distinct stable `iid` values
with accepted activity in the selected window. Keeping these denominators
separate prevents reinstalls, downloads that never run, and dormant installs
from inflating active-install health.

## Reference Payloads

Minimal `tool_call`:

```json
{
  "event": "tool_call",
  "iid": "e41ce1f047c8",
  "sid": "8510f6ce8ca743c2",
  "ts": "2026-04-15T08:10:01Z",
  "v": "0.8.6",
  "os": "darwin-arm64",
  "channel": "stable",
  "family": "observe",
  "name": "page",
  "tool": "observe:page",
  "outcome": "success"
}
```

Minimal `usage_summary`:

```json
{
  "event": "usage_summary",
  "iid": "e41ce1f047c8",
  "sid": "8510f6ce8ca743c2",
  "ts": "2026-04-15T08:35:00Z",
  "v": "0.8.6",
  "os": "darwin-arm64",
  "channel": "stable",
  "window_m": 5,
  "tool_stats": [
    {
      "family": "observe",
      "name": "page",
      "tool": "observe:page",
      "count": 12
    }
  ]
}
```
