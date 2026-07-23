---
doc_type: feature_index
feature_id: feature-terminal
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-23
code_paths:
  - src/lib/brand.ts
  - cmd/browser-agent/terminal_handlers.go
  - cmd/browser-agent/terminal_server.go
  - cmd/browser-agent/terminal_assets/terminal.html
  - extension/sidepanel.html
  - extension/sidepanel.js
  - src/content/ui/terminal-panel-bridge.ts
  - src/content/ui/terminal-widget-session.ts
  - src/content/ui/terminal-widget-types.ts
  - src/content/ui/tracked-hover-launcher.ts
  - src/background/message-handlers.ts
  - src/background/terminal-panel.ts
  - src/background/side-panel-availability.ts
  - src/background/keyboard-shortcuts.ts
  - src/background/context-menus.ts
  - src/types/runtime-messages.ts
  - src/sidepanel.ts
  - internal/pty/manager.go
  - internal/pty/session.go
test_paths:
  - tests/extension/brand-metadata.test.js
  - cmd/browser-agent/terminal_handlers_test.go
  - tests/extension/sidepanel-terminal.test.js
  - tests/extension/terminal-widget-session-branding.test.js
  - tests/extension/tracked-hover-launcher.test.js
  - tests/extension/message-handlers.test.js
  - internal/pty/manager_test.go
  - internal/pty/session_test.go
  - cmd/browser-agent/internal/terminal/sandbox_error_test.go
  - tests/extension/terminal-session-start-errors.test.js
  - tests/extension/terminal-panel-gesture-entrypoints.test.js
  - tests/extension/terminal-panel-open-failure.test.js
  - tests/extension/terminal-panel-presence.test.js
  - tests/extension/terminal-panel-close-and-scope.test.js
last_verified_version: 0.8.5
last_verified_date: 2026-07-23
---

# Terminal

## TL;DR
- Status: shipped
- Side-panel terminal host that embeds a PTY-backed shell via iframe
- Availability: macOS + Linux only (Windows currently reports terminal unavailable / `terminal_port: 0`)
- Runs on a **dedicated HTTP server** at `main_port + 1` (e.g., 7891) for isolation
- Singleton session shared across all tabs via `chrome.storage.session`
- One Kaboom work context maps to one Chrome tab group; the panel opens synchronously on the requesting tab (to keep the user gesture valid), then the tracked tab is grouped into the workspace best-effort
- Three UI states: **open**, **minimized**, **closed** - all persisted across page refreshes
- Hover launcher keeps the page overlay for quick actions, but the terminal button now opens the side panel on the active workspace tab and hides the launcher only while the panel is open
- Three entry points open the panel, all through the shared `openTerminalSidePanel()` in `src/background/terminal-panel.ts`: the page context menu item **Open Kaboom Terminal**, the keyboard command `open_terminal_panel` (**ships unbound** — Chrome refuses a manifest with more than four `suggested_key` commands, and four are already taken, so users assign a key at `chrome://extensions/shortcuts`), and the in-page launcher button. The first two get a full user gesture from Chrome and are dependable; the launcher button goes through `runtime.onMessage`, which Chrome grants only a *restricted* gesture that `sidePanel.open()` rejects on some Chrome/Brave builds ([crbug 355266358](https://issues.chromium.org/issues/355266358)) — so it is best-effort, and its failure toast names the other two
- A failed open is always reported (console error + toast). It used to be swallowed by `catch { return false }`, which made the Terminal button look simply dead
- Background must call `chrome.sidePanel.open()` synchronously in the forwarded click gesture path; **nothing may be awaited before it** — neither `resolveTerminalWorkspaceTarget()` nor `setOptions()`, or Chrome expires the gesture and refuses to open the panel (regression: the panel loaded nothing). Workspace grouping + `setOptions()` run afterward as best-effort refinement.
- Header redraw control (`↻`) reloads iframe graphics without killing the PTY session
- Header close control (`✕`, rightmost) closes the drawer and **leaves the shell running** — reopening reconnects to the same session. Closing must never destroy a shell the user only wanted out of the way
- Header power control (`⏻`) is the explicit teardown: stops the PTY and closes the panel. Kept because sessions are capped (`maxSessions = 10`), so there must be a way to end one
- `chrome.sidePanel.close()` only exists in very recent Chrome. `closeBrowserSidePanel()` falls back to `window.close()` from the panel document; the old code returned silently when the API was absent, so the close button did nothing and `unmountPanel()` left a blank panel the user could neither close nor recover
- A session-less panel renders a **recoverable** state — a *Start terminal* button and a **root folder** field — instead of a dead sentence. Changing the root folder stops the session and restarts it there, because a PTY's cwd is fixed at spawn
- The page context menu item is a **toggle**: it reads *Open Kaboom Terminal* or *Close Kaboom Terminal* depending on whether a panel document is actually connected, refreshed via `contextMenus.onShown`
- **Panel liveness comes from a port, not from storage.** The panel connects `TERMINAL_PANEL_PORT` for as long as it lives; `isTerminalPanelOpenSync()` is `livePanelPort !== null`. Mirroring `TERMINAL_UI_STATE` did not work: dismissing the panel with Chrome's own X destroys the document before it can record anything, so the flag stuck at "open" and the toggle kept trying to close a panel that was already gone — the user could never reopen it. `TERMINAL_UI_STATE` remains the launcher's hover-overlay signal
- **The panel is scoped to the tracked tab by default.** The manifest's `side_panel.default_path` makes it available on every tab, where it renders empty; `syncTerminalPanelAvailability()` disables the global default and enables the panel only on the tracked tab. That governs the default only — an explicit open enables its target tab first, because `chrome.sidePanel.open()` on a disabled tab fails with **"No active side panel for tabId: N"**, which is what made the Terminal button dead on every untracked page
- **The panel path never varies.** Changing `path` in `setOptions` reloads the side panel document, so the old per-tab `sidepanel.html?tabId=…` (set right after `open()`) booted an xterm and immediately destroyed it. Only `tabId` was ever read, and `getHostTabId()` already falls back to the active tab
- **Opening onto an existing panel restores it.** `sidePanel.open()` on a live panel only focuses it — no code runs inside — so the opener also posts `restore_terminal_panel` over the presence port. `restoreTerminalPanel()` re-shows a mounted terminal, rebuilds an unmounted one (revalidating the token so the xterm reconnects to a live shell), or retries session start when the panel is mounted with no terminal
- Header minimize control hides the side panel while preserving the current PTY session
- The current side panel rollout is terminal-only; xterm fills the available panel height
- Terminal startup failure guidance now consistently points users at the Kaboom daemon command: `npx kaboom-agentic-browser`
- Any legacy or fallback terminal shell that still mounts from content-script code now uses `Kaboom Terminal` so mixed-brand terminal chrome does not reappear.
- Annotation auto-send now uses a typing-aware write queue: if the user is active in terminal, writes wait until ~1.5s idle
- Queued submit is reconnect-safe: if WS drops before Enter, submit waits until connection is back
- WebSocket frame writes are serialized per-connection to prevent concurrent writer frame interleaving
- Scrollback buffer capped at 256 KB for memory safety
- PTY session tests share a bounded `readUntilContains` helper to keep echo/size assertions consistent
- Canonical flow maps: [terminal-side-panel-host.md](../../../architecture/flow-maps/terminal-side-panel-host.md), [terminal-server-isolation.md](../../../architecture/flow-maps/terminal-server-isolation.md)
- Feature flow-map pointer: [flow-map.md](./flow-map.md)

---

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Flow Map Pointer: [flow-map.md](./flow-map.md)

## Architecture

### Dedicated Terminal Server

The terminal runs on its own `http.Server` on **port+1** (e.g., main daemon on 7890, terminal on 7891). This isolates:

- **Timeouts**: Main server has `WriteTimeout: 65s` for MCP blocking tools. Terminal server has `WriteTimeout: 0` for long-lived WebSocket connections.
- **Middleware**: Main server uses `AuthMiddleware` for API key validation. Terminal server uses its own session token validation (no AuthMiddleware).
- **Failure isolation**: If the main server has issues, the terminal keeps running. If the terminal server dies, the main daemon logs it but keeps serving MCP.

### Port Assignment

| Server | Port | Purpose |
|--------|------|---------|
| Main daemon | `PORT` (default 7890) | MCP, capture, health, diagnostics |
| Terminal | `PORT + 1` (default 7891) | Terminal HTML, static assets, WebSocket, session lifecycle |

The terminal port is surfaced in:
- `/health` HTTP response as `terminal_port`
- MCP `configure(what: "health")` response in `server.terminal_port`
- Startup lifecycle logs

### Port Conflict Handling

If port+1 is already in use at startup:
- **Logged loudly** to stderr with actionable instructions
- **Lifecycle event** `terminal_server_bind_failed` logged
- **Main daemon continues** — terminal is non-essential
- `/health` response omits `terminal_port` (signals terminal unavailable)
- MCP health returns `terminal_port: 0`

If the terminal server dies at runtime:
- Logged as `terminal_server_died`
- `terminal_port` set to 0
- Main daemon is **not** affected

---

## Workspace Model

Kaboom now treats the terminal side panel as belonging to one browser work context:

- **One work context = one Chrome tab group**
- **One main tab** anchors that workspace group
- **Any tab inside the workspace group** can host the visible side panel
- **Tabs outside the workspace group** must redirect panel open to a workspace tab

The initial rollout keeps the broader extension tracking contract on `TRACKED_TAB_ID`, but terminal workspace resolution upgrades the tracked tab into a named Chrome tab group when needed and persists workspace ownership separately from ordinary tracked-tab UI state.

## UI Panel State Machine

The terminal panel has three visual states, tracked by the `TerminalUIState` type:

```
                 open_terminal_panel
    ┌───────────────────────────────────────┐
    │                                       ▼
 CLOSED ──browser side panel opened──────► OPEN ──minimizePanel()──► MINIMIZED
    ▲                                       │                           │
    │          browser side panel closed    │                           │
    └───────────────────────────────────────┘                           │
    ▲                                                                   │
    └──────────────exitTerminalSession()─────────────────────────────────┘
```

### State Descriptions

| State | Visual | PTY Session | Persisted As |
|-------|--------|-------------|-------------|
| **Closed** | Side panel closed, hover launcher visible again | Stopped | `'closed'` or cleared |
| **Open** | Full side panel visible (terminal header + terminal iframe) | Active, WebSocket connected | `'open'` |
| **Minimized** | Side panel hidden, hover launcher visible again | Active, WebSocket reconnectable | `'minimized'` |

### State Transitions

| Action | Trigger | From → To |
|--------|---------|-----------|
| `openTerminalPanel()` | Launcher button click or popup action | Closed/Minimized → Open (starts session if needed) |
| `browser side panel closed` | Browser UI | Open → Minimized or Closed depending on persisted intent |
| `minimizePanel()` | Minimize (▁) button | Open → Minimized |
| `exitTerminalSession()` | Power (⏻) button | Open/Minimized → Closed (kills PTY) |
| `side panel page load` | Browser reopens panel | Restores previous state from persistence |

### Key Distinction: Close vs Exit

- **Minimize** - The browser side panel is closed but the PTY session stays alive on the daemon and the launcher becomes visible again.
- **Exit** (`exitTerminalSession`) - Kills the PTY process on the daemon (`POST /terminal/stop`), clears persisted session, closes the side panel, and resets the panel host completely.

---

## Session Management

### Singleton Session Model

The terminal uses a **singleton session** — one PTY session shared across all tabs in the browser. This is because `chrome.storage.session` (where the session token is persisted) is scoped to the entire extension session, not per-tab.

### Storage Layers

| Storage | Scope | Keys | Purpose |
|---------|-------|------|---------|
| `chrome.storage.session` | Browser session (all tabs) | `TERMINAL_SESSION`, `TERMINAL_UI_STATE` | Active session token + UI state; clears on browser close |
| `chrome.storage.local` | Persistent (survives restart) | `TERMINAL_CONFIG`, `TERMINAL_AI_COMMAND`, `TERMINAL_DEV_ROOT` | User preferences: shell, AI command, dev root path |

### Session Token Flow

```
Extension                          Terminal Server (port+1)
   │                                        │
   ├─ POST /terminal/start ────────────────►│ Creates PTY, returns {session_id, token}
   │◄────────── {session_id, token} ────────┤
   │                                        │
   ├─ Persist token to chrome.storage.session
   │                                        │
   ├─ Open iframe: /terminal?token=...     │
   │     └─ iframe connects WS:            │
   │        /terminal/ws?token=... ────────►│ Validates token, upgrades to WebSocket
   │◄────────── scrollback replay ──────────┤
   │◄────────── live PTY I/O ──────────────►│
```

### Session Persistence Across Page Refresh and Panel Reopen

1. On every state change, the side panel writes `{token, sessionId}` and `uiState` to `chrome.storage.session`.
2. On panel load, the side panel host reads the persisted state.
3. If a session exists:
   - Validates the token against the daemon (`GET /terminal/validate?token=...`).
   - If valid: mounts the panel in the persisted UI state (open or minimized).
   - If invalid (daemon restarted, process died): clears stale state and starts a fresh session.
4. The hover launcher observes `TERMINAL_UI_STATE` and hides only while the panel is open.

### Session Conflict (409)

If the client calls `POST /terminal/start` with an ID that already exists:
- Server returns HTTP 409 with the existing session's token.
- Client reconnects using the returned token instead of creating a new session.
- This prevents orphaned sessions from accumulating.

### CWD Priority

When starting a session, the working directory is resolved in this order:
1. `dir` from the request body (explicit)
2. `active_codebase` set via MCP/extension (`server.GetActiveCodebase()`)
3. Auto-detected from the first registered MCP client's CWD
4. Falls back to the daemon's working directory

---

## Scrollback and Memory

### Scrollback Buffer (Server-Side)

The daemon maintains a **256 KB ring buffer** per session (`session.go:maxScrollback`). Every byte read from the PTY is appended via `AppendScrollback()`. When the buffer exceeds 256 KB, the oldest bytes are evicted (trimmed from the front).

On WebSocket reconnect (page refresh), the entire scrollback buffer is replayed to the client in 4 KB chunks, so the user sees prior output immediately.

### xterm.js Scrollback (Client-Side)

The `terminal.html` xterm.js instance has `scrollback: 1500` lines. This is intentionally low — the terminal is for interactive use, not log viewing. The server-side 256 KB buffer handles reconnect replay, so the browser doesn't need to retain deep history. Combined:
- **Reconnect replay**: last 256 KB of raw terminal output (server-side)
- **In-session scroll**: last 1,500 lines of rendered text (browser-side)

### Memory Pressure

- Server-side: 256 KB per session is fixed and bounded. With the singleton model (one session), this is negligible.
- Client-side: xterm.js manages its own memory. The 10,000-line scrollback is the main consumer.
- The WebSocket idle timeout (`terminalWSIdleTimeout = 5 minutes`) closes stale connections that stop sending data, preventing resource leaks.

---

## WebSocket Protocol

### Frame Types

| Direction | Opcode | Content |
|-----------|--------|---------|
| PTY → Browser | Binary (0x2) | Raw terminal output bytes |
| Browser → PTY | Binary (0x2) | Raw keystroke bytes |
| Browser → PTY | Text (0x1) | JSON control messages (e.g., `{"type":"resize","cols":80,"rows":24}`) |
| Both | Ping/Pong (0x9/0xA) | Keep-alive |
| Both | Close (0x8) | Graceful disconnect |

### Reconnect Behavior

The `terminal.html` WebSocket has built-in auto-reconnect with exponential backoff (1s → 2s → 4s → ... → 10s max). On reconnect:
1. New WebSocket handshake to `/terminal/ws?token=...`
2. Server replays scrollback buffer
3. Client sends resize control message
4. Server sends `SIGWINCH` to force TUI redraw (even if dimensions haven't changed)

### Connection Status Dot

The iframe sends `postMessage` events to the parent panel host:
- `connected` → green dot
- `disconnected` → orange dot
- `exited` → red dot

---

## Extension Integration

### Terminal Server URL Computation

The extension computes the terminal server URL from the base daemon URL:
```typescript
function getTerminalServerUrl(baseUrl: string): string {
  const url = new URL(baseUrl)
  url.port = String(parseInt(url.port || '7890', 10) + TERMINAL_PORT_OFFSET)
  return url.origin
}
```

`TERMINAL_PORT_OFFSET = 1` is defined in `src/lib/constants.ts`.

### PostMessage Bridge

The side panel host communicates with the terminal iframe via `postMessage`:

| Direction | Message | Purpose |
|-----------|---------|---------|
| Parent → Iframe | `{target: 'kaboom-terminal', command: 'focus'}` | Focus the xterm.js instance |
| Parent → Iframe | `{target: 'kaboom-terminal', command: 'resize'}` | Refit terminal after panel resize |
| Parent → Iframe | `{target: 'kaboom-terminal', command: 'redraw'}` | Soft redraw xterm canvas without iframe/session reload |
| Parent → Iframe | `{target: 'kaboom-terminal', command: 'write', text: '...'}` | Write text to PTY stdin |
| Iframe → Parent | `{source: 'kaboom-terminal', event: 'connected'}` | WebSocket connected |
| Iframe → Parent | `{source: 'kaboom-terminal', event: 'disconnected'}` | WebSocket disconnected |
| Iframe → Parent | `{source: 'kaboom-terminal', event: 'exited'}` | PTY process exited |
| Iframe → Parent | `{source: 'kaboom-terminal', event: 'focus', data: { focused }}` | xterm focus/blur state updates |
| Iframe → Parent | `{source: 'kaboom-terminal', event: 'typing', data: { at }}` | Throttled typing heartbeat timestamp |

Origin validation: parent only accepts messages from the terminal server origin. Iframe sends to `*` (since it doesn't know the parent's origin in advance).

### Queued Write Guard

When `writeToTerminal()` is called (for example from annotation auto-send), the panel host queues writes and applies a focus guard:

1. If terminal is connected and user is idle, write is sent immediately.
2. If terminal has focus and recent typing (< 1.5s), write is deferred.
3. A warning toast is shown (`waiting for user to stop typing`) at a throttled interval.
4. After idle clears, the panel host soft-redraws terminal, writes text, then sends `\r`.
5. If WebSocket disconnects before submit, queued Enter waits until reconnect, then continues.
6. Focus is returned to xterm after submit.

If the user re-focuses and types again during the auto-submit window, Enter is deferred again until idle.

---

## PTY Layer

### Manager (`internal/pty/manager.go`)

- Manages a map of `sessionID → *Session` and `token → sessionID`
- Tokens are 32-byte cryptographic random hex strings
- Thread-safe: all operations hold `sync.RWMutex`
- `Stop()` removes map entries under lock, then calls `sess.Close()` outside the lock to avoid blocking concurrent reads during slow child process teardown

### Session (`internal/pty/session.go`)

- Wraps a PTY master fd + child process
- `Spawn()`: opens `/dev/ptmx`, grants/unlocks slave, sets initial `winsize`, starts child with `Setsid + Setctty`
- `Close()`: sends `SIGTERM`, closes PTY master, waits up to 2s for child exit, escalates to `SIGKILL` if needed
- `Resize()`: `TIOCSWINSZ` ioctl on the PTY master
- `ForceRedraw()`: sends `SIGWINCH` directly to the child process (used on reconnect when dimensions match but display is stale)
- Environment: inherits from parent process, adds `TERM=xterm-256color`

### Default Shell

`HandleTerminalStart` resolves the shell via `DefaultShell()` when the caller does not name one. It was hardcoded to `/bin/zsh`, so **every terminal start failed on Linux** (`fork/exec /bin/zsh: no such file or directory`) — most Linux installs do not ship zsh. macOS never saw it because zsh is the system default there.

Resolution order:

| Step | Source | Why |
|------|--------|-----|
| 1 | `$SHELL`, if it is an **executable regular file** | the user's actual shell wins. `os.Stat` alone is not enough — a `$SHELL` pointing at a directory or a non-executable leftover would otherwise reach `fork/exec` |
| 2 | first of `zsh`, `bash`, `sh` found on **`$PATH`** | no path is assumed. `/bin/bash` does not exist on distros without usr-merge, Homebrew and NixOS install shells elsewhere, and Alpine ships no bash at all — the golang:1.24 image resolves bash to `/usr/bin/bash` |
| 3 | `/bin/sh` | POSIX requires it; guarantees a non-empty command |

zsh is only *first in preference*, never required — step 2 skips whatever is missing.

Tests (`sandbox_error_test.go`): the chosen shell must be executable on the machine running the test; `$SHELL` wins when usable; a missing, non-executable, or directory `$SHELL` falls through; a candidate reachable only via `PATH` is still found; an empty `PATH` still yields `/bin/sh`.

### Start Failure Reporting

Being spawned by an MCP client does **not**, by itself, prevent the daemon from spawning a PTY — a daemon launched over MCP stdio spawns terminals normally. Only a genuinely restricted sandbox profile blocks it, and its signature is a fork/exec `EPERM`.

`IsSandboxError` (`cmd/browser-agent/internal/terminal/handlers.go`) therefore matches exactly that shape: an `*fs.PathError` with `Op == "fork/exec"` wrapping `syscall.EPERM`. It previously substring-matched `"not permitted"` anywhere in the error, which swept in every other `EPERM` in the spawn path (`open /dev/ptmx`, `TIOCPTYGRANT`, `TIOCPTYUNLK`, opening the slave, `TIOCSWINSZ`) and replaced the real cause with a confident "restart your daemon" instruction that fixed nothing.

Reporting rules, in order of precedence:

| Condition | Status | Body | Panel |
|-----------|--------|------|-------|
| Session ID already in use | 409 | `error`, `session_id`, existing `token` | reconnects silently |
| fork/exec `EPERM` | 503 | `sandbox_restricted` + `message`, `instruction`, `command`, `detail` | inline error with restart command |
| Anything else | 409 | `error` | inline error, no remedy offered |

Two invariants:

- **`detail` always carries the underlying error.** The sandbox label is an inference; the error is the fact. `startSession` appends it to the message so a misattribution is still diagnosable from the panel.
- **Every spawn failure is logged** by `HandleTerminalStart` with the session ID, dir, and cmd. Previously a failed start left no trace in `~/.kaboom/logs/kaboom.jsonl`, so the only signal was whichever label the client happened to render.

On the client, `startSession` routes *all* failures through `reportStartFailure` to the panel's error overlay. Non-sandbox rejections used to only `console.warn` and return `null`, so the panel rendered nothing — indistinguishable from the terminal being broken.

Tests: [`cmd/browser-agent/internal/terminal/sandbox_error_test.go`](../../../../cmd/browser-agent/internal/terminal/sandbox_error_test.go), [`tests/extension/terminal-session-start-errors.test.js`](../../../../tests/extension/terminal-session-start-errors.test.js)

---

## Routes (on terminal server, port+1)

| Route | Method | Purpose |
|-------|--------|---------|
| `/terminal` | GET | Serve terminal HTML page (embedded in binary) |
| `/terminal/static/` | GET | Serve xterm.js, xterm.css (embedded FS) |
| `/terminal/ws` | GET→101 | WebSocket upgrade for PTY I/O (token-validated) |
| `/terminal/start` | POST | Create a new PTY session (returns token) |
| `/terminal/stop` | POST | Destroy a PTY session (kills process) |
| `/terminal/validate` | GET | Check if a session token maps to a live session |
| `/terminal/config` | GET | List active sessions and count |

Note: `/config/active-codebase` is on the **main** daemon server (not terminal server) — it's not terminal-specific.

---

## Code Paths

| File | Responsibility |
|------|---------------|
| `cmd/browser-agent/terminal_server.go` | Dedicated server setup: `setupTerminalMux()`, `startTerminalServer()` |
| `cmd/browser-agent/terminal_handlers.go` | All HTTP handlers: page, WS, start, stop, validate, config |
| `cmd/browser-agent/terminal_assets/terminal.html` | xterm.js terminal page with WS reconnect and postMessage bridge |
| `extension/sidepanel.html` | Side panel shell that loads the terminal host |
| `src/sidepanel.ts` | Side panel UI: terminal shell, terminal iframe, write guard, session restore |
| `src/content/ui/terminal-panel-bridge.ts` | Content-script bridge for opening the panel and forwarding writes |
| `src/content/ui/terminal-widget-session.ts` | Shared terminal session persistence and lifecycle helpers |
| `src/content/ui/terminal-widget-types.ts` | Shared terminal state, timing, and DOM ids |
| `src/content/ui/tracked-hover-launcher.ts` | Hover launcher terminal button + launcher hide/show coordination |
| `src/lib/constants.ts` | `TERMINAL_PORT_OFFSET`, storage keys |
| `internal/pty/manager.go` | Session manager: create, get, destroy, token auth |
| `internal/pty/session.go` | PTY session: spawn, I/O, resize, scrollback, close |
| `cmd/browser-agent/main_connection_mcp.go` | Terminal server startup wiring |
| `cmd/browser-agent/main_connection_mcp_shutdown.go` | Terminal server graceful shutdown |
