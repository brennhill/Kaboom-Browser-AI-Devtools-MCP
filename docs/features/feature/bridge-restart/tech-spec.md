---
doc_type: tech_spec
feature_id: feature-bridge-restart
status: implemented
last_reviewed: 2026-11-21
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Bridge Restart — Tech Spec


## Architecture

Kaboom uses a **two-process model**:
- **Bridge** (foreground): stateless stdio-to-HTTP proxy, reads JSON-RPC from stdin, forwards `tools/call` to daemon via HTTP
- **Daemon** (background): persistent HTTP server on `127.0.0.1:<port>`, holds all state

The bridge already handles certain methods without the daemon ("fast path"): `initialize`, `tools/list`, `ping`, `resources/*`. This feature extends the fast path to intercept `configure(what="restart")` before the daemon check.

## Flow

```
LLM calls configure(what="restart")
  → bridge receives tools/call JSON-RPC on stdin
  → Runner.StdioToHTTPFast() detects configure+restart via handleBridgeRestart()
  → forceKillOnPort(): SIGCONT any frozen processes on the port
  → lifecycle.StopServerForUpgrade(): HTTP shutdown → SIGTERM → SIGKILL escalation
  → bridge resets daemonState (ready=false, fresh channels)
  → spawnDaemonAsync(): launch fresh daemon process
  → wait on readyCh/failedCh with 6s timeout
  → sendFastResponse(): return MCP tool result via stdout
```

## Key Functions

### `ExtractToolAction(req mcp.JSONRPCRequest) (toolName, action string)`

Defined in `cmd/browser-agent/internal/bridge/bridge_transport.go`; delegates to `internal/bridge` (`ExtractToolAction(method, params)`). Lightweight JSON parse of `req.Params` to extract the tool name and action field from a `tools/call` request. Returns empty strings for non-matching methods or parse failures.

### `Runner.forceKillOnPort(port int)`

Sends SIGCONT (via `processsignal.Resume`; no-op on Windows) to all processes bound to the port. This unfreezes SIGSTOP'd processes so that subsequent SIGTERM/SIGKILL from `StopServerForUpgrade` (`cmd/browser-agent/internal/daemonrecovery/primitives.go`) can be delivered. Without this, a frozen daemon would appear "released" to HTTP health checks but still hold the socket.

### `Runner.handleBridgeRestart(req, state, port, framing) bool`

Main orchestrator, in `cmd/browser-agent/internal/bridge/bridge_transport.go`. Returns true if the request was handled (i.e., it was `configure`+`restart`). Steps:
1. Extract tool action — bail if not configure+restart
2. `forceKillOnPort()` — unfreeze any stopped processes
3. `r.lifecycle.StopServerForUpgrade()` — kill daemon (HTTP → PID → lsof)
4. Reset `daemonState` fields (`ready`, `failed`, `err`, fresh signal channels)
5. `spawnDaemonAsync()` — launch fresh daemon
6. Wait on ready/failed channels with 6s timeout (`restartGracePeriod`)
7. Return MCP tool result via `sendFastResponse()`

### `handleConfigureRestart(req) JSONRPCResponse`

Daemon-side handler in `cmd/browser-agent/internal/toolruntime/tools_configure.go` for when the daemon is responsive. Sends self-SIGTERM via `util.SafeGo` after a 100ms delay (`restartSelfSignalDelay`) (so the response is sent first). The bridge detects the daemon died and auto-respawns.

## Insertion Point

In `Runner.StdioToHTTPFast()` (`cmd/browser-agent/internal/bridge/bridge.go`), the restart intercept sits **between** `handleFastPath()` and `checkDaemonStatus()`:

```go
// FAST PATH: initialize, tools/list, etc.
if r.handleFastPath(req, toolsList, framing) { ... }

// RESTART FAST PATH: configure(what="restart")
if r.handleBridgeRestart(req, state, port, framing) { ... }

// SLOW PATH: check daemon, forward to HTTP
if status := checkDaemonStatus(state, req, port); status != "" { ... }
```

This position ensures restart works even when the daemon is completely unresponsive (checkDaemonStatus would block or fail).

## Startup Contention Algorithm (Current)

Bridge startup now uses a single-host, lock-file startup leader election to converge concurrent clients quickly:

1. Probe existing daemon (`Runner.tryConnectToExisting`).
2. If absent, attempt startup lock acquisition (`startuplock.Manager.Acquire`, lock file `run/bridge-startup-<port>.lock.json` under the state root) in `Runner.coordinateDaemonStartup`.
3. Lock holder becomes leader (`startAsStartupLeader`) and spawns daemon immediately.
4. Followers poll for peer readiness within bounded wait budget (`waitForPeerDaemon`).
5. If lock owner is stale/dead, follower reclaims lock (`ClearStale`) and takes over startup.
6. During `starting`, `tools/call` requests are forwarded to converge instead of returning transient startup retry envelopes.

Reference implementation:

- `cmd/browser-agent/internal/bridge/bridge_startup.go`
- `cmd/browser-agent/internal/bridge/startuplock/lock.go`
- `cmd/browser-agent/internal/bridge/bridge_startup_state.go`
- `cmd/browser-agent/internal/bridge/bridge.go`

## Response Format

```json
{
  "status": "ok",
  "restarted": true,
  "message": "Daemon restarted successfully",
  "previous_stopped": true
}
```

On failure:
```json
{
  "status": "error",
  "restarted": false,
  "message": "Daemon restart failed: <reason>",
  "previous_stopped": true
}
```
