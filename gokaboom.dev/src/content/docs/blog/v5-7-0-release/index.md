---
title: "KaBOOM v5.7.0 Released"
description: "New graceful shutdown, PID file management, and sync protocol for better extension connectivity"
date: 2026-02-05T20:33:00Z
authors: [brennhill]
tags: [release]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['release', 'blog', 'v5']
---

## What's New in v5.7.0

This release focuses on reliability and developer experience improvements, with a new graceful shutdown system and improved extension connectivity.

### Graceful Server Shutdown

Stop running servers cleanly with the new `--stop` flag:

```bash
kaboom-agentic-browser --stop                # Stop server on default port (7890)
kaboom-agentic-browser --stop --port 8080    # Stop server on specific port
```

The shutdown uses a hybrid approach for maximum reliability:
1. **PID file** (fast) - Reads process ID from `~/.kaboom-{port}.pid`
2. **HTTP endpoint** (graceful) - Sends shutdown request to `/shutdown`
3. **lsof fallback** - Finds process by port if other methods fail

### Sync Protocol

The extension now uses a server-sent events based `/sync` endpoint instead of polling. This means:
- Lower CPU usage when idle
- Faster response to server queries
- More reliable connection state tracking

### Server Always Runs as Daemon

The `--persist` flag has been removed. The server now always runs as a background daemon that persists until explicitly stopped with `--stop`.

### Internal Improvements

- New regression test framework in `tests/regression/`
- Comprehensive UAT test suite with shutdown tests
- Major documentation cleanup (40+ obsolete files removed)
- Better error handling for multi-client scenarios

## Upgrade

```bash
npx kaboom-agentic-browser@5.7.0
```

Or if you've installed globally:

```bash
npm install -g kaboom-agentic-browser@5.7.0
```

## Full Changelog

See the complete list of changes on [GitHub](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/releases/tag/v5.7.0).
