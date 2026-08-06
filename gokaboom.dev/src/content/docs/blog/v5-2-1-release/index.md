---
title: "KaBOOM v5.2.1 Released"
description: "Hotfix for error clustering and network filtering"
date: 2026-01-10T21:47:00Z
authors: [brennhill]
tags: [release]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['release', 'blog', 'v5']
---

## What's New in v5.2.1

Quick patch release fixing issues with error clustering in v5.2.0.

### Fixes

- Fixed error clustering not deduplicating similar stack frames
- Resolved network filter persistence across tab navigation
- Improved WebSocket connection tracking stability

## Upgrade

```bash
npm install -g kaboom-agentic-browser@5.2.1
```

## Full Changelog

[v5.2.1 Release](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/releases/tag/v5.2.1)
