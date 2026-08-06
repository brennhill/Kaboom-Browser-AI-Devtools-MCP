---
doc_type: feature_index
feature_id: feature-lazy-server-start
status: implemented
feature_type: feature
owners: []
last_reviewed: 2026-08-06
code_paths:
  - cmd/browser-agent/internal/bridge/bridge_startup.go
  - cmd/browser-agent/internal/bridge/bridge_startup_state.go
  - cmd/browser-agent/internal/bridge/healthprobe/probe.go
  - cmd/browser-agent/internal/bridge/daemoncmd/command.go
  - cmd/browser-agent/internal/bridge/startuplock/lock.go
  - cmd/browser-agent/internal/bridge/bridge.go
  - cmd/browser-agent/internal/bridge/bridge_fastpath.go
  - cmd/browser-agent/internal/bridge/fastpathtelemetry/telemetry.go
  - cmd/browser-agent/internal/launchmode/launch_mode.go
  - cmd/browser-agent/internal/toolguard/guards.go
  - internal/identity/mcp.go
  - src/popup/tabs/tab-tracking.ts
  - src/popup/shell/status-display.ts
  - extension/popup.html
test_paths:
  - cmd/browser-agent/internal/bridge/fastpathtelemetry/telemetry_test.go
  - cmd/browser-agent/internal/toolguard/guards_test.go
  - cmd/browser-agent/internal/bridge/bridge_test_support_test.go
  - cmd/browser-agent/internal/bridge/bridge_startup_test.go
  - cmd/browser-agent/internal/bridge/startuplock/lock_test.go
  - cmd/browser-agent/internal/bridge/bridge_fastpath_unit_test.go
  - cmd/browser-agent/internal/bridge/healthprobe/probe_test.go
  - cmd/browser-agent/internal/bridge/daemoncmd/command_test.go
  - cmd/browser-agent/internal/launchmode/launch_mode_test.go
  - cmd/browser-agent/tools_coldstart_gate_test.go
  - internal/identity/mcp_test.go
---

# Lazy Server Start

## Overview

Kaboom supports a **lazy server start** model: users can bind pages (track tabs) and configure the extension at any time, regardless of whether the MCP daemon is running. The server starts automatically when an AI tool (Claude Code, Cursor, etc.) invokes its first MCP command.

Fast-path diagnostics use a bounded asynchronous queue. Request handling updates
its counters and enqueues an immutable JSON record without filesystem I/O; the
bridge drains accepted records when its stdio loop ends. A full queue drops
diagnostics instead of delaying or corrupting MCP protocol traffic.

## Contracts

### 1. Tab Tracking is Always Available

The extension popup's "Track This Tab" button works independently of server connectivity. Tab tracking state is stored locally in `chrome.storage.local` and synchronized to the server asynchronously via the `/sync` heartbeat. When the server is offline:

- The popup shows an amber "Offline" indicator (not a red error)
- The troubleshooting section is informational, not alarming
- Tab tracking, recording, and all popup controls remain fully functional
- The sync client retries every 1 second until the server comes up

### 2. Tool Calls Auto-Start the Daemon

When an MCP client (Claude Code, Cursor, etc.) invokes a tool call:

1. The binary starts in **bridge mode** (stdio transport for MCP protocol)
2. Bridge checks if a daemon is already running on the configured port
3. If no daemon found, bridge **spawns one asynchronously** via `startDaemonSpawnCoordinator`
4. `initialize` and `tools/list` respond immediately (fast-path, no daemon needed)
5. `tools/call` waits for the daemon to become ready (up to grace period)
6. Once daemon is ready, the tool call is proxied via HTTP

Bridge readiness probes recognize the daemon only through the canonical
`name` field in `/health`; historical `service` and `service-name` spellings
are rejected.

### 3. Daemon Recovery on Failure

If the daemon dies mid-session:

1. Bridge detects connection error on the next forwarded request
2. `respawnIfNeeded()` attempts to re-launch the daemon
3. The tool call is retried with a fresh timeout after respawn
4. If respawn fails, a structured error with `retryable: true` is returned

### 4. Extension Reconnection

When the daemon starts (or restarts), the extension's sync client reconnects automatically:

1. Sync client polls `/sync` every 1 second (no exponential backoff on reconnect)
2. On first successful sync, `onConnectionChange(true)` fires
3. Badge updates from "!" to normal
4. Tracked tab state is sent on the reconnection sync
5. `requireExtension` guard waits up to 5 seconds for extension connectivity

The runtime guard checks and the diagnostic state rendered in their recovery
responses are intentionally colocated in `internal/toolguard/guards.go`; changes to a
guard and its user-facing recovery context must ship together.

## UX Contract

| Server State | Popup Status | Status Color | Track Button | Troubleshooting |
|---|---|---|---|---|
| Running + Connected | "Connected" | Green | Enabled | Hidden |
| Not Running | "Offline" | Amber | Enabled | Informational (collapsed) |
| Starting | "Offline" | Amber | Enabled | Informational (collapsed) |

The popup must never show a blocking error or disable the Track button due to server state.

## Flow Map

See `docs/architecture/flow-maps/bridge-daemon-lifecycle.md` for the canonical bridge startup flow.
