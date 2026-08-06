---
title: "KaBOOM v0.7.0 Released"
description: "Major release: complete browser observability platform for AI coding agents with zero-dependency Go daemon, Chrome MV3 extension, and 5 MCP tools."
date: 2026-02-15
authors: [brennhill]
tags: [release]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['release', 'blog', 'v0']
---

## What's New in v0.7.0

v0.7.0 is a ground-up rewrite delivering a complete browser observability platform. This is the first stable release — all prior versions are deprecated.

### Highlights

- **Zero-dependency Go daemon** with MCP JSON-RPC 2.0 protocol — no runtime dependencies, single binary
- **Chrome MV3 extension** capturing console logs, network requests, DOM state, screenshots, and Web Vitals in real-time
- **5 MCP tools** — observe, generate, configure, interact, analyze — giving AI agents full browser visibility
- **File upload pipeline** with 4-stage escalation and OS automation for native file dialogs
- **Draw mode** for visual region selection and annotation directly in the browser
- **SARIF export** for integrating accessibility and security findings into CI/CD pipelines
- **Session recording** with WebM video capture
- **Pilot mode** for autonomous browser interaction via AI agents
- **Link health analysis** with CORS detection
- **npm + PyPI distribution** with auto-update daemon lifecycle

### Features

- Complete MCP server with tool-based architecture (observe, generate, configure, interact, analyze)
- Real-time browser telemetry: console, network, WebSocket, DOM, performance, errors
- HAR and SARIF export for network traces and accessibility audits
- Test generation from browser interactions (Playwright, Vitest)
- Noise filtering with persistent rules
- CSP policy generation from observed network traffic
- Network waterfall analysis with body capture

### Install

```bash
curl -sSL https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/install.sh | bash
```

### Full Changelog

[View on GitHub](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/releases/tag/v0.7.0)
