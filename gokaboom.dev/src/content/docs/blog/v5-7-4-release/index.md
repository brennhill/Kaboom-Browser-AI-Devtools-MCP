---
title: "KaBOOM v5.7.4 Released"
description: "Stability and MCP reliability improvements"
date: 2026-02-07T22:56:00Z
authors: [brennhill]
tags: [release]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['release', 'blog', 'v5']
---

## What's New in v5.7.4

Kaboom v5.7.4 improves stability and MCP protocol reliability based on production feedback.

### Improvements

- Better handling of slow client connections
- Improved timeout recovery and reconnection logic
- Enhanced message serialization performance
- More robust error reporting to clients

### Fixes

- Fixed observer timeout on pages with extremely high event volume
- Resolved occasional message ordering issues
- Improved cleanup of abandoned connections
- Better resilience to malformed MCP requests

### Performance

- Reduced latency for high-frequency events
- Optimized buffer management for large responses

## Upgrade

```bash
npm install -g kaboom-agentic-browser@5.7.4
```

## Full Changelog

[v5.7.4 Release](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/releases/tag/v5.7.4)
