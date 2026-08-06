---
title: "Troubleshooting"
description: "Fix common Kaboom issues: extension not connecting, MCP mode problems, port conflicts, stale processes, and debug mode."
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['troubleshooting']
---

## Extension Not Connecting to Server

1. **Check server is running**: Look for `[kaboom] vX.Y.Z — HTTP on port 7890` on stderr
2. **Check extension badge**: Red `!` means disconnected, green means connected
3. **Check port conflict**: Another process may be using port 7890. Try `--port 7891`
4. **Update extension URL**: If using a different port, go to Options and change the Server URL
5. **Check browser console**: Right-click extension icon → Inspect → Console tab

## Logs Not Appearing

1. **Check capture level**: Popup may be set to "Errors Only" — try "All Logs"
2. **Check domain filter**: Options page may have filters excluding your domain
3. **Reload the page**: Extension injects on page load — navigate or refresh

## MCP Mode Not Working

1. **Check config path**: Must be in your AI tool's config directory
2. **Restart your AI tool**: MCP servers are loaded on startup
3. **Check for JSON errors**: Invalid JSON in config will silently fail
4. **Verify with AI**: Ask _"What MCP tools do you have?"_ — Kaboom tools should appear

## MCP Config Conflicts (User vs Project)

**Symptom**: You added `.mcp.json` to your project but Kaboom uses the wrong settings (different port, old path, etc.)

**Cause**: Claude Code has multiple config levels with a precedence order:
- **User-level**: Managed via `claude mcp add --scope user` (applies globally)
- **Project-level**: `.mcp.json` in project root (applies to that project)

**User-level config takes precedence.** If both define `"kaboom"`, the user-level wins silently.

**To diagnose**:
```bash
# Check if kaboom is installed at user level
claude mcp list
```

**To fix**:
1. Remove user-level kaboom: `claude mcp remove --scope user kaboom`
2. Or update the user-level config to match your desired settings
3. Restart Claude Code to pick up changes

**Best practice**: Use project-level `.mcp.json` for Kaboom. This keeps the config with the project and avoids conflicts when working on multiple projects.

## MCP Server Shows "Failed"

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

Then retry `/mcp` — your AI tool will spawn a fresh instance.

## MCP Disconnected — Recovery

When the AI client disconnects (closes its session), Kaboom logs the disconnect and exits after a brief grace period. This is by design — it frees the port so the next AI session can spawn a fresh process.

**What you'll see on stderr:**
```
[kaboom] MCP disconnected, shutting down in 100ms (port 7890 will be freed)
[kaboom] Shutdown complete
```

**Want to keep the server running?** Use `--persist`:
```bash
kaboom-agentic-browser --persist
```
This keeps the HTTP server running after MCP disconnect so the extension stays connected between AI sessions. Press Ctrl+C to stop.

**To recover:** Simply start a new AI session. Your AI tool will spawn a fresh Kaboom process automatically. The extension reconnects to the new instance on its next poll.

**If port is still in use:** A previous Kaboom process may not have exited cleanly. Kill it:
```bash
lsof -ti :7890 | xargs kill
```

## Changing the Server Port

If port 7890 is in use:

```bash
kaboom-agentic-browser --port 7891
```

Then update the extension:

1. Click the Kaboom extension icon
2. Click "Options"
3. Change **Server URL** to `http://localhost:7891`
4. Click "Save Options"

## Verifying Your Setup

Run the built-in setup check:
```bash
kaboom-agentic-browser --doctor
```

This verifies port availability, binary version, client configuration, and prints next steps. You can also test from the extension: go to **Options** and click the **Test** button next to the Server URL.

## Version Mismatch Warning

**Symptom**: Extension popup shows a yellow "Version mismatch" banner.

**Cause**: The extension and server have different major versions.

**To fix**:
1. Update: re-run the installer to get the latest version of both:
   ```bash
   curl -sSL https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/install.sh | bash
   ```
2. Reload the extension: go to `chrome://extensions`, remove the old version, click **Load unpacked**, select `~/KaboomAgenticDevtoolExtension`
3. Both should show the same major version

Minor/patch version differences are fine and won't trigger the warning.

## Using Debug Mode

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

## Reporting Issues

[Open an issue](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/issues/new) with:

1. **Environment**: OS, Chrome version, Kaboom version
2. **Steps to reproduce**
3. **Expected vs actual behavior**
4. **Extension popup screenshot**
5. **Debug log export** (JSON file from popup)
6. **Browser console errors** (right-click extension → Inspect → Console)
7. **Server output** (any errors from stderr)
