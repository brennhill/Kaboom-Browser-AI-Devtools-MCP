---
title: Fire It Up
description: Install and configure Kaboom in under 2 minutes. Start streaming browser logs to your autonomous coding agent with a single command.
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['getting', 'started']
---

Kaboom is an open-source browser extension plus MCP server that streams real-time browser telemetry (console logs, network errors, exceptions, WebSocket events) to AI coding assistants like Claude Code, Cursor, Windsurf, and Zed. One command to install. Zero dependencies.

## 1. Install Everything

One command downloads the binary, stages the extension, and auto-configures all detected AI tools:

**macOS / Linux:**
```bash
curl -sSL https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/setup/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/setup/install.ps1 | iex
```

This automatically:
- Downloads the latest stable binary to `~/.kaboom/bin/`
- Verifies SHA-256 checksum
- Extracts the Chrome extension to `~/KaboomAgenticDevtoolExtension/`
- Auto-configures all detected MCP clients (Claude Code, Cursor, Windsurf, Zed, Gemini CLI, OpenCode, Antigravity, Claude Desktop, VS Code)

## 2. Load the Chrome Extension

This is the one step that requires human interaction — Chrome doesn't allow programmatic extension installation.

1. Open `chrome://extensions`
2. Enable **Developer mode** (top right toggle)
3. Click **Load unpacked**
4. Select the folder: **`~/KaboomAgenticDevtoolExtension`**

You'll see the Kaboom icon in your toolbar. It will show "Not Connected" until you complete step 3.

:::tip[Skip the UI clicks]
If you're willing to restart Chrome, you can pre-load the extension via CLI flag:
```bash
# macOS
open -a "Google Chrome" --args --load-extension="$HOME/KaboomAgenticDevtoolExtension"
```
This only applies to that Chrome session. For persistent installation, use the Load Unpacked flow above.
:::

## 3. Verify It Works

**Restart your AI tool** (quit and reopen Claude Code, Cursor, etc.) to activate the MCP server.

Open your web app in Chrome. Trigger a test error:

```javascript
console.error("kaboom test: is the fire lit?")
```

Ask your AI: _"What browser errors do you see?"_

The extension icon should now show **Connected** (green indicator).

You can also verify with the built-in doctor command:
```bash
~/.kaboom/bin/kaboom-agentic-browser --doctor
```

## What tools does Kaboom give my AI?

Your AI now has 5 tools covering the full debugging lifecycle:

| Tool | What it does |
|------|-------------|
| `observe` | Browser state — errors, logs, network, WebSocket, actions, Web Vitals, page info, recordings |
| `analyze` | Active analysis — DOM queries, accessibility audits, security audits, performance, link health, visual annotations |
| `generate` | Artifacts — Playwright tests, reproduction scripts, PR summaries, SARIF, HAR, CSP, SRI, test healing |
| `configure` | Session — noise filtering, persistent storage, recording, streaming, health |
| `interact` | Browser control — navigate, click, type, execute JS, upload, draw mode, state management |

Each tool has sub-modes. For example, `observe` with `what: "errors"` returns console errors, while `what: "websocket_status"` returns active WebSocket connections.

See [MCP Integration](/mcp-integration/) for full tool documentation.

## Alternative Install Methods

**npm** (if you prefer Node.js):
```bash
npm install -g kaboom-agentic-browser && kaboom-agentic-browser --install
```

**From source** (for development):
```bash
git clone https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP.git
cd Kaboom-Browser-AI-Devtools-MCP
go run ./cmd/browser-agent
```
Requires [Go 1.24+](https://go.dev/).

## Next Steps

- [MCP Integration](/mcp-integration/) — setup for your specific tool
- [All capabilities](/features/) — everything Kaboom captures
