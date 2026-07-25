---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-03-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Flow Map: Terminal Server Isolation

## Overview

The terminal runs on a dedicated HTTP server (port+1) isolated from the main MCP daemon. This prevents shared timeouts, middleware, and connection handling from interfering with long-lived terminal WebSocket connections.

## WebSocket Keepalive

The terminal WebSocket uses server-initiated ping/pong keepalive instead of idle timeouts:

```
Server                          Browser
  │                                │
  ├─── ping (0x9) every 30s ────→ │
  │                                ├─── pong (0xA) auto-reply
  │ ←── pong ──────────────────── │
  │                                │
  │  [no frame for 60s = dead]     │
  ├─── conn.Close() ────────────→ │
  │                                ├─── onclose → auto-reconnect
  │                                │    (scrollback replayed)
```

- **Ping interval:** 30 seconds (server → browser)
- **Pong timeout:** 60 seconds (any frame resets deadline)
- **On dead connection:** WebSocket closed, PTY stays alive for reconnection
- **Idle users:** Never timed out — pong replies keep the connection alive indefinitely

## Concurrent Frame Write Safety

Terminal WebSocket traffic has three concurrent server-side writers:

1. PTY downstream output frames.
2. Keepalive ping frames.
3. Upstream control replies (pong/close).

As of 2026-03-05 these writers are funneled through a per-connection serialized writer (`newTerminalFrameWriter`) in `terminalWSLoop`, so frame bytes cannot interleave on the shared buffered socket writer.

## Startup Flow

```
runMCPMode()
  │
  ├─ startHTTPServer(port)          → Main daemon (MCP, capture, health)
  │    └─ AuthMiddleware(apiKey)
  │    └─ WriteTimeout: 65s
  │
  ├─ setupTerminalMux()             → New ServeMux with terminal routes only
  │    └─ registerTerminalRoutes()
  │    └─ No AuthMiddleware
  │
  ├─ startTerminalServer(port+1)    → Dedicated terminal server
  │    └─ WriteTimeout: 0 (WebSocket)
  │    └─ IdleTimeout: 0
  │    └─ On bind failure → log WARNING, continue without terminal
  │
  └─ awaitShutdownSignal()
       └─ termSrv.Shutdown() first (2s timeout)
       └─ srv.Shutdown() second (3s timeout)
```

## Port Assignment

```
Main:     127.0.0.1:{PORT}      (default 7890)
Terminal: 127.0.0.1:{PORT+1}    (default 7891)
```

## Extension URL Computation

```
Base URL:     http://localhost:7890
Terminal URL: http://localhost:7891  (port + TERMINAL_PORT_OFFSET)
```

Applied to: iframe src, fetch() calls, postMessage origin checks.

## Failure Modes

| Scenario | Behavior |
|----------|----------|
| Port+1 busy at startup | WARNING logged, daemon starts without terminal |
| Terminal server dies at runtime | Logged (`terminal_server_died`), `terminal_port` set to 0, main daemon unaffected, then `terminalSupervisor` auto-restarts it with exponential backoff (gives up loudly after 8 attempts) |
| Main daemon dies | Terminal server also shuts down (graceful shutdown sequence) |
| Widget graphics/layout corruption in page overlay | User clicks header redraw (`↻`) to reset geometry and reload iframe without terminating PTY |
| Auto-sent annotation command collides with active user typing | Parent queues writes, waits for user idle, then submits and restores focus |

## Widget Recovery Redraw

The terminal header includes a redraw control (`↻`) for client-side recovery when the overlay gets visually corrupted or dimensions drift.

Redraw behavior:
1. Restores default widget geometry (`50vw x 40vh`, bottom-right anchored).
2. Restores visible/open state if the widget was minimized.
3. Forces iframe reload to repaint terminal graphics using the same session token.
4. Re-sends `resize` and `focus` commands to refit xterm.js after reload.
5. Keeps the PTY session alive (no `/terminal/stop` call).

This is a UI-only recovery path; server session identity and process lifetime are unchanged.

Note: this header redraw control is separate from the queued-write `redraw` postMessage command, which is a soft xterm canvas refresh and does not reload iframe/session.

## Typing-Aware Write Guard

Parent widget now treats terminal auto-writes as queued jobs:

1. `writeToTerminal(text)` trims and enqueues text.
2. Queue only flushes when terminal is connected and user is idle.
3. Idle condition: terminal not focused, or last typing heartbeat older than 1.5s.
4. On defer, widget shows throttled toast: `waiting for user to stop typing`.
5. On flush, parent sends `redraw` then `write(text)`.
6. Enter submit (`\r`) is delayed (600ms) and re-guarded; if user starts typing again, submit waits.
7. If WS disconnects before queued submit, submit waits for reconnect and then resumes.
8. After submit, parent sends `focus` to return caret to xterm.

Iframe support in `terminal.html`:

- Emits `focus` events when xterm textarea gains/loses focus.
- Emits throttled `typing` heartbeat timestamps from `term.onData`.
- Handles new parent command `redraw` for soft canvas refresh without iframe reload.

## Reliability invariants (daemon-restart & shutdown)

- **Dead-session self-heal:** `Manager.Start` evicts a session whose child exited on its own and spawns fresh (`StartResult.Replaced`), so a restart/exit never strands the terminal on `ErrSessionExists`+dead fanout. The downstream pump distinguishes a genuine end from a backpressure drop via `Relay.ended`, emitting `exited` only on a real end.
- **Full-restart client recovery:** the iframe caps reconnects (`MAX_RECONNECT_ATTEMPTS`) and emits `reconnect_exhausted`; `sidepanel.ts` runs `redrawTerminal` (validate → rebuild). Reconnect-gap keystrokes are buffered and flushed on `replay_end`.
- **Bounded teardown:** `WriteBuffer.Close` is time-bounded; shutdown order is `StopAll` → `CloseAll` (close PTYs before draining write buffers) so a write blocked in `ptmx.Write` unblocks; `Session.Close`'s post-SIGKILL reap wait is bounded. Frame writes carry a `WSWriteTimeout` deadline; the supervisor won't publish a server bound after shutdown began.

## Code Anchors

| File | Purpose |
|------|---------|
| `cmd/browser-agent/terminal_server.go` | `setupTerminalMux()`, `startTerminalServer()` |
| `internal/pty/manager.go` | `Manager.Start` dead-session self-heal (`deleteSessionLocked`, `StartResult.Replaced`) |
| `cmd/browser-agent/internal/terminal/relay.go` | `Relay.ended` (session-end vs slow-drop), `readLoop` |
| `internal/pty/writebuf.go` | Time-bounded `Close` (`writeBufferCloseTimeout`) |
| `cmd/browser-agent/terminal_handlers.go` | All terminal route handlers |
| `cmd/browser-agent/terminal_handlers_test.go` | Terminal handler + frame writer serialization tests |
| `cmd/browser-agent/main_connection_mcp.go` | Terminal server startup wiring |
| `cmd/browser-agent/main_connection_mcp_shutdown.go` | Terminal server graceful shutdown |
| `cmd/browser-agent/server_routes_health_diagnostics.go` | `terminal_port` in health response |
| `cmd/browser-agent/terminal_assets/terminal.html` | xterm.js page, WS reconnect, postMessage bridge, focus/typing events, soft redraw command |
| `src/content/ui/terminal-widget.ts` | `getTerminalServerUrl()`, terminal widget state machine, redraw recovery control, queued typing guard |
| `extension/content/ui/terminal-widget.js` | Runtime copy consumed by extension tests and packaged content script |
| `tests/extension/terminal-widget.test.js` | Regression coverage for header controls and typing-aware queued write guard behavior |
| `src/lib/constants.ts` | `TERMINAL_PORT_OFFSET = 1` |

## Feature Docs

- [Terminal feature index](../../features/feature/terminal/index.md)
- [Terminal product spec](../../features/feature/terminal/product-spec.md)
- [Terminal tech spec](../../features/feature/terminal/tech-spec.md)
- [Terminal QA plan](../../features/feature/terminal/qa-plan.md)
- [Terminal flow-map pointer](../../features/feature/terminal/flow-map.md)
