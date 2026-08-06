---
title: "Kaboom + Zed"
description: "Configure Kaboom as a context server for Zed editor. Give Zed's AI assistant access to browser console logs, network errors, and DOM state."
keywords: "Zed MCP server, Zed context server, Zed browser debugging, Zed AI browser logs"
permalink: /mcp-integration/zed/
header:
  overlay_image: /assets/images/brand/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Fuel Zed's AI with live browser data."
toc: true
toc_sticky: true
status: reference
last_reviewed: 2026-03-28
---

## <i class="fas fa-file-code"></i> Configuration

Add to `~/.config/zed/settings.json`:

```json
{
  "context_servers": {
    "kaboom-browser-devtools": {
      "command": {
        "path": "npx",
        "args": ["kaboom-agentic-browser"]
      }
    }
  }
}
```

> <i class="fas fa-info-circle"></i> Zed uses `context_servers` instead of `mcpServers`, and the command format differs slightly.

## <i class="fas fa-fire-alt"></i> Usage

After restarting Zed, the AI assistant taps into Kaboom's full MCP toolset.

## <i class="fas fa-wrench"></i> Troubleshooting

1. **Restart Zed** after editing settings
2. **Check the config key** — must be `context_servers`
3. **Verify extension popup** shows "Connected"
