---
doc_type: product-spec
feature_id: feature-lazy-server-start
status: proposed
feature: lazy-server-start
owners: []
created: 2026-06-29
updated: 2026-06-29
last_reviewed: 2026-07-05
links:
  index: ./index.md
  tech: ./tech-spec.md
  qa: ./qa-plan.md
---

# Lazy Server Start (Product Spec)

> Let users track tabs and configure the extension at any time, regardless of whether the Kaboom daemon is running. The daemon starts automatically the first time an artificial intelligence (AI) coding tool issues a tool call.

## Problem

Kaboom has two cooperating processes: a Chrome extension that captures browser telemetry, and a Model Context Protocol (MCP) daemon that AI tools talk to over HyperText Transfer Protocol (HTTP). In earlier designs the user had to start the daemon manually before anything worked. This created two recurring failures:

1. **Dead-on-arrival popup.** When the daemon was not running, the extension popup showed a red error and disabled the "Track This Tab" button. A user who simply wanted to mark a tab for capture was blocked by an unrelated server-lifecycle concern.

2. **Startup races and cold-start latency.** MCP clients (Claude Code, Cursor, and similar) launch the Kaboom binary in stdio bridge mode for every session. If each launch tried to bind the port synchronously, concurrent clients raced each other, and the `initialize` handshake stalled while a daemon booted.

The user should never have to think about whether a background server is running. Binding pages is a browser concern; serving MCP is a tooling concern; the two must not be coupled.

## Solution

Adopt a **lazy server start** model with four contracts:

- **Tab tracking is always available.** The popup reads and writes tracking state from `chrome.storage.local` and synchronizes it to the daemon asynchronously through a one-second `/sync` heartbeat. When the daemon is offline the popup shows a calm amber "Offline" indicator, never a blocking error, and every control stays usable.

- **Tool calls auto-start the daemon.** The binary runs in bridge mode (stdio transport for the MCP protocol). The bridge answers `initialize` and `tools/list` immediately from a fast path, spawns the daemon asynchronously in the background, and only makes `tools/call` wait for readiness.

- **The daemon recovers from failure.** If the daemon dies mid-session, the next forwarded request detects the broken connection, re-launches the daemon, and retries the call with a fresh timeout. A persistent failure returns a structured error marked `retryable`.

- **The extension reconnects automatically.** When the daemon starts or restarts, the extension's sync client reconnects on its next one-second poll, the badge clears, and tracked-tab state is resent.

## User Stories

- As a developer, I want to mark a tab for capture before I start any AI session, so that telemetry is ready the moment the daemon comes up.
- As an AI coding agent, I want my first MCP `initialize` and `tools/list` calls to return instantly, so that the session handshake is never blocked by a cold daemon boot.
- As a developer running several MCP clients at once, I want them to coordinate rather than race for the port, so that exactly one daemon owns the state directory.
- As a developer, I want a daemon crash to self-heal on the next tool call, so that one transient failure does not end my session.
- As a developer in an interactive shell, I want a clear warning when my launch is likely transient, so that I can choose to start the daemon persistently instead.

## Behavior Contracts

### 1. Tab tracking is always available

The popup's "Track This Tab" button works independently of daemon connectivity. While the daemon is offline:

- The popup shows an amber "Offline" indicator, not a red error.
- The troubleshooting section is informational and collapsed, not alarming.
- Tracking, recording, and all popup controls remain fully functional.
- The sync client retries every one second until the daemon comes up.

### 2. Tool calls auto-start the daemon

1. The binary starts in bridge mode (stdio transport for the MCP protocol).
2. The bridge checks whether a compatible daemon already serves the configured port.
3. If none is found, the bridge spawns one asynchronously through a startup coordinator.
4. `initialize` and `tools/list` respond immediately from the fast path; no daemon is required.
5. `tools/call` waits for the daemon to become ready, up to a short grace period.
6. Once the daemon is ready, the tool call is proxied over HTTP.

### 3. Daemon recovery on failure

1. The bridge detects a connection error on the next forwarded request.
2. The bridge attempts to re-launch the daemon.
3. The tool call is retried with a fresh timeout after respawn.
4. If respawn fails, a structured error with `retryable: true` is returned.

### 4. Extension reconnection

1. The sync client polls `/sync` every one second with no exponential backoff on reconnect.
2. On the first successful sync, the connection-change callback fires with `true`.
3. The badge updates from the warning glyph back to normal.
4. Tracked-tab state is sent on the reconnection sync.
5. The extension-readiness guard waits up to its configured timeout for extension connectivity before failing a tool call.

## UX Contract

| Daemon State | Popup Status | Status Color | Track Button | Troubleshooting |
|---|---|---|---|---|
| Running and connected | "Connected" | Green | Enabled | Hidden |
| Not running | "Offline" | Amber | Enabled | Informational (collapsed) |
| Starting | "Offline" | Amber | Enabled | Informational (collapsed) |

The popup must never show a blocking error or disable the Track button because of daemon state.

## Launch-Mode Classification

The bridge classifies how it was launched so it can warn when a session is likely to disconnect:

- **Persistent.** The `--daemon` flag is set, a supervisor was detected (systemd, launchd, Cloud Run, or container markers), or the process was started non-interactively over stdio. No warning.
- **Likely transient.** The process was started interactively from an ad-hoc shell (`bash`, `zsh`, `npm`, `npx`, and similar) without the `--daemon` flag. Kaboom emits a `launch_mode_warning` suggesting a persistent start.

When `KABOOM_REQUIRE_PERSISTENT` is enabled, a likely-transient launch is rejected with an actionable error instead of a warning.

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | The popup Track button and recording controls function while the daemon is offline | must |
| R2 | Offline daemon state renders as amber "Offline", never a blocking red error | must |
| R3 | `initialize`, `initialized`, and `tools/list` respond from the bridge fast path without a daemon | must |
| R4 | A missing daemon is spawned asynchronously without blocking the stdio read loop | must |
| R5 | Concurrent bridges coordinate startup so exactly one daemon owns the state directory | must |
| R6 | `tools/call` waits during the startup grace period rather than returning an instant error | must |
| R7 | A dead daemon is respawned on the next forwarded request and the call is retried | must |
| R8 | Persistent failures return a structured error with `retryable: true` | must |
| R9 | The extension sync client reconnects within one poll interval after the daemon restarts | must |
| R10 | The extension-readiness guard waits for cold-start reconnection before failing | should |
| R11 | Launch-mode classification emits a warning for likely-transient interactive launches | should |
| R12 | `KABOOM_REQUIRE_PERSISTENT` converts the transient warning into a hard error | should |

## Non-Goals

- This feature does NOT change the MCP tool surface (the five tools are unchanged).
- This feature does NOT add a user-facing "start server" button; starting is implicit.
- This feature does NOT manage daemon shutdown policy beyond singleton lock cleanup.
- Out of scope: multi-machine or remote daemons. The daemon is always local on `127.0.0.1`.
- Out of scope: persisting tracked-tab state to the daemon's disk; the extension remains the source of truth until sync succeeds.

## Performance SLOs

| Metric | Target | Rationale |
|--------|--------|-----------|
| `initialize` / `tools/list` response (cold) | < 50ms | Fast path serves these without a daemon |
| `tools/call` wait during cold start | Bounded by grace period (~2s) | Must wait, not error, while the daemon boots |
| Extension reconnect after daemon restart | <= 1 poll interval (~1s) | Sync client polls every second |
| Peer-bridge startup coordination overhead | < 250ms typical | Followers wait briefly for the leader's daemon |

## Security and Privacy

- All traffic stays on `127.0.0.1`; the daemon never binds a public interface.
- The daemon lock file records only a process identifier, port, state directory, and version; no telemetry or page content.
- Takeover never kills a process blindly: the lock process identifier must match the port's process-identifier file, and a non-Kaboom service on the port is reported rather than terminated.
- Tracked-tab state held in `chrome.storage.local` never leaves the machine; it syncs only to the local daemon.

## Edge Cases

- **Port occupied by a non-Kaboom service.** The bridge marks the state fatally failed and reports the occupying service rather than retrying or killing it.
- **Incompatible daemon version on the port.** The bridge stops the old daemon for upgrade and spawns a compatible one; if recycling fails it reports a fatal, non-retryable error.
- **Two bridges spawn at once.** A startup lock elects one leader; followers wait for the leader's daemon and only reclaim the lock if the leader stalls.
- **Stale lock from a crashed leader.** A lock older than the stale threshold is cleared so a new leader can take over.
- **Daemon dies between tool calls.** The next call respawns it; if the call method is `tools/call`, it waits and retries rather than failing fast.
- **Extension never connects.** The readiness guard times out and returns a structured, retryable error explaining that the extension is required.

## Dependencies

- **Depends on:** the MCP stdio bridge transport, the daemon lock-file lifecycle, and the extension `/sync` heartbeat.
- **Depended on by:** every MCP tool call (all tools route through the bridge) and the extension popup status display.
