---
title: "KaBOOM v5.7.5 Released"
description: "Fast-start MCP mode, port diagnostics, and improved error messages for better developer experience"
date: 2026-02-06
authors: [brennhill]
tags: [release]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['release', 'blog', 'v5']
---

## What's New in v5.7.5

This release makes KaBOOM MCP feel instant. The new fast-start mode responds to MCP clients in ~130ms while the daemon boots in the background.

### Fast-Start MCP Mode

Previously, MCP clients had to wait for the full daemon to boot before getting any response. Now, `initialize` and `tools/list` respond immediately from the bridge process:

```
Before: Client → wait 2-4s for daemon → get response
After:  Client → get response in ~130ms → daemon boots in background
```

This means your AI coding agent gets tool definitions instantly and can start planning while the server finishes starting up. If you call a tool before the daemon is ready, you get a helpful retry message instead of a hang.

### Port Conflict Diagnostics

The `--doctor` command now checks if port 7890 is available:

```bash
npx kaboom-agentic-browser --doctor

# Now shows:
# ✅ Port 7890
#    Default port is available
#
# Or if blocked:
# ⚠️  Port 7890
#    Port 7890 is in use (PID: 12345)
#    Suggestion: Use --port 7891 or kill the process using the port
```

### Better Error Messages

When the daemon can't start because the port is blocked, you now get actionable suggestions:

```
Server failed to start: port 7890 already in use. Port may be in use. Try: npx kaboom-agentic-browser --port 7891
```

### Faster Failure Detection

Daemon startup timeout reduced from 10s to 4s. If something is wrong, you'll know in 4 seconds instead of 10.

## Upgrade

```bash
npx kaboom-agentic-browser@5.7.5
```

## Full Changelog

See the complete list of changes on [GitHub](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/releases/tag/v5.7.5).
