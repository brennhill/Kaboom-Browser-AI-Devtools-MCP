---
doc_type: tech_spec
feature_id: feature-bridge-restart
status: implemented
last_reviewed: 2026-07-05
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
  → bridgeStdioToHTTPFast() detects configure+restart via handleBridgeRestart()
  → forceKillOnPort(): SIGCONT any frozen processes on the port
  → stopServerForUpgrade(): HTTP shutdown → SIGTERM → SIGKILL escalation
  → bridge resets daemonState (ready=false, fresh channels)
  → spawnDaemonAsync(): launch fresh daemon process
  → wait on readyCh/failedCh with 6s timeout
  → sendFastResponse(): return MCP tool result via stdout
```

## Key Functions

### `extractToolAction(req JSONRPCRequest) (toolName, action string)`

Lightweight JSON parse of `req.Params` to extract the tool name and action field from a `tools/call` request. Returns empty strings for non-matching methods or parse failures.

### `forceKillOnPort(port int)`

Sends SIGCONT to all processes bound to the port. This unfreezes SIGSTOP'd processes so that subsequent SIGTERM/SIGKILL from `stopServerForUpgrade` can be delivered. Without this, a frozen daemon would appear "released" to HTTP health checks but still hold the socket.

### `handleBridgeRestart(req, state, port) bool`

Main orchestrator. Returns true if the request was handled (i.e., it was `configure`+`restart`). Steps:
1. Extract tool action — bail if not configure+restart
2. `forceKillOnPort()` — unfreeze any stopped processes
3. `stopServerForUpgrade()` — kill daemon (HTTP → PID → lsof)
4. Reset `daemonState` fields
5. `spawnDaemonAsync()` — launch fresh daemon
6. Wait on ready/failed channels with 6s timeout
7. Return MCP tool result via `sendFastResponse()`

### `toolConfigureRestart(req) JSONRPCResponse`

Daemon-side handler for when the daemon is responsive. Sends self-SIGTERM via `util.SafeGo` after 100ms delay (so the response is sent first). The bridge detects the daemon died and auto-respawns.

## Insertion Point

In `bridgeStdioToHTTPFast()`, the restart intercept sits **between** `handleFastPath()` and `checkDaemonStatus()`:

```go
// FAST PATH: initialize, tools/list, etc.
if handleFastPath(req, toolsList) { ... }

// RESTART FAST PATH: configure(what="restart")
if handleBridgeRestart(req, state, port) { ... }

// SLOW PATH: check daemon, forward to HTTP
if status := checkDaemonStatus(state, req, port); status != "" { ... }
```

This position ensures restart works even when the daemon is completely unresponsive (checkDaemonStatus would block or fail).

## Startup Contention Algorithm (Current)

Bridge startup now uses a single-host, lock-file startup leader election to converge concurrent clients quickly:

1. Probe existing daemon (`tryConnectToExisting`).
2. If absent, attempt startup lock acquisition (`bridge-startup-<port>.lock.json`).
3. Lock holder becomes leader and spawns daemon immediately.
4. Followers poll for peer readiness within bounded wait budget.
5. If lock owner is stale/dead, follower reclaims lock and takes over startup.
6. During `starting`, `tools/call` requests are forwarded to converge instead of returning transient startup retry envelopes.

Reference implementation:

- `cmd/browser-agent/bridge_startup_orchestration.go`
- `cmd/browser-agent/bridge_startup_lock.go`
- `cmd/browser-agent/bridge_startup_state.go`
- `cmd/browser-agent/bridge.go`

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
