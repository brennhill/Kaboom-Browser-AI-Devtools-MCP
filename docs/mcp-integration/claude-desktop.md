---
title: "Kaboom + Claude Desktop"
description: "Configure Kaboom as an MCP server for Claude Desktop. Give Claude real-time access to your browser's console logs, network errors, and DOM state."
keywords: "Claude Desktop MCP server, Claude Desktop browser errors, Claude Desktop MCP config, Claude Desktop debugging"
permalink: /mcp-integration/claude-desktop/
header:
  overlay_image: /assets/images/brand/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Fuel Claude Desktop with live browser data."
toc: true
toc_sticky: true
status: reference
last_reviewed: 2026-03-28
---

## <i class="fas fa-file-code"></i> Configuration

Edit the Claude Desktop config file:

- <i class="fab fa-apple"></i> **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- <i class="fab fa-windows"></i> **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "kaboom-browser-devtools": {
      "command": "npx",
      "args": ["kaboom-agentic-browser"]
    }
  }
}
```

## <i class="fas fa-fire-alt"></i> Usage

After restarting Claude Desktop, Claude can:

- <i class="fas fa-exclamation-triangle"></i> Read browser console errors and warnings
- <i class="fas fa-wifi"></i> Inspect failed API calls with response bodies
- <i class="fas fa-code"></i> Query the live DOM
- <i class="fas fa-plug"></i> Monitor WebSocket connections
- <i class="fas fa-universal-access"></i> Run accessibility audits

Ask Claude: _"What errors are showing in my browser?"_

## <i class="fas fa-wrench"></i> Troubleshooting

1. **Restart Claude Desktop** after editing config
2. **Check file location** — path is OS-specific
3. **Verify JSON syntax** — invalid JSON fails silently
4. **Check extension popup** — should show "Connected"
