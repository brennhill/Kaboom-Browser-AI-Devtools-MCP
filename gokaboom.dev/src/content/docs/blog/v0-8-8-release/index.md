---
title: "KaBOOM v0.8.8 Released"
description: "Terminal and daemon reliability: the in-browser terminal now survives full daemon restarts, single-instance takeover is bulletproof, and every shutdown and reconnect path is bounded."
date: 2026-07-26
authors: [brennhill]
tags: [release]
last_verified_version: 0.8.8
last_verified_date: 2026-07-26
normalized_tags: ['release', 'blog', 'v0']
---

## What's New in v0.8.8

Kaboom v0.8.8 is a reliability release focused on the in-browser terminal and the
local daemon. Three rounds of hardening make the terminal recover cleanly from a
daemon restart, reconnect without false "session ended" errors, and keep the daemon
healthy when a connection stalls or flaps.

### Highlights

- **The terminal survives a full daemon restart.** A restarted daemon drops every
  session and token; the terminal now self-heals the dead PTY session, re-tokenizes,
  and replays scrollback instead of sitting on a silent, dead connection.

- **Bulletproof single-instance daemon.** A new install defers to a healthy
  same-version daemon and takes over only a stalled or older one — so you never kill
  a working server, and never end up with two.

- **No more false "terminal exited."** A full subscriber fan-out (or a slow reader)
  is no longer misreported as a dead shell, so the client keeps reconnecting on a
  perfectly healthy terminal.

- **No connection leaks.** Scrollback-replay writes are now deadline-bound, so a
  client that stalls mid-reconnect can no longer leak a goroutine and file descriptor
  per connection — the failure mode that could slowly starve the daemon.

- **Ordered input on reconnect.** Keystrokes typed while scrollback is still
  replaying are buffered and flushed in order, so nothing races ahead of the restored
  shell state.

- **Bounded everything on shutdown.** Every shutdown wait, WebSocket write, and child
  reaper is time-bounded and panic-recovered, and the terminal server auto-restarts
  if it dies unexpectedly.

### Security

- Request-body size caps on the terminal inject and intent endpoints.
- Path-traversal hardening on the upload path.

### Quality Gates

- `make test` (Go server tests, including `-race` on the terminal and PTY packages)
- `npm run test:ext` (extension suite, with Node 20 parity)

Both passed for the `v0.8.8` release cut.

### Upgrade

```bash
curl -sSL https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/install.sh | bash
```

The installer defers to a healthy same-version daemon and cleanly takes over an older
one, so upgrading is safe to run while Kaboom is active. After upgrading, reload the
browser extension (`chrome://extensions` → reload) so it matches the new daemon.

### Full Changelog

[View v0.8.8 on GitHub](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/releases/tag/v0.8.8)
