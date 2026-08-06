---
title: "KaBOOM v5.3.0 Released"
description: "WebSocket inspection and improved performance"
date: 2026-01-13T19:34:00Z
authors: [brennhill]
tags: [release]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['release', 'blog', 'v5']
---

## What's New in v5.3.0

Kaboom v5.3.0 adds comprehensive WebSocket inspection and significant performance improvements.

### Features

- **WebSocket Capture** — Full message tracking for WebSocket connections
- **Real-time Message Streaming** — Monitor WebSocket traffic as it happens
- **Binary Message Support** — Handle binary WebSocket frames alongside text
- **Connection State Tracking** — Visualize connection open/close lifecycle

### Performance

- 40% reduction in memory overhead
- Optimized log buffer management
- Faster message serialization

### Fixes

- Fixed observer timeout on high message volume
- Improved handling of concurrent connections
- Better cleanup of abandoned resources

## Upgrade

```bash
npm install -g kaboom-agentic-browser@5.3.0
```

## Full Changelog

[v5.3.0 Release](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/releases/tag/v5.3.0)
