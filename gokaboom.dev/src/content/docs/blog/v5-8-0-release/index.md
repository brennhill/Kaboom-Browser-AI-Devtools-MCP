---
title: "KaBOOM v5.8.0 Released"
description: "Early-patch WebSocket capture, visual action toasts, and a 106-test UAT suite"
date: 2026-02-06T22:52:00Z
authors: [brennhill]
tags: [release]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['release', 'blog', 'v5']
---

## What's New in v5.8.0

Kaboom v5.8.0 solves a long-standing WebSocket capture blind spot: pages that create WebSocket connections before the inject script loads now have those connections captured automatically. This release also adds visual feedback for AI actions and ships a comprehensive 106-test UAT suite.

### Features

- **Early-patch WebSocket capture** — A new `world: "MAIN"` content script patches `window.WebSocket` before any page JavaScript runs. This means sites like Binance that create WebSocket connections immediately on page load now have those connections captured and visible via `observe(websocket_status)`. Buffered connections are seamlessly adopted when the full inject script initializes.

- **Visual action toasts** — When AI tools use `interact()` to navigate, execute JavaScript, or highlight elements, a brief toast overlay appears on the page showing what the AI is doing. This makes AI actions visible to developers watching the browser.

### Fixes

- Fixed camelCase to snake_case field mapping for network waterfall entries (duration, transfer_size, etc.)
- Command results now route through the /sync endpoint with proper client ID filtering
- After navigation, tracking state is broadcast so favicon updates correctly
- Empty arrays return `[]` instead of `null` in JSON responses
- Bridge timeouts now return a proper `extension_timeout` error code

### Testing

- **106-test parallel UAT suite** replacing the previous 8-test script, covering observe, generate, configure, interact, and data pipeline categories
- **16-test human smoke test** with error clusters, DOM query, full form lifecycle, highlight, and real WebSocket traffic tests
- All tests default to fail (not pass) with strict field validation throughout

### Performance

Binary sizes decreased ~4% from v5.7.5. All SLOs continue to pass:
- MCP fast-start: ~130ms
- Tool response: < 1ms
- Max binary: 7.7 MB (target: < 15 MB)

## Upgrade

```bash
npx kaboom-agentic-browser@5.8.0
```

## Full Changelog

[v5.7.5...v5.8.0](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/compare/v5.7.5...v5.8.0)
