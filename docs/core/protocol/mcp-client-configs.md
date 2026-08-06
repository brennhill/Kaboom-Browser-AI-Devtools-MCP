---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-03-28
---

# MCP Client Configuration Locations

Quick reference for updating Kaboom MCP configuration across all supported AI coding assistants.

## Configuration Files

| Client | Config Path | Format |
|--------|-------------|--------|
| Claude Desktop | `~/Library/Application Support/Claude/claude_desktop_config.json` | JSON |
| Cursor | `~/.cursor/mcp.json` | JSON |
| OpenCode | `~/.config/opencode/opencode.json` | JSON |

## Standard Configuration

All clients should use the same configuration pattern for consistency:

```json
{
  "command": "npx",
  "args": ["-y", "kaboom-agentic-browser@VERSION"]
}
```

### Claude Desktop

**File:** `~/Library/Application Support/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "kaboom-browser-devtools": {
      "command": "npx",
      "args": ["-y", "kaboom-agentic-browser@VERSION"]
    }
  }
}
```

### Cursor

**File:** `~/.cursor/mcp.json`

```json
{
  "mcpServers": {
    "kaboom-browser-devtools": {
      "command": "npx",
      "args": ["-y", "kaboom-agentic-browser@VERSION"]
    }
  }
}
```

### OpenCode

**File:** `~/.config/opencode/opencode.json`

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "kaboom-browser-devtools": {
      "type": "local",
      "command": [
        "npx",
        "-y",
        "kaboom-agentic-browser@VERSION"
      ]
    }
  }
}
```

## Quick Update Script

Update all configs to a new version:

```bash
VERSION="0.7.12"

# Claude Desktop
sed -i '' "s/kaboom-agentic-browser@[0-9.]*\"/kaboom-agentic-browser@$VERSION\"/g" \
  ~/Library/Application\ Support/Claude/claude_desktop_config.json

# Cursor
sed -i '' "s/kaboom-agentic-browser@[0-9.]*\"/kaboom-agentic-browser@$VERSION\"/g" \
  ~/.cursor/mcp.json

# OpenCode
sed -i '' "s/kaboom-agentic-browser@[0-9.]*\"/kaboom-agentic-browser@$VERSION\"/g" \
  ~/.config/opencode/opencode.json
```

## UAT Checklist

After updating configs:

1. **Restart each client** (quit and reopen)
2. **Verify MCP connection** - check for connection status/errors
3. **Test a tool call** - e.g., ask "check browser errors"
4. **Check logs for errors:**
   - Claude Desktop: `~/Library/Logs/Claude/mcp*.log`
   - Cursor: View → Output → MCP
   - OpenCode: Terminal output

## Flags Reference

| Flag | Purpose |
|------|---------|
| `-y` | Auto-confirm npx package installation |

## Troubleshooting

**"Unexpected end of JSON input"**
- Upgrade to v5.5.0+ (fixed double-newline bug)

**Connection timeout**
- Restart the MCP client after config changes
- Verify Kaboom path/version is current (`kaboom-agentic-browser --version`)

**Tools not appearing**
- Verify browser extension shows "Connected"
- Check extension is on correct tab
