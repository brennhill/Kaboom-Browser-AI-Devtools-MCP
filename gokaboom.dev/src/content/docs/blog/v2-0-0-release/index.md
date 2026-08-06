---
title: "KaBOOM v2.0.0 Released"
description: "First MCP implementation - proof of concept, not production-ready"
date: 2025-12-14T23:47:00Z
authors: [brennhill]
tags: [release]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['release', 'blog', 'v2']
---

## What's New in v2.0.0

Kaboom v2.0.0 adds MCP protocol support. The implementation is naive and has significant limitations, but it works as a proof of concept.

### Features

- **MCP Protocol (Naive)** — Basic JSON-RPC 2.0 transport over HTTP
- **Console Logs + Network Errors** — Extension captures logs and failed API calls (4xx, 5xx)
- **Simple Resource API** — `resources/list` and `resources/read` endpoints
- **Multi-tool stub** — Four tools defined but only one partially implemented

### Architecture

- Extension → Go server (HTTP)
- Go server → MCP client (HTTP polling)
- Blocking request/response model

### What We Learned

This version taught us that:
- HTTP polling is too slow for real-time telemetry
- Single-request blocking architecture doesn't work for streaming data
- MCP needs bidirectional communication, not request/response

### Limitations

- **Performance** — 500ms+ latency due to polling
- **Streaming** — Can't stream data in real-time
- **Single tool** — Only `observe()` partially works
- **Network overhead** — Constant polling even with no data

### Known Issues

- Random timeouts on concurrent requests
- Lost messages during high-frequency events
- Extension crashes if server is restarted

---

**Lesson learned:** HTTP polling doesn't work for a telemetry system. Next version will use WebSockets.

See [GitHub](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP for source.
