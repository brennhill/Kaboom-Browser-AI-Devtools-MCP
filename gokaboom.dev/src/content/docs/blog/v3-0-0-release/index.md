---
title: "KaBOOM v3.0.0 Released"
description: "MCP stdio server - proper bidirectional protocol implementation"
date: 2025-12-19T01:23:00Z
authors: [brennhill]
tags: [release]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['release', 'blog', 'v3']
---

## What's New in v3.0.0

Kaboom v3.0.0 replaces the HTTP polling disaster with a proper MCP stdio server. This was the breakthrough version.

### Major Changes

- **Stdio-based MCP Server** — Proper bidirectional JSON-RPC 2.0 over stdio
- **Real-time streaming** — Events stream immediately, no polling
- **All 4 tools implemented** — `observe()`, `generate()`, `configure()`, `interact()`
- **Event types** — Console logs, network requests, WebSocket messages, exceptions

### Features

- **Console Logs** — Full argument capture, log levels
- **Network Capture** — Request/response bodies, headers, status codes
- **WebSocket Events** — Message payloads and connection lifecycle
- **Exceptions** — Stack traces and error context
- **User Actions** — Click/type/navigate event recording
- **Persistence** — Logs survive server restart (SQLite backend)

### Architecture

- Extension → Go daemon (localhost HTTP)
- Daemon ↔ MCP client (stdio, bidirectional)
- Ring buffer for event history
- Persistent storage for audit trail

### Performance

- **<100ms latency** for event delivery
- **Concurrent clients** — Multiple AI tools can connect simultaneously
- **Low memory** — Circular buffer limits are enforced
- **Zero external deps** — Pure Go stdlib

### Known Limitations

- **Chrome-only** — Manifest V3 requirement
- **Local network only** — Binds to 127.0.0.1
- **No replay** — Can't scrub through history yet
- **Recording not yet implemented**

---

**Milestone:** This was the version that proved the concept could work. Everything from here builds on this foundation.

See [GitHub](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP for source.
