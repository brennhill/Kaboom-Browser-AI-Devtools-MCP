---
title: "KaBOOM v5.6.0 Released"
description: "Server reliability, persistence guarantees, and architecture tests"
date: 2026-02-06
authors: [brennhill]
tags: [release]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['release', 'blog', 'v5']
---

## What's New in v5.6.0

Kaboom v5.6.0 focuses on server-side reliability with persistence guarantees and comprehensive architecture invariant tests.

### Features

- **Persistent Message Queue** — Guarantees no messages lost during server restarts
- **Transaction-Safe State** — Atomic operations for observer state updates
- **Architecture Validation** — New test suite validating core invariants

### Improvements

- Improved graceful shutdown of long-running observations
- Better handling of concurrent client connections
- Enhanced observability into server health and performance
- Stricter validation of MCP protocol compliance

### Testing

- 50+ new architecture invariant tests
- Stress testing with high message volume
- Connection resilience testing

## Upgrade

```bash
npm install -g kaboom-agentic-browser@5.6.0
```

## Full Changelog

[v5.6.0 Release](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/releases/tag/v5.6.0)
