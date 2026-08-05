---
doc_type: feature_index
feature_id: feature-terminal
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-05
code_paths:
  - internal/pty/upload/upload.go
  - src/content/ui/hover/screenshot-feedback.ts
  - src/content/ui/panel/host-tab.ts
  - src/content/ui/panel/shell.ts
  - src/content/ui/panel/status-indicators.ts
  - cmd/browser-agent/internal/terminal/ws.go
  - src/lib/brand.ts
  - cmd/browser-agent/internal/terminal/handlers.go
  - cmd/browser-agent/internal/terminal/spawn_retry.go
  - cmd/browser-agent/internal/terminal/relay.go
  - cmd/browser-agent/internal/terminal/dirs.go
  - cmd/browser-agent/internal/terminal/server.go
  - cmd/browser-agent/internal/terminal/supervisor/supervisor.go
  - cmd/browser-agent/main_connection_mcp.go
  - cmd/browser-agent/internal/daemonlife/lifecycle.go
  - cmd/browser-agent/internal/nativeinstall/installer.go
  - cmd/browser-agent/internal/terminal/intent_handlers.go
  - cmd/browser-agent/internal/terminal/static.go
  - cmd/browser-agent/internal/terminal/terminal_assets/terminal.html
  - extension/sidepanel.html
  - extension/sidepanel.js
  - src/content/ui/terminal-panel-bridge.ts
  - src/content/ui/terminal-widget-session.ts
  - src/lib/storage/recovery.ts
  - src/lib/storage/validated.ts
  - src/content/ui/terminal-widget-types.ts
  - src/lib/terminal-server.ts
  - src/content/ui/terminal-root-folder.ts
  - src/content/ui/terminal-panel-states.ts
  - src/content/ui/terminal-write-guard.ts
  - src/content/ui/tracked-hover-launcher.ts
  - src/background/ui/terminal-panel.ts
  - src/background/ui/terminal-workspace.ts
  - src/background/ui/side-panel-availability.ts
  - src/background/message-handlers.ts
  - src/background/message-routing/utility-handler.ts
  - src/types/runtime-messages.ts
  - src/sidepanel.ts
  - internal/pty/manager.go
  - internal/pty/session.go
  - internal/pty/writebuf.go
  - internal/pty/diagnostics/diagnostics.go
  - internal/pty/fanout/fanout.go
  - npm/kaboom-agentic-browser/lib/daemon/kill-daemon.js
  - scripts/tests/framework/framework.sh
  - scripts/tests/workflows/cat-28-terminal.sh
test_paths:
  - tests/extension/terminal-reconnect/terminal-html-liveness.test.js
  - cmd/browser-agent/internal/terminal/relay_rebind_test.go
  - cmd/browser-agent/internal/terminal/relay_test.go
  - cmd/browser-agent/internal/terminal/fakes_test.go
  - cmd/browser-agent/internal/terminal/spawn_retry_test.go
  - cmd/browser-agent/internal/terminal/sandbox_error_test.go
  - cmd/browser-agent/internal/terminal/handlers_start_decisions_test.go
  - tests/extension/terminal-session/terminal-start-pending.test.js
  - tests/extension/branding/brand-metadata.test.js
  - tests/extension/terminal-session/terminal-write-guard.test.js
  - cmd/browser-agent/internal/terminal/dirs_test.go
  - cmd/browser-agent/internal/terminal/handlers_test.go
  - cmd/browser-agent/internal/terminal/handlers_extra_test.go
  - cmd/browser-agent/internal/terminal/ws_panic_test.go
  - cmd/browser-agent/internal/terminal/supervisor/supervisor_test.go
  - tests/extension/terminal-sidepanel/sidepanel-terminal-fixture.js
  - tests/extension/terminal-sidepanel/sidepanel-terminal-io.test.js
  - tests/extension/terminal-sidepanel/sidepanel-terminal-ui.test.js
  - tests/extension/terminal-sidepanel/sidepanel-terminal.test.js
  - tests/extension/terminal-session/terminal-widget-session-branding.test.js
  - tests/extension/terminal-session/terminal-root-folder.test.js
  - tests/extension/terminal-session/terminal-session-start-errors.test.js
  - cmd/browser-agent/internal/terminal/handlers_logging_test.go
  - tests/extension/terminal-panel/terminal-panel-presence.test.js
  - tests/extension/terminal-panel/terminal-panel-close-and-scope.test.js
  - tests/extension/terminal-panel/terminal-panel-open-failure.test.js
  - tests/extension/terminal-panel/terminal-panel-gesture-entrypoints.test.js
  - tests/extension/ui-controls/tracked-hover-launcher.test.js
  - tests/extension/state-recovery/state-recovery-contract.test.js
  - tests/extension/state-recovery/validated-storage.test.js
  - tests/extension/terminal-panel/terminal-panel-bridge.test.js
  - tests/extension/content/message-handlers.test.js
  - tests/extension/contracts/background-boundaries.test.js
  - tests/extension/terminal-panel/terminal-panel-gesture-entrypoints.test.js
  - tests/extension/terminal-panel/terminal-panel-presence.test.js
  - tests/extension/terminal-session/terminal-session-stop.test.js
  - tests/extension/contracts/entry-point-parity.test.js
  - internal/pty/manager_test.go
  - internal/pty/session_test.go
  - internal/pty/runtime_lifecycle_test.go
  - internal/pty/diagnostics/diagnostics_test.go
  - internal/pty/fanout/fanout_test.go
  - internal/pty/upload/upload_test.go
  - cmd/browser-agent/internal/terminal/session_end_signal_test.go
  - cmd/browser-agent/internal/terminal/frame_writer_deadline_test.go
  - cmd/browser-agent/internal/terminal/handlers_fanout_test.go
  - cmd/browser-agent/internal/terminal/handlers_replay_deadline_test.go
  - cmd/browser-agent/internal/terminal/relay_boundary_test.go
  - cmd/browser-agent/internal/terminal/relay_replace_test.go
  - cmd/browser-agent/internal/terminal/relay_close_test.go
  - cmd/browser-agent/internal/terminal/relay_init_test.go
  - cmd/browser-agent/internal/terminal/intent_handlers_test.go
  - cmd/browser-agent/internal/daemonlife/lifecycle_takeover_test.go
  - cmd/browser-agent/internal/nativeinstall/connect_refused_test.go
  - tests/extension/terminal-reconnect/terminal-html-reconnect.test.js
  - tests/extension/terminal-reconnect/terminal-reconnect-recovery-contract.test.js
  - tests/extension/terminal-reconnect/terminal-reconnect-budget-contract.test.js
  - tests/extension/terminal-reconnect/terminal-port-discovery.test.js
  - tests/extension/terminal-reconnect/terminal-iframe-message-contract.test.js
  - npm/kaboom-agentic-browser/lib/daemon/kill-daemon.test.js
  - tests/cli/contracts/uat-harness-regressions.test.cjs
last_verified_version: 0.8.1
last_verified_date: 2026-03-28
---

# Terminal

## TL;DR

- Start and resize dimensions are bounded before conversion to PTY wire widths;
  oversized external values are rejected instead of wrapping. Intent counts use
  one bounded synchronization helper.
- Status: shipped
- Side-panel terminal host that embeds a PTY-backed shell via iframe
- Availability: macOS + Linux only (Windows currently reports terminal unavailable / `terminal_port: 0`)
- Runs on a **dedicated HTTP server** at `main_port + 1` (e.g., 7891) for isolation
- Singleton session shared across all tabs via `chrome.storage.session`
- One Kaboom work context maps to one Chrome tab group; the panel opens on a workspace tab, not whichever tab sent the request
- Three UI states: **open**, **minimized**, **closed** - all persisted across page refreshes
- Hover launcher keeps the page overlay for quick actions, but the terminal button now opens the side panel on the active workspace tab and hides the launcher only while the panel is open
- Background must call `chrome.sidePanel.open()` in the original click gesture path; tab-specific `setOptions()` cannot be awaited first or Chrome may refuse to open the panel
- Header redraw control (`↻`) reloads iframe graphics without killing the PTY session
- Header power control (`⏻`) closes the side panel and ends the PTY session
- AI launches classify Claude and Codex authentication before execution. The
  terminal header persistently identifies Subscription, API billing, or Unknown;
  known API credentials and provider overrides require an explicit `y` confirmation
  before the CLI starts. Codex uses `codex login status`, Claude uses
  `claude auth status`, and Claude's own `API Usage Billing` output remains a
  fallback warning. Only the provider classification is surfaced—never account
  identifiers or credential values.
- Saved CLI authentication is authoritative over unrelated API environment
  variables: a confirmed Codex ChatGPT or Claude subscription is not downgraded
  to API billing merely because another provider's credential is exported.
- Header minimize control hides the side panel while preserving the current PTY session
- The current side panel rollout is terminal-only; xterm fills the available panel height
- Terminal startup failure guidance now consistently points users at the Kaboom daemon command: `npx kaboom-agentic-browser`
- Implicit terminal shells start in login mode so launchd-managed daemons load the user's profile and command `PATH`; explicitly requested commands retain their exact arguments.
- PTY failure diagnostics are owned by a platform-neutral package. Manager and
  write-buffer failures therefore compile and remain visible on every target;
  Unix-only session code no longer owns the shared hook or event contract.
- Folder browsing deliberately accepts cleaned absolute paths because selecting
  an arbitrary working directory is the feature contract; relative traversal
  remains rejected before filesystem access.
- Terminal UAT preserves literal JSON booleans, including `false`, when validating a stopped session.
- The terminal iframe accepts control messages only from its actual parent window; same-shaped messages from sibling or unrelated frames are ignored before target dispatch.
- Start failures are never silently dropped: `startSession` classifies each failure (`unreachable` transport / `unavailable` reachable-500 / `sandbox`), and the side panel surfaces `unreachable`/`sandbox` even with no panel body mounted (via toast at daemon-down-at-open), while `unavailable` falls through to the recoverable no-session state. The daemon also logs state-mutating failures (`terminal_session_start_failed`, `terminal_session_stop_failed`) to `~/.kaboom/logs/kaboom.jsonl`.
- Any legacy or fallback terminal shell that still mounts from content-script code now uses `Kaboom Terminal` so mixed-brand terminal chrome does not reappear.
- Annotation auto-send now uses a typing-aware write queue: if the user is active in terminal, writes wait until ~1.5s idle
- Annotation→terminal writes are delivery-verified: `terminal_panel_write` is acked by the side-panel document (the background never replies to this type), so the bridge tells a delivered write from one that vanished. A missing ack surfaces a toast (fail-loud) and reconciles the stale `TERMINAL_UI_STATE` visibility mirror to `false` (rule 18) so the gate stops firing at a panel that was closed with Chrome's own X.
- Terminal write attempts carry the bridge generation from dispatch through
  promise settlement, so a late response after teardown cannot schedule a stale
  retry into a newly opened panel session.
- Queued submit is reconnect-safe: if WS drops before Enter, submit waits until connection is back
- Write-guard escape hatch: the genuine wedge — a socket that stays DOWN (`!terminalConnected`) — is bounded by `TERMINAL_GUARD_MAX_WAIT_MS` (derived from the iframe reconnect schedule, see below); the poller gives up LOUDLY (error toast + `resetWriteGuardState`). The typing-defer branch is self-limiting and reachable, so it resets `guardBlockedSince` and never trips the hatch — continuous typing no longer drops a healthy write with a false "terminal not reachable". Momentary blips still queue-and-flush within the window. Queue backlog is bounded (`MAX_QUEUED_WRITES`), and an overflow drop is logged (not silent).
- Submit re-guard ordering is tested at the canonical write-guard owner with a
  controlled clock: focus returning before delayed Enter suppresses submission,
  and blur releases it on the next poll. The former multi-second side-panel
  duplicate used real timers and was removed so sharded load cannot invert the
  test's intended event order.
- **Dead-session self-heal:** nothing removed a PTY session whose child exited on its own, so the next Start returned `ErrSessionExists` (409+old token) and the client reconnected onto a dead fanout → immediate `exited`, wedging the terminal forever. `Manager.Start` now evicts a session that is no longer `IsAlive` and spawns fresh (`StartResult.Replaced`, `terminal_session_healed` log); the handler drops the stale relay.
- **Slow-drop ≠ exit:** a subscriber dropped for backpressure (big build, backgrounded tab) is no longer reported to the browser as `exited`. `Relay.ended` (set before the deferred `fanout.Close`) distinguishes a genuine end from a fanout drop; a drop closes the connection so the browser reconnects+replays instead of showing a dead terminal.
- **Full-daemon-restart client recovery:** the iframe caps consecutive failed reconnects (`MAX_RECONNECT_ATTEMPTS`) and, on exhaustion, signals the parent `reconnect_exhausted` instead of looping forever on a dead token; the parent runs `redrawTerminal` (validate-then-rebuild) into a fresh session. Keystrokes typed during a reconnect gap are buffered (bounded) and flushed on `replay_end` rather than dropped.
- **The write queue is bounded by bytes, not just entries:** `MAX_QUEUED_WRITES` (200) bounded nothing that matters — 200 one-megabyte writes was a legal state, ~200 MB pinned in the side panel. `enqueueBoundedWrite` now also enforces `MAX_QUEUED_WRITE_BYTES` (1 MB, mirroring the daemon's PTY write-buffer cap) in UTF-8 bytes, evicting oldest-first down to empty so an oversized single write cannot lodge in the queue. Both drop paths warn (rule 25). The helper moved from `sidepanel.ts` into `terminal-write-guard.ts`, which owns the rest of the queue lifecycle, so the bound cannot be bypassed by a second enqueue site (rule 19).
- **Reconnect-gap input buffer is correctly bounded:** eviction used `while (total > MAX && pendingInput.length > 1)`, so a single paste larger than the 8192 cap could never be evicted — it sat in the buffer for the whole outage and was replayed whole. It also counted UTF-16 code units rather than the UTF-8 bytes actually sent (a `€` is 1 vs 3), and re-summed the entire queue on every keystroke. Entries are now `{text, bytes}` with a running `pendingInputBytes` total, eviction runs down to empty, and a drop logs `[KaBOOM! terminal] dropped N byte(s)…` instead of being silent. An oversized chunk is dropped rather than truncated on purpose: half a pasted command, then Enter, is worse than no paste.
- **Reconnect backoff is jittered:** a daemon restart drops every open panel at the same instant, and the unjittered schedule sent them all back at the same instants — straight into the fanout's 32-subscriber cap, where the rejected ones retried in lockstep again. `terminal.html` now jitters each wait by up to `RECONNECT_JITTER_RATIO` (25%), additive only so the derived write-guard budget stays an upper bound (`terminalReconnectExhaustionMs()` multiplies by the same ratio, and the contract test pins the two ratios together).
- **The terminal port is discovered, not assumed:** the browser derived it as `main_port + 1` and never read `terminal_port` from `/health`, which is what the daemon actually publishes after binding (it logs `terminal_server_bind_failed` when base+1 is taken). Every request then went to a port nothing was listening on and the terminal just looked broken. `resolveTerminalServerUrl()` performs the discovery inside the helper (rule 19 — no caller can forget it), caches it per base URL with a 60s TTL, and falls back to base+1 on any failure (daemon down, non-OK, no `terminal_port`, Windows), so it can only improve on the old assumption. The synchronous `getTerminalServerUrl()` remains for call sites that cannot await (postMessage target origin, `createPanelShell`) and reads the same cache.
- **Write-guard budget is derived, not guessed:** `TERMINAL_GUARD_MAX_WAIT_MS` was a hand-picked 30s while the iframe does not emit `reconnect_exhausted` until ~45s (delays 1,2,4,8,10,10,10), so the guard discarded queued agent writes 15s *before* the parent's recovery even began — the queue could never survive the outage it exists for. The schedule is now declared in `terminal-widget-types.ts` (`TERMINAL_RECONNECT_BASE_DELAY_MS`, `_MAX_DELAY_MS`, `TERMINAL_MAX_RECONNECT_ATTEMPTS`) and the budget is computed from it (`terminalReconnectExhaustionMs()` + `TERMINAL_GUARD_RECOVERY_GRACE_MS`). Terminal port discovery is owned by `src/lib/terminal-server.ts`, so background, content, and side-panel callers share it without crossing runtime-context boundaries. `terminal-reconnect-budget-contract.test.js` pins those declarations to terminal.html's own literals so the two cannot drift.
- **`Relay.Close` actually tears the relay down:** it used to close only the write buffer, so readLoop stayed blocked in `sess.Read` — the goroutine, the PTY fd and the open fanout all survived `/terminal/stop` and daemon shutdown. Close now closes the session (which is what breaks readLoop out), waits up to `RelayCloseTimeout` (5s) for the teardown defers, and re-closes the write buffer as defense in depth; every step is idempotent. Relays also evict themselves from the `Map` when their session ends on its own (`onExit` → `removeIfCurrent`, which never evicts a replacement), so an ordinary `exit` no longer leaks a map entry bound to a dead fanout.
- **Duplicate fanout ids are rejected, not swallowed:** `Fanout.Subscribe` overwrote an existing id's map entry without closing the old channel, orphaning that subscriber's goroutine on a channel nothing would write to or close (and the incumbent's `Unsubscribe` then closed the *new* subscriber's channel). The guard lived in one caller (`WaitForPromptViaRelay`'s unique ids); per rule 19 it now lives in the primitive — a duplicate id returns `fanout.ErrDuplicateSubscriber` and the incumbent is untouched.
- **"Terminal gone" ≠ "terminal behind":** `WriteBuffer.Write` returned `ErrWriteBufferFull` both when the buffer overflowed the backpressure cap AND when it was closed, so no caller could tell a session that ended from one that is wedged. Writing to a closed buffer now returns `pty.ErrWriteBufferClosed`. The WS upstream loop no longer discards the error either — a refused keystroke frame logs `terminal_input_dropped` with `reason: session_ended | backpressure | write_error` (`writeDropReason`).
- **Idle timer cannot outlive the session:** `AppendScrollback` never checked whether the session was closed, so a chunk landing after `Close` armed a fresh 30s timer nothing would stop, and `Close`'s `idleTimer.Stop()` could not un-fire a timer whose deadline had already passed. Either way the callback logged "session X is idle" for a shell that had already exited. `Session` now carries an `idleStopped` flag (under `scrollMu`, so the idle path never has to take `mu` and invert Close's lock order): `AppendScrollback` refuses to re-arm, and the callback (`fireIdle`) re-reads the flag at fire time.
- **PTY I/O shutdown is deterministic:** `Session` depends on the minimal
  read/write/close/fd PTY boundary implemented by `os.File` in production.
  Tests use a blocking fake to prove in-flight and concurrent reads/writes have
  entered I/O before `Close` releases them as `ErrSessionClosed`; no scheduling
  sleep is needed. Idle reset/disable behavior lives exclusively with the
  injected fake-clock suite rather than duplicate real-timer tests.
- **Bounded reap in the relay:** `Relay.reapExitCode` called `sess.Wait()`, which blocked on the reaper channel with no timeout. It runs *before* readLoop's deferred `fanout.Close()`, so an unreapable child parked readLoop forever — the fanout never closed and every WebSocket pump hung on a channel that would never close. `Session.Wait(timeout)` now mirrors `Close`'s bound (`terminal.ReapTimeout`, 2s), returns `pty.ErrReapTimeout` and logs `pty_session_reap_timeout` with `phase: wait`; teardown then proceeds with exit code -1 (unknown).
- **No re-lookup after Start:** `HandleTerminalStart` used to do `sess, _ := mgr.Get(result.SessionID)` and then dereference `sess` (`SetIdleConfig`, `NewRelay`). A `/terminal/stop` landing in that window removed the session from the map, so the swallowed error became a nil-pointer panic in the handler. `pty.StartResult` now carries the spawned `Session`, so there is no second lookup and no window.
- **PTY failures are no longer silent (rule 25):** signalling, closing and flushing were all discarded with `_ =`. `internal/pty` now has one structured sink (`pty.SetDiagnosticHook`, wired to the daemon log in `RegisterRoutes`) and emits `pty_session_signal_failed`, `pty_session_reap_timeout` (child survived SIGKILL + both bounded waits), `pty_session_close_failed` (Start/StopAll discarded these) and `pty_writebuffer_write_failed` (stranded stdin bytes, with the undelivered byte count). Control flow is unchanged; an already-exited child (`os.ErrProcessDone`) is still treated as the expected case and is not logged.
  Close-failure coverage injects a minimal PTY whose `Close` deterministically
  fails; nil test PTYs represent an intentionally absent descriptor and are not
  misused as an error fixture.
- **Start never freezes the manager:** `Manager.Start` used to `Close()` the session it evicts (self-heal) while holding `m.mu`. A child that is not `IsAlive` can still be unreapable, so that Close runs the full SIGTERM→2s→SIGKILL→2s escalation — freezing `Get`/`GetByToken`/`List` and therefore every terminal route for ~4s. All bookkeeping now happens in `startAndRegister` under the lock; every Close (the evicted corpse, and the fresh session on a token-generation failure) runs after the lock is released, matching `Stop`/`StopAll`.
- **Bounded shutdown:** daemon teardown can no longer hang. `WriteBuffer.Close` is time-bounded (a drain blocked in `ptmx.Write` can't wait forever), shutdown order is `StopAll` (close PTYs) → `CloseAll` (drain write buffers) so the blocked write unblocks, and `Session.Close`'s post-SIGKILL reap wait is bounded.
- WebSocket frame writes are serialized per-connection and bounded by `WSWriteTimeout`: a stalled reader (backgrounded-tab zero-window, hostile client) can no longer block the downstream pump or ping keepalive for up to `PongTimeout`.
- Per-connection WS goroutines (downstream pump, ping keepalive, upstream reader) are panic-recovered via `goConnWorker`: a fault tears down only that connection (structured `terminal_ws_panic` log + `closeConn`), never the daemon process. `WriteBuffer.drain` and the init goroutine are `util.SafeGo`-wrapped for the same invariant.
- Terminal lifecycle tests use owner-specific completion signals: child reaping
  through `Session.Wait`, a private relay-subscription callback, supervisor event
  generations, and one-shot panic-log delivery. None poll process, subscriber,
  or logging state with scheduler sleeps.
- Single-instance election never kills a healthy same-version daemon: the install hook (`kill-daemon.js`) and the in-process election both retry `/health` within a budget before concluding "down" (a momentary hiccup won't re-trigger a restart storm), and a future-dated lock (clock skew) is treated as brand-new (defer).
- Upload paths sanitize the session id to a single segment (`sanitizeSessionID`) so a `../` id can't escape the uploads directory.
- Scrollback buffer capped at 256 KB for memory safety
- PTY session tests share a bounded `readUntilContains` helper to keep echo/size assertions consistent

---

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

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
- **Auto-restart**: the terminal supervisor reclaims the port and rebinds with exponential backoff (500ms → 30s, up to 8 attempts). On success it logs `terminal_server_restarted` and restores `terminal_port`; if all attempts fail it logs `terminal_server_restart_giveup` and leaves the terminal unavailable until a daemon restart. The supervisor never restarts during graceful daemon shutdown (`Supervisor.Shutdown` stops the loop and closes the current server).

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

The root PTY package is exactly ten files and owns process/session management,
platform PTY opening, and buffered input. Subscriber broadcast is an independent
`internal/pty/fanout` package because it changes with relay backpressure rather
than process lifecycle. Runtime clock, diagnostic, write-buffer, and reaper
tests share one lifecycle suite; manager spawn/self-heal tests share the manager
suite. Package regression coverage prevents the ten-file or 800-line boundaries
from regressing.

Relay completion is a strict teardown barrier: the write buffer and fanout
close first, map self-removal runs next, and only then does the relay completion
channel close. Tests and callers can therefore assert session-end postconditions
without polling scheduler time; WebSocket replay completion likewise proves the
downstream subscription is ready before a test stops its session.
Manager self-heal tests likewise synchronize on the evicted session's `done`
transition and the child reaper's completion, proving replacement publication
and dead-process recovery without polling manager or process state.

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

### Sandbox Detection

If the daemon was spawned by an MCP client's stdio transport, macOS sandbox restrictions may prevent `posix_spawn`/`fork`. The `handleTerminalStart` handler detects this and returns HTTP 503 with a `sandbox_restricted` error, which the side panel displays as an actionable inline error with the command to restart the daemon with full permissions.

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
| `cmd/browser-agent/internal/terminal/supervisor/supervisor.go` | Restart/backoff supervision and graceful shutdown behind explicit host callbacks |
| `cmd/browser-agent/main_connection_mcp.go` | Terminal server startup, supervision, root adapters, and graceful shutdown |
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
| `internal/pty/writebuf.go` | Bounded, close-aware PTY input buffering |
| `internal/pty/fanout/fanout.go` | Subscriber isolation, caps, and slow-consumer backpressure |
