---
title: Downloads
description: Download the Kaboom extension and tools for your platform
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['downloads']
---

<!-- markdownlint-disable MD033 -->
<img src="/images/sparky-working-laptop-web.webp" alt="Sparky coding at laptop" style="max-width:50%;height:auto;" />
<!-- markdownlint-enable MD033 -->

## One-Liner Install (Recommended)

One command downloads the binary, stages the Chrome extension, and auto-configures all detected AI tools.

**macOS / Linux:**
```bash
curl -sSL https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/install.ps1 | iex
```

**What this does:**
- Downloads the platform binary to `~/.kaboom/bin/`
- Verifies SHA-256 checksum
- Extracts the Chrome extension to `~/KaboomAgenticDevtoolExtension/`
- Runs `--install` which auto-detects and configures: Claude Code, Claude Desktop, Cursor, Windsurf, VS Code, Gemini CLI, OpenCode, Antigravity, Zed

After running the installer, load the extension in Chrome:
1. Open `chrome://extensions/`
2. Enable **Developer mode** (toggle in top right)
3. Click **Load unpacked**
4. Select **`~/KaboomAgenticDevtoolExtension`**

### Chrome Extension

The extension captures browser telemetry and sends it to the local Kaboom server.

#### What's New in 0.8.x

- **Annotation System Upgrade** — Draw mode now pairs with review/export flows (`annotations`, `annotation_report`, `annotation_issues`) for multi-page feedback sessions.
- **Tracked Hover Island** — A floating quick-action island on tracked tabs for one-click annotate, screenshot, terminal, and recording controls.
- **Side-Panel Terminal** — Open the dedicated side panel terminal from the hover island and run live commands without page-embedded xterm.
- **Recording Approval UX** — Clear pending/approved recording state in the popup, with explicit approve/deny flow and improved recording status feedback.
- **Context Menu Consistency** — Dynamic start/stop labels for recording, annotations, and control states so extension actions always match runtime state.
- **Protocol & Reliability Hardening** — Sync-path cleanup and stronger daemon/extension consistency for fewer tool-mode and state drift issues.

#### What's New in 0.7.x

- **5th Tool: analyze** — Active analysis with 27 modes (DOM queries, accessibility, security audits, link health, visual annotations, API validation, forms, visual diff, and more)
- **Link Health & Validation** — Browser-based link checker with CORS detection and SSRF-safe server-side validation
- **Draw Mode & Visual Annotations** — Draw rectangles and type feedback directly on the page with multi-page sessions
- **Test Healing & Classification** — Self-healing Playwright selectors and context-aware test generation
- **Recording & Playback** — Full tab video recording with audio capture and log diff comparison
- **Terminal Integration** — In-browser terminal with WebSocket relay and PTY session management
- **Multi-Client Support** — Multiple AI tools can connect to the same daemon

## Alternative Install Methods

### From Source

```bash
git clone https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP.git
cd Kaboom-Browser-AI-Devtools-MCP
make build
./dist/kaboom-agentic-browser-darwin-arm64  # or your platform binary
```

**Available Binaries:**

- `kaboom-agentic-browser-darwin-arm64` (macOS Apple Silicon)
- `kaboom-agentic-browser-darwin-x64` (macOS Intel)
- `kaboom-agentic-browser-linux-arm64` (Linux ARM64)
- `kaboom-agentic-browser-linux-x64` (Linux x86-64)
- `kaboom-agentic-browser-win32-x64.exe` (Windows x86-64)

## System Requirements

- **Browser:** Chrome/Chromium 120+ (for MV3 support)
- **Runtime:** Native Go binary (no Node.js required for standalone binary installs)
- **Node.js:** 18+ (optional, only if you install via npm)
- **Platform:** macOS, Linux, Windows
- **MCP Client:** Claude Code, OpenAI Codex, Cursor, Windsurf, Claude Desktop, Zed, Gemini CLI, OpenCode, Antigravity, or any other MCP-compliant system/agent

## Verification

To verify the extension installed correctly:

1. Open any webpage
2. Click the **Kaboom icon** (<img src="/images/logo.png" alt="Kaboom" style="display: inline; width: 20px; height: 20px; vertical-align: middle; margin: 0; padding: 0;" />) in your toolbar
3. You should see the popup with recording and tracking options
4. Check the extension's popup shows "Connected" status

To verify the binary:
```bash
~/.kaboom/bin/kaboom-agentic-browser --doctor
```

## Troubleshooting

**Extension not appearing?**
- Try refreshing the page (Cmd+R / Ctrl+R)
- Ensure Chrome is version 120 or higher
- Check that Developer mode is enabled

**Recording not working?**
- Click the Kaboom icon and approve the recording request
- Ensure the tab you want to record is the tracked tab
- Check your Chrome permissions for microphone access (if recording audio)

**Issues with MCP integration?**
- See [Claude Code Integration Guide](/mcp-integration/claude-code)
- See [OpenAI Codex Integration Guide](/mcp-integration/codex)
- See [Cursor Integration Guide](/mcp-integration/cursor)

## Support

- [Documentation](/getting-started)
- [Report Issues](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/issues)
- [GitHub Discussions](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/discussions)
- [Security Policy](/security)

## Release Notes

See [GitHub Releases](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/releases) for complete version history.
