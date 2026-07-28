---
title: "Troubleshooting"
description: "Fix common Kaboom issues: extension not connecting, logs not appearing, MCP mode problems, port conflicts, and debug mode usage."
keywords: "kaboom troubleshooting, extension not connecting, MCP not working, kaboom debug mode, port conflict fix"
permalink: /troubleshooting/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Flame out? Get it burning again."
toc: true
toc_sticky: true
status: reference
last_reviewed: 2026-07-28
---

## <i class="fas fa-unlink"></i> Extension Not Connecting to Server

1. **Check server is running**: Look for `[Kaboom] vX.Y.Z — HTTP on port 7890` on stderr
2. **Check extension badge**: Red `!` means disconnected, green means connected
3. **Check port conflict**: Another process may be using port 7890. Try `--port 7891`
4. **Update extension URL**: If using a different port, go to Options and change the Server URL
5. **Check browser console**: Right-click extension icon → Inspect → Console tab

## <i class="fas fa-eye-slash"></i> Logs Not Appearing

1. **Check capture level**: Popup may be set to "Errors Only" — try "All Logs"
2. **Check domain filter**: Options page may have filters excluding your domain
3. **Check log file**: `cat ~/kaboom-logs.jsonl | tail -5` to see recent entries
4. **Reload the page**: Extension injects on page load

## <i class="fas fa-robot"></i> MCP Mode Not Working

1. **Check config path**: Must be in your AI tool's config directory
2. **Restart your AI tool**: MCP servers are loaded on startup
3. **Check for JSON errors**: Invalid JSON in config will silently fail
4. **Verify with AI**: Ask _"What MCP tools do you have?"_ — Kaboom tools should appear

## <i class="fas fa-layer-group"></i> MCP Config Conflicts (User vs Project)

**Symptom**: You added `.mcp.json` to your project but Kaboom uses wrong settings (different port, old path, etc.)

**Cause**: Claude Code has multiple config levels with a precedence order:
- **User-level**: Managed via `claude mcp add --scope user` (applies globally)
- **Project-level**: `.mcp.json` in project root (applies to that project)

**User-level config takes precedence.** If both define `"kaboom-browser-devtools"`, the user-level wins silently.

**To diagnose**:
```bash
# Check if Kaboom is installed at user level
claude mcp list
```

**To fix**:
1. Remove user-level Kaboom: `claude mcp remove --scope user kaboom-browser-devtools`
2. Or update the user-level config to match your desired settings
3. Restart Claude Code to pick up changes

**Best practice**: Use project-level `.mcp.json` for Kaboom. This keeps the config with the project and avoids conflicts when working on multiple projects.

## <i class="fas fa-times-circle"></i> MCP Server Shows "Failed"

**Symptom**: `/mcp` shows Kaboom as "failed" even though the config looks correct.

**Common cause**: A stale Kaboom process is holding the port from a previous session.

**To diagnose**:
```bash
# Check if something is using the port
lsof -i :7890
```

**To fix**:
```bash
# Kill the stale process
lsof -ti :7890 | xargs kill
```

Then retry `/mcp` — Claude Code will spawn a fresh instance.

## <i class="fas fa-sync-alt"></i> MCP Disconnected — Recovery

When an AI client disconnects, the persistent daemon keeps its local HTTP server running so the extension and other clients remain connected.

**What you'll see on stderr:**
```
[Kaboom] MCP client disconnected; daemon remains available on port 7890
```

Persistence is the only runtime model; no compatibility flag is required. Use `kaboom --stop` for a clean shutdown or press Ctrl+C in a foreground session.

**To recover:** Start a new AI session. It reconnects to the existing daemon, or starts one if none is running.

**If port is still in use:** A previous Kaboom process may not have exited cleanly. Kill it:
```bash
lsof -ti :7890 | xargs kill
```

## <i class="fas fa-plug"></i> Changing the Server Port

If port 7890 is in use:

```bash
npx kaboom-agentic-browser --port 7891
```

Then update the extension:

1. Click the Kaboom extension icon
2. Click "Options"
3. Change **Server URL** to `http://localhost:7891`
4. Click "Save Options"

## <i class="fas fa-check-circle"></i> Verifying Your Setup

Run the built-in setup check:
```bash
npx kaboom-agentic-browser --doctor
```

This verifies port availability, log file directory, and prints next steps. You can also test from the extension: go to **Options** and click the **Test** button next to the Server URL.

## <i class="fas fa-exclamation-triangle"></i> Version Mismatch Warning

**Symptom**: Extension popup shows a yellow "Version mismatch" banner.

**Cause**: The extension and server have different major versions (e.g., extension v4.x, server v5.x).

**To fix**:
1. Update the server: `npx kaboom-agentic-browser@latest`
2. Update the extension: pull latest from GitHub and reload in chrome://extensions
3. Both should show the same major version

Minor/patch version differences are fine and won't trigger the warning.

## <i class="fas fa-bug"></i> Using Debug Mode

The extension has built-in debug logging for troubleshooting:

1. **Enable**: Open popup → scroll to "Debugging" → toggle "Debug Mode" on
2. **View logs**: With debug mode on, activity appears in the browser console
3. **Export**: Click "Export Debug Log" to download a JSON file

### Debug Log Categories

| Category | What it captures |
|----------|-----------------|
| `connection` | Server connection/disconnection events |
| `capture` | Log capture, filtering, processing |
| `error` | Extension internal errors |
| `lifecycle` | Extension startup/shutdown |
| `settings` | Configuration changes |

The debug buffer holds 200 entries (circular — oldest are dropped). Logs are stored even when debug mode is off, so you can export after an issue occurs.

## <i class="fas fa-flag"></i> Reporting Issues

[Open an issue](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/issues/new) with:

1. **Environment**: OS, Chrome version, Kaboom version (`window.__kaboom.version`)
2. **Steps to reproduce**
3. **Expected vs actual behavior**
4. **Extension popup screenshot**
5. **Debug log export** (JSON file from popup)
6. **Browser console errors** (right-click extension → Inspect → Console)
7. **Server output** (any errors from `npx kaboom-agentic-browser`)
