---
title: "Kaboom + Windsurf"
description: "Configure Kaboom as an MCP server for Windsurf (Codeium). Give Windsurf's AI access to browser console logs, network errors, and page state."
keywords: "Windsurf MCP server, Windsurf browser debugging, Codeium MCP, Windsurf AI browser logs"
permalink: /mcp-integration/windsurf/
header:
  overlay_image: /assets/images/brand/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Fuel Windsurf's AI with live browser data."
toc: true
toc_sticky: true
status: reference
last_reviewed: 2026-03-28
---

## <i class="fas fa-file-code"></i> Configuration

Add to `~/.codeium/windsurf/mcp_config.json`:

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

After restarting Windsurf, the AI can access all browser state:

- <i class="fas fa-exclamation-triangle"></i> Console errors and warnings
- <i class="fas fa-wifi"></i> Failed network requests with response bodies
- <i class="fas fa-plug"></i> WebSocket events and connection states
- <i class="fas fa-code"></i> Live DOM queries
- <i class="fas fa-universal-access"></i> Accessibility audit results

## <i class="fas fa-wrench"></i> Troubleshooting

1. **Restart Windsurf** after adding config
2. **Check extension popup** — should show "Connected"
3. **Verify config path** — must be `~/.codeium/windsurf/mcp_config.json`
