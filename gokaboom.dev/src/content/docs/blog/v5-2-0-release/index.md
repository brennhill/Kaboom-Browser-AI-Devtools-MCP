---
title: "KaBOOM v5.2.0 Released"
description: "Improved error handling and network inspection"
date: 2026-01-10T20:15:00Z
authors: [brennhill]
tags: [release]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['release', 'blog', 'v5']
---

## What's New in v5.2.0

Kaboom v5.2.0 improves error handling, network inspection, and adds better filtering options for high-volume environments.

### Features

- **Enhanced Error Clustering** — Group related errors for clearer debugging
- **Network Filtering** — Filter requests by status, content-type, and size
- **Better Error Context** — Stack traces and request/response details inline
- **Performance Improvements** — Reduced memory usage on long-running sessions

### Improvements

- Error deduplication across similar stack traces
- Network waterfall visualization improvements
- Better handling of WebSocket connections
- Improved timeout handling for slow networks

## Upgrade

```bash
npm install -g kaboom-agentic-browser@5.2.0
```

## Full Changelog

[v5.2.0 Release](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/releases/tag/v5.2.0)
