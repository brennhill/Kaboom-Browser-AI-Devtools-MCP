---
title: "KaBOOM v1.0.0 Released"
description: "Initial proof of concept - Go server with Chrome extension communication"
date: 2025-12-11T22:34:00Z
authors: [brennhill]
tags: [release]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['release', 'blog', 'v1']
---

## What's New in v1.0.0

Kaboom v1.0.0 is the first public release. It's a proof of concept showing a Go server talking to a Chrome extension to capture browser telemetry.

### Features

- **Basic Browser Telemetry** — Chrome extension captures console logs and sends them to a local Go server
- **HTTP API** — Simple REST endpoints to retrieve captured logs
- **Local-only** — All data stays on `127.0.0.1`, no cloud services
- **Zero dependencies** — Go binary with no external packages

### Architecture

- Chrome extension (MV3) — Listens to `console.log()` calls
- Go HTTP server (port 7890) — Receives and stores logs in memory
- Local persistence — Logs written to disk

### Known Limitations

- Console logs only (no network, exceptions, or WebSockets)
- Single client only
- No MCP protocol yet
- Memory-based storage (logs lost on restart)

### Installation

```bash
go build -o kaboom ./cmd/server
./kaboom

# Then load the extension in chrome://extensions/
```

This is early. Very early. But it works.

---

**Next steps:** Add network request capture, implement MCP protocol, persistent storage.

See [GitHub](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP) for source.
