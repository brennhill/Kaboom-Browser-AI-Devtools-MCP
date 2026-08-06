---
title: "Fire It Up"
description: "Install and configure Kaboom in under 2 minutes. Start streaming browser logs to your autonomous coding agent with a single command."
keywords: "install kaboom, kaboom agentic browser setup, browser extension install, MCP server setup"
permalink: /getting-started/
header:
  overlay_image: /assets/images/brand/hero-banner.png
  overlay_filter: 0.85
  excerpt: "One command. Two minutes. Your AI sees your browser."
toc: true
toc_sticky: true
status: reference
last_reviewed: 2026-03-02
---

## <i class="fas fa-fire"></i> 1. Ignite the Server & Configure Clients

<img src="/assets/images/sparky/features/sparky-fight-fire-web.webp" alt="Sparky firing up the server" style="float: right; width: 140px; margin: 0 0 20px 20px; border-radius: 6px;" />

**The Quickest Way (One-liner)**

**macOS / Linux:**
```bash
curl -sSL https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/setup/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/setup/install.ps1 | iex
```

This script automatically:
1.  **Downloads** the latest stable `kaboom-agentic-browser` binary for your OS and architecture.
2.  **Installs** the browser extension files to `~/KaboomAgenticDevtoolExtension`.
3.  **Auto-configures** all detected MCP clients (Claude Code, Cursor, Windsurf, Zed, etc.).

---

## <i class="fas fa-puzzle-piece"></i> 2. Install the Extension

<img src="/assets/images/sparky/features/sparky-wave-web.webp" alt="Sparky waving from the toolbar" style="float: left; width: 140px; margin: 0 20px 20px 0; border-radius: 6px;" />

Since you've already downloaded the extension files with the script above, you just need to load them into Chrome:

1.  Open `chrome://extensions`
2.  Enable **Developer mode** (top right)
3.  Click **Load unpacked**
4.  Select the folder: `~/KaboomAgenticDevtoolExtension`

Click the Kaboom Agentic Browser icon in your toolbar — it should show **Connected**.

## <i class="fas fa-plug"></i> 3. Verify Your AI Tool

The install script has already added Kaboom to your MCP configuration. Just **restart your AI tool** (Claude Code, Cursor, etc.) and the server will ignite automatically.

<i class="fas fa-fire-alt"></i> See [MCP Integration](/mcp-integration/) for manual setup if needed.

### Launch Mode Guard (Persistent vs Transient)

Kaboom now classifies launch context at startup:

- `persistent`: expected long-lived runtime (daemon flag, supervisor, or non-interactive stdio).
- `likely_transient`: interactive shell launch likely to disconnect when the process exits.

If launch mode is `likely_transient`, Kaboom prints a one-time warning with remediation:

```bash
kaboom-agentic-browser --daemon --port 7890
```

To enforce this in CI/team environments, set:

```bash
KABOOM_REQUIRE_PERSISTENT=true
```

When strict mode is enabled, Kaboom exits non-zero on `likely_transient` launches.

## <i class="fas fa-check-circle"></i> Verify the Flame

Open your web app. Trigger an error:

```javascript
console.error("Kaboom test — is it live?")
```

Ask your AI: _"What browser errors do you see?"_

## <i class="fas fa-tools"></i> Available Tools

Your AI now has 5 tools covering the full debugging lifecycle:

| Tool | What it does |
|------|-------------|
| `observe` | Captured state — errors, logs, network, WebSocket, actions, Web Vitals, page, tabs, timeline, error bundles, screenshot |
| `analyze` | Active analysis — DOM inspection, performance, accessibility, security audits, error clusters, link health, annotations |
| `generate` | Artifacts — reproduction scripts, CSP, SARIF, test generation (from context, heal, classify), visual tests, annotation reports |
| `configure` | Session — persistent memory, noise filtering, health, streaming, recording, log diff |
| `interact` | Browser automation — navigate, click, type, select, execute JS, highlight, upload, draw mode, save/load state |

Each tool has sub-modes. For example, `observe` with `what: "errors"` returns console errors, while `what: "websocket_status"` returns active WebSocket connections.

See [MCP Integration](/mcp-integration/) for full tool documentation.

## Next Steps

- <i class="fas fa-sliders-h"></i> [Configure the server](/configuration/) — port, log rotation, file path
- <i class="fas fa-plug"></i> [MCP Integration](/mcp-integration/) — setup for your specific tool
- <i class="fas fa-fire-alt"></i> [All capabilities](/features/) — everything Kaboom captures
