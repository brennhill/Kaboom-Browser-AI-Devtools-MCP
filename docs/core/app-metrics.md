# Kaboom App Telemetry Contract

Date: 2026-04-20

This document is the canonical telemetry contract for Kaboom app analytics sent to `POST /v1/event`.

Only the event types and fields defined here are part of the analytics contract. Unknown extra fields may be ignored by the ingest service and must not be used for analysis.

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
| `v` | string | yes | `0.8.5` | App version |
| `os` | string | yes | `darwin-arm64` | OS/platform identifier |
| `channel` | string | yes | `stable` | Release channel |
| `llm` | string | no | `claude-code` | MCP client name from initialize handshake |
| `screen` | string | no | `review` | Current visible surface |
| `workspace_bucket` | string | no | `2_5` | Approximate workload/project-size bucket |

Notes:

- Kaboom is the only producer. There is no `app` field.
- `iid` must remain stable for one install across launches and upgrades.
- `iid` is **install-stable for life** — there is no rotation API. The daemon prefers a stored ID over any deterministic re-derivation when both are present (a stale `.bak` won't resurrect a value, but it also won't be discarded in favor of a fresh derivation). If you need a "new install" identity, the user must wipe `~/.kaboom/` AND the platform-stable mirror together.
- Generate `iid` only when it can be durably persisted to `~/.kaboom/install_id`.
- `iid` creation must be serialized across concurrent daemon startups so one fresh install cannot mint multiple IDs.
- If a stable install ID cannot be read or written, drop telemetry instead of minting a replacement ID.
- `first_tool_call` claiming must be serialized across concurrent daemon startups so one install activates exactly once.
- `sid` must remain stable for one session and rotate on session boundaries.
- `sid` is normalized as lowercase hex by the ingest service.
- `ts` is the app event time. Analytics Engine write time must not be used as a substitute.
- `/v1/event` is daemon-owned. Installers, wrapper scripts, and extension service workers must not emit direct analytics rows.
- Privileged installers must not auto-start the daemon or write user-scoped analytics state on behalf of another home directory.

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
- `daemon_start` — operational anchor (see below)

In addition to those producer event types, the ingest service may write a storage-only row type named `malformed` for debugging invalid payloads.

### Operational events

`daemon_start` is emitted once per successful daemon boot from
`cmd/browser-agent/main_connection_mcp.go` via `telemetry.BeaconEvent`.
It anchors session-level correlation in observability tooling and
signals "the install is alive."

Producers MUST NOT add new top-level event names. Failure cases — panic,
boot failure, rate limit, parse error, etc. — belong in `app_error`
(see classifyAppError in `internal/telemetry/beacon.go` for the full
classification table).

### `app_error` codes used in production

`telemetry.AppError(code, props)` re-routes every call through the
`app_error` event with `error_code = uppercase(code)`. The full set of
codes currently in use:

| Code | Kind | Severity | Source | Emitter |
|------|------|----------|--------|---------|
| `daemon_panic` | internal | fatal | daemon | `cmd/browser-agent/main_helpers_panic.go` |
| `daemon_start_failed` | internal | error | daemon | `cmd/browser-agent/main_runtime_mode.go` |
| `tool_rate_limited` | integration | warning | daemon | `cmd/browser-agent/handler_tools_call.go` |
| `install_config_error` | internal | error | installer | `cmd/browser-agent/native_install.go` |
| `install_id_migrated` | integration | warning | daemon | `internal/telemetry/install_id_drift.go` (carries `derived_iid` prop linking the new derivation back to the canonical stored `iid`; cadence-bounded by the on-disk lineage marker `~/.kaboom/install_id_lineage` so a permanently-renamed host emits exactly once across all subsequent daemon starts) |
| `extension_disconnect` | integration | warning | extension | `internal/capture/sync.go` |
| `bridge_parse_error` | integration | warning | bridge | `cmd/browser-agent/internal/bridge/bridge_transport_helpers.go` |
| `bridge_method_not_found` | integration | warning | bridge | same |
| `bridge_stdin_error` | internal | error | bridge | same |
| `bridge_connection_error` | network | error | bridge | `cmd/browser-agent/internal/bridge/bridge_forward.go` |
| `bridge_port_blocked` | integration | error | bridge | `cmd/browser-agent/internal/bridge/bridge_startup_orchestration.go` |
| `bridge_spawn_build_error` | internal | fatal | bridge | `cmd/browser-agent/internal/bridge/bridge_startup_state.go` |
| `bridge_spawn_start_error` | internal | fatal | bridge | same |
| `bridge_spawn_timeout` | internal | error | bridge | same (retryable) |

Adding a new code: extend `classifyAppError` in
`internal/telemetry/beacon.go` AND this table in the same change.
Tests in `internal/telemetry/contract_compliance_test.go` and
`internal/telemetry/e2e_reporting_test.go` enforce the kind/severity
mapping.

### Daemon-owned on-disk artifacts

The daemon writes a small set of files at startup and during normal
operation. All paths are anchored at the **primary kaboom directory**
(`~/.kaboom/` on darwin/linux, `%USERPROFILE%\.kaboom\` on windows).
Most files are mirrored at the **platform-stable secondary mirror** so
a wipe of `~/.kaboom/` alone does NOT reset the install identity.

| File | Purpose | Mirrored to secondary? |
|------|---------|------------------------|
| `install_id` | The 12-char hex anonymous install ID. Source of truth for `iid`. | ✓ |
| `install_id.bak` | Backup of `install_id`; read fallback if primary is corrupt or missing. | ✓ |
| `install_id.lock` | Cross-process O_EXCL lock used by `withKaboomStateLock` while persisting `install_id`. Removed on success; cleaned up after `installStateLockStale=10s` if the writer crashed. | — |
| `install_id_lineage` | The most-recent derived ID seen by `CheckInstallIDDrift`. Cadence guard: ensures `install_id_migrated` beacons fire at most once per actual identity change across daemon starts. | — |
| `first_tool_call_install_id` | Marker indicating the install ID for which a `first_tool_call` event has already been emitted. Prevents duplicate emissions across restarts. | — |
| `first_tool_call_install_id.lock` | Cross-process O_EXCL lock used while persisting the marker. Removed on success; same staleness behavior as `install_id.lock`. | — |

**Platform-stable secondary mirror locations** (resolved by
`secondaryKaboomDirForOS` in `internal/telemetry/install_id.go`):

- darwin: `~/Library/Application Support/Kaboom/`
- linux: `$XDG_STATE_HOME/kaboom/` (fallback `~/.local/state/kaboom/`)
- windows: `%LOCALAPPDATA%\Kaboom\` (fallback `%USERPROFILE%\AppData\Local\Kaboom\`)

To fully reset the install identity (rare; users normally never need
this), wipe BOTH the primary kaboom directory AND the platform-stable
mirror. Wiping only one will result in self-heal restoring the missing
files from the other location on next daemon start.

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
  "v": "0.8.5",
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
  "v": "0.8.5",
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
  "v": "0.8.5",
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
| `active_window_m` | integer | no | `25` | Optional active minutes estimate |

Example:

```json
{
  "event": "session_end",
  "iid": "e41ce1f047c8",
  "sid": "8510f6ce8ca743c2",
  "ts": "2026-04-15T08:35:00Z",
  "v": "0.8.5",
  "os": "darwin-arm64",
  "channel": "stable",
  "reason": "shutdown",
  "duration_s": 1500,
  "tool_calls": 28
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
  "v": "0.8.5",
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

Recommended `error_kind` values:

- `network`
- `auth`
- `validation`
- `timeout`
- `internal`
- `integration`
- `unknown`

Example:

```json
{
  "event": "app_error",
  "iid": "e41ce1f047c8",
  "sid": "8510f6ce8ca743c2",
  "ts": "2026-04-15T08:11:00Z",
  "v": "0.8.5",
  "os": "darwin-arm64",
  "channel": "stable",
  "error_kind": "internal",
  "error_code": "DAEMON_PANIC",
  "severity": "fatal",
  "source": "daemon"
}
```

## Emission Rules

### Install identity

- Generate `iid` once per install.
- Persist it locally before emitting telemetry.
- If local persistence or readback fails, emit no telemetry until a stable `iid` is available.
- Keep it stable across launches and upgrades.

### Session identity

- Generate a new `sid` when a session begins.
- Rotate it after 30 minutes of inactivity.
- Rotate it when a session ends because of timeout, shutdown, restart, crash, or backgrounding.

### Event emission

- Emit `session_start` on the first activity in a new session.
- Emit `tool_call` for each meaningful tool invocation.
- Emit `first_tool_call` once per install ever.
- Emit `session_end` when the session closes.
- Emit `usage_summary` every 5 minutes when there has been activity in the window.
- Emit `app_error` only for runtime/product failures that are not one ordinary failed tool call.
- Do not emit installer, upgrade, or hooks-only analytics rows. Activation and adoption should be derived from canonical daemon events such as `first_tool_call`.

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
| `blob15` | async outcome |
| `blob16` | error kind |
| `blob17` | error code |
| `blob18` | severity, or joined validation errors for `malformed` rows |
| `blob19` | screen |
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
- `ts` is stored as `double1` and should be used for v2 time filtering.
- `malformed` rows use `error_kind = malformed_payload`, `error_code = json_parse_failed`, `contract_validation_failed`, or `body_read_failed`, and `source = ingest`.
- `malformed` rows store only a preview in `blob8`; the full raw payload is archived in R2 under `telemetry-malformed/...` and the archive key is stored in `blob11` when archival succeeds.
- product analytics queries should exclude `malformed` rows by default.

## Analysis Mapping

| Question | Primary source |
|----------|----------------|
| Monthly active installs | distinct `iid` over range |
| Tool popularity | `tool_call` and `tool_summary` |
| Tool latency | `tool_call.latency_ms` and `tool_summary.latency_*` |
| Error rate by tool | `tool_call.outcome = error` and `tool_summary.error_count` |
| Async reliability | `tool_call.async_outcome` and `async_outcome` rows |
| First-use funnel | `first_tool_call` |
| Session depth | `session_end.tool_calls` |
| Session length | `session_end.duration_s` |
| Install-level drilldown | all rows filtered by `iid` |
| Product/runtime failures | `app_error` |

## Reference Payloads

Minimal `tool_call`:

```json
{
  "event": "tool_call",
  "iid": "e41ce1f047c8",
  "sid": "8510f6ce8ca743c2",
  "ts": "2026-04-15T08:10:01Z",
  "v": "0.8.5",
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
  "v": "0.8.5",
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
