---
title: "KaBOOM v5.4.3 Released"
description: "Bug fixes and stability improvements"
date: 2026-01-18
authors: [brennhill]
tags: [release]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['release', 'blog', 'v5']
---

## What's New in v5.4.3

Kaboom v5.4.3 includes additional bug fixes and stability improvements following v5.4.1.

### Fixes

- Fixed click handling on elements inside iframes
- Resolved memory leak in toast notification cleanup
- Improved robustness of element scrolling
- Better error recovery when pages reload during interaction

### Performance

- Reduced CPU usage during idle observation
- Optimized selector matching algorithm

## Upgrade

```bash
npm install -g kaboom-agentic-browser@5.4.3
```

## Full Changelog

[v5.4.3 Release](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/releases/tag/v5.4.3)
