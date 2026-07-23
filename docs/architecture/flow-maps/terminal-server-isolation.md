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
| Terminal server dies at runtime | Logged, `terminal_port` set to 0, main daemon unaffected |
| Main daemon dies | Terminal server also shuts down (graceful shutdown sequence) |
| Widget graphics/layout corruption in page overlay | User clicks header redraw (`↻`) to reset geometry and reload iframe without terminating PTY |
| Auto-sent annotation command collides with active user typing | Parent queues writes, waits for user idle, then submits and restores focus |
| `POST /terminal/start` fails to spawn a PTY | Real error always logged by `HandleTerminalStart`, then classified — see Start Failure Classification |

## Start Failure Classification

Being spawned by an MCP client does not prevent PTY spawning; an MCP-launched daemon spawns terminals normally. Only a fork/exec `EPERM` means the daemon genuinely cannot fork.

| Condition | Status | Body | Panel |
|-----------|--------|------|-------|
| Session ID already in use | 409 | `error`, `session_id`, existing `token` | reconnects silently |
| `*fs.PathError{Op:"fork/exec"}` wrapping `syscall.EPERM` | 503 | `sandbox_restricted` + `message`, `instruction`, `command`, `detail` | inline error with restart command |
| Anything else | 409 | `error` | inline error, no remedy offered |

`IsSandboxError` matches only the typed fork/exec `EPERM` shape. Matching a bare `"not permitted"` substring swept in every other `EPERM` in the spawn path (`open /dev/ptmx`, `TIOCPTYGRANT`, `TIOCPTYUNLK`, opening the slave, `TIOCSWINSZ`) and replaced the real cause with an unactionable "restart your daemon".

Two invariants hold regardless of classification:

- `detail` carries the underlying error, because the sandbox label is an inference and the error is the fact.
- Every spawn failure is logged with session ID, dir, and cmd; a failed start used to leave no trace in `~/.kaboom/logs/kaboom.jsonl`.

Client side, `startSession` routes every failure through `reportStartFailure` into the panel overlay. Non-sandbox rejections previously only `console.warn`ed and returned `null`, leaving the panel blank — indistinguishable from a broken terminal.

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

## Code Anchors

| File | Purpose |
|------|---------|
| `cmd/browser-agent/terminal_server.go` | `setupTerminalMux()`, `startTerminalServer()` |
| `cmd/browser-agent/terminal_handlers.go` | All terminal route handlers |
| `cmd/browser-agent/terminal_handlers_test.go` | Terminal handler + frame writer serialization tests |
| `cmd/browser-agent/internal/terminal/handlers.go` | `HandleTerminalStart()`, `IsSandboxError()`, `sandboxPayload()` |
| `cmd/browser-agent/internal/terminal/sandbox_error_test.go` | Sandbox attribution scope + `detail` preservation |
| `src/content/ui/terminal-widget-session.ts` | `startSession()`, `reportStartFailure()` |
| `tests/extension/terminal-session-start-errors.test.js` | Start-failure reporting reaches the panel for every failure mode |
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
