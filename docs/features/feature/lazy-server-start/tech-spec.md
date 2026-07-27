---
doc_type: tech-spec
feature_id: feature-lazy-server-start
status: proposed
feature: lazy-server-start
owners: []
last_reviewed: 2026-07-05
links:
  index: ./index.md
  product: ./product-spec.md
  qa: ./qa-plan.md
code_paths:
  - cmd/browser-agent/internal/bridge/bridge_startup_orchestration.go
  - cmd/browser-agent/internal/bridge/bridge_startup.go
  - cmd/browser-agent/internal/bridge/bridge_startup_state.go
  - cmd/browser-agent/internal/bridge/bridge_startup_status.go
  - cmd/browser-agent/internal/bridge/bridge_fastpath.go
  - cmd/browser-agent/internal/bridge/bridge_forward.go
  - cmd/browser-agent/internal/daemonlife/lifecycle.go
  - cmd/browser-agent/internal/launchmode/launch_mode.go
  - cmd/browser-agent/internal/toolguard/guards.go
test_paths:
  - cmd/browser-agent/internal/bridge/lazy_server_start_test.go
  - cmd/browser-agent/internal/bridge/bridge_spawn_race_test.go
  - cmd/browser-agent/internal/bridge/bridge_fastpath_unit_test.go
  - cmd/browser-agent/internal/bridge/bridge_startup_contention_test.go
  - cmd/browser-agent/internal/launchmode/launch_mode_test.go
  - cmd/browser-agent/tools_coldstart_gate_test.go
---

# Lazy Server Start Tech Spec

> Plain language only. Describes HOW the bridge starts, recovers, and coordinates the daemon without blocking the Model Context Protocol (MCP) stdio handshake.

## TL;DR

- Design: the stdio bridge answers read-only MCP methods from a fast path, spawns the daemon asynchronously, and makes only `tools/call` wait on a readiness signal.
- Key constraints: never block the stdio read loop, elect a single startup leader across concurrent bridges, recover from a dead daemon on the next request.
- Rollout risk: medium — touches the hot startup path, but every change is guarded by deterministic unit tests with injected clocks and ports.

## Architecture Overview

Every MCP client launches the Kaboom binary in **bridge mode**. The bridge is a thin process that translates MCP messages on standard input and output (stdio) into HTTP requests against a local daemon on `127.0.0.1:<port>`. The daemon holds all telemetry state; the bridge holds none.

`RunMode` drives the lifecycle:

1. Build the daemon state object with `readyCh` and `failedCh` signal channels.
2. Phase 1 — `tryConnectToExisting`: if a compatible daemon already serves the port, mark the state ready and skip spawning.
3. Phase 2 — if no daemon is found, call `startDaemonSpawnCoordinator`, which runs the peer-wait and spawn policy on a background goroutine so stdio handling is never delayed.
4. Start `StdioToHTTPFast`, which begins reading MCP messages immediately.

The critical invariant is that the read loop starts before, or concurrently with, daemon spawning. A cold daemon must never delay `initialize` or `tools/list`.

## Key Components

**Fast path (`bridge_fastpath.go`).** `handleFastPath` answers methods that need no daemon: `initialize`, `initialized`, `tools/list`, `resources/list`, `resources/templates/list`, `resources/read`, plus static `ping` and `prompts/list`. For `initialize`, the bridge negotiates the protocol version, stores the client's push capabilities, records the stdio framing, and returns server info from in-process constants. JSON-RPC notifications (requests with no identifier) are dropped without a response.

**Startup coordinator (`bridge_startup_orchestration.go`).** `coordinateDaemonStartup` elects a single startup leader using a port-scoped lock:
- `tryAcquireBridgeStartupLock` — if acquired, this bridge becomes the leader, spawns the daemon, and holds leadership until the spawn resolves so followers do not stampede.
- If the lock is held by a peer, `waitForPeerDaemon` polls for a compatible daemon within the follower wait budget.
- If the leader appears stalled, `clearStaleBridgeStartupLock` reclaims a stale or dead lock and the bridge retries acquisition.
- A final short wait covers the window where the leader finishes just as the lock hands off.

**Daemon state and recovery (`bridge_startup_state.go`, `bridge_startup_status.go`).** `daemonState` tracks `ready`, `failed`, and an error string behind a mutex, with resettable signal channels. `checkDaemonStatus` classifies an incoming request: it returns `"starting"` while the daemon boots inside the grace period and an empty string once ready. `respawnIfNeeded` re-launches a daemon that stopped responding and resets the failure state.

**Request forwarding (`bridge_forward.go`).** Forwarded requests proxy to the daemon over HTTP. On a connection error the bridge calls `respawnIfNeeded` and retries. Responses with status code 500 or higher are treated as `retryable`, and the structured error carries that flag back to the client.

**Daemon singleton lifecycle (`internal/daemonlife/lifecycle.go`).** `daemonlife.EnforceStartupPolicy` reads the daemon lock record and either validates parallel isolation (for `--state-dir`-isolated runs) or performs a default takeover. Takeover is conservative: it requires the lock process identifier to match the port's process-identifier file before terminating anything, reclaims stale mismatched locks only when the port is not serving, and never kills a non-Kaboom service.

**Launch-mode classification (`internal/launchmode/launch_mode.go`).** `launchmode.Classify` receives the daemon flag, terminal interactivity, and detected parent process while inspecting supervisor environment markers to label the launch `persistent` or `likely_transient`. `launchmode.Warning` produces the advisory string; `launchmode.EnforcePersistent` upgrades it to a hard error when `KABOOM_REQUIRE_PERSISTENT` is set.

**Extension readiness guard (`internal/toolguard/guards.go`).** `RequireExtension` blocks a tool call until the extension reports connectivity, waiting up to `capture.ExtensionReadinessTimeout`. This window lets the extension's one-second sync loop reconnect after a cold daemon start before the call fails.

## Data Flow

```
MCP client launches `kaboom ... ` in bridge mode
  |
  v
RunMode(port, logFile, maxEntries)
  |-- tryConnectToExisting(port)?  --- yes --> markReady(), skip spawn
  |        | no
  |        v
  |   startDaemonSpawnCoordinator(state, port)   [background goroutine]
  |        |-- coordinateDaemonStartup: acquire startup lock
  |        |     |-- leader  -> spawnDaemonAsync, hold until ready/failed
  |        |     |-- follower -> waitForPeerDaemon (poll every 100ms)
  |        |-- on stall -> clearStaleBridgeStartupLock -> retry
  v
StdioToHTTPFast(serverURL+"/mcp", state, port)   [starts immediately]
  |
  |-- initialize / tools/list / resources/* --> handleFastPath (no daemon)
  |
  |-- tools/call --> checkDaemonStatus
  |        |-- "starting" -> wait on readyCh up to grace period, then forward
  |        |-- ready      -> forward over HTTP
  |        v
  |   forward: on connection error -> respawnIfNeeded -> retry
  |            on HTTP >= 500       -> structured error {retryable: true}
  v
daemon responds; result streamed back over stdio
```

Meanwhile the extension polls `/sync` every second. On the first successful sync after a (re)start it fires `onConnectionChange(true)`, clears the badge, and resends tracked-tab state.

## Timing Constants

| Constant | Value | Role |
|----------|-------|------|
| `daemonStartupGracePeriod` | 2s | How long `tools/call` waits while the daemon boots before erroring |
| `daemonStartupReadyTimeout` | 2s | Leader's readiness wait after spawning |
| `daemonPeerWaitTimeout` | 2s | Follower's budget to wait for the leader's daemon |
| `daemonPeerPollInterval` | 100ms | Poll cadence while waiting for a peer daemon |
| `daemonPeerFallbackWaitTimeout` | 250ms | Final short wait during lock handoff |
| `daemonStartupLockStaleAfter` | 2s | Age after which a startup lock is reclaimed |

These are package variables so tests can shrink them for deterministic timing.

## Implementation Strategy

**Bridge (`cmd/browser-agent/internal/bridge/`):** the fast path, startup coordinator, daemon state, and forwarding live here. Startup leadership uses a port-scoped lock file in the state directory; daemon state uses channels rather than sleeps so waiters wake on the exact transition.

**Daemon process (`cmd/browser-agent/`):** `internal/daemonlife/lifecycle.go` enforces the singleton via the lock record; `internal/launchmode/launch_mode.go` classifies the launch; `internal/toolguard/guards.go` gates tool calls on extension readiness.

**Extension (`src/popup/`):** `tab-tracking.ts` keeps tracking state in `chrome.storage.local`; `status-display.ts` renders the connected/offline state. The sync client polls every second with no backoff so reconnection is prompt.

**Trade-offs:**
- Asynchronous spawn versus synchronous bind: asynchronous spawn keeps the handshake fast but requires careful leader election to avoid duplicate daemons.
- Fixed grace period versus unbounded wait: a bounded grace period guarantees `tools/call` returns a result or a retryable error rather than hanging forever.
- Conservative takeover versus aggressive reclaim: refusing to kill mismatched or non-Kaboom processes trades a rare manual cleanup for never killing the wrong process.

## Edge Cases and Assumptions

### Edge Cases

- **No daemon running:** `tryConnectToExisting` returns false and the coordinator spawns one; fast-path methods still answer immediately.
- **Daemon already running and compatible:** state is marked ready and no spawn occurs.
- **Incompatible daemon version:** the old daemon is stopped for upgrade; if it cannot be recycled, the state is marked fatally failed (non-retryable).
- **Port held by a non-Kaboom service:** the state is marked failed with the occupying service name; the bridge never kills it.
- **Concurrent bridges:** the startup lock elects one leader; followers poll and only reclaim a stale lock if the leader stalls.
- **Daemon crash mid-session:** the next forwarded request triggers `respawnIfNeeded` and retries; `tools/call` waits rather than failing instantly.
- **Extension offline at tool time:** `requireExtension` waits up to the readiness timeout, then returns a structured retryable error.

### Assumptions

- A1: the daemon is always local (`127.0.0.1`); no remote transport is involved.
- A2: the MCP client relaunches the bridge per session, so cold start is the common path.
- A3: the state directory is writable for the lock and process-identifier files.
- A4: the extension's one-second sync loop is the canonical reconnection mechanism.

## Risks and Mitigations

### Risk 1: Duplicate daemons from a spawn race
- Description: two bridges spawn daemons simultaneously and fight over the port.
- Mitigation: port-scoped startup lock elects one leader; the leader holds leadership until its spawn resolves; followers wait and reclaim only on stall. Covered by `bridge_spawn_race_test.go` and `bridge_startup_contention_test.go`.

### Risk 2: Cold start blocks the handshake
- Description: a slow daemon boot delays `initialize` or `tools/list`.
- Mitigation: those methods are answered from the fast path with no daemon dependency; spawning runs on a background goroutine. Covered by `lazy_server_start_test.go`.

### Risk 3: A crashed daemon ends the session
- Description: a daemon dies and every subsequent call fails.
- Mitigation: `respawnIfNeeded` re-launches on the next request and the call is retried; persistent failure returns `retryable: true` so the client can retry.

### Risk 4: Takeover kills the wrong process
- Description: an aggressive reclaim terminates an unrelated process on the port.
- Mitigation: takeover requires lock-to-process-identifier-file agreement, reclaims stale mismatches only when the port is not serving, and never terminates a non-Kaboom service.

## Performance

| Operation | Budget | Method |
|-----------|--------|--------|
| Fast-path response | < 50ms | In-process JSON encode, no daemon round trip |
| `tools/call` cold wait | <= grace period (~2s) | Block on `readyCh` with deadline |
| Peer wait poll | 100ms cadence | `waitForPeerDaemonWithin` loop |
| Extension reconnect | <= 1s | One-second `/sync` poll, no backoff |
