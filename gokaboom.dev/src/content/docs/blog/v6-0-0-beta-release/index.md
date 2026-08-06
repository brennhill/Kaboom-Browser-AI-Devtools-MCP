---
title: "KaBOOM v6.0.0 Released"
description: "AI agents can now check all links on your page, automate your browser with complete visibility, and identify performance bottlenecks in real time."
date: 2026-02-09T23:34:00Z
authors: [brennhill]
tags: [release]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['release', 'blog', 'v6', 'beta']
---

## What's New in v6.0.0

Kaboom v6.0.0 introduces the Link Health Analyzer, plus browser automation, recording, and performance analysis for AI agents. Check all links on your page, record full sessions with video, capture performance metrics, and let AI agents test, debug, and fix your app automatically. Complete visibility. You stay in control.

### Features

- **Link Health Analyzer** — Automatically check all links on your page for issues (broken, redirects, auth-required). 20 concurrent checks, categorized results, and async tracking with correlation IDs.

- **Full Recording System** — Record browser tabs with video and audio. Videos stream to local disk. No cloud, no transcoding—raw WebM format.

- **Permission Prompts** — When recording starts, you get a clear prompt to approve it. No silent recordings. You're always in control.

### Security

- **CWE-942 Fixed** — Replaced wildcard `postMessage` origins with `window.location.origin` across content scripts, test helpers, and background workers. Prevents message hijacking on cross-origin pages.

- **Secure Cookie Attributes** — Cookie deletion and restoration now include `Secure` and `SameSite` attributes, preventing session fixation and CSRF vulnerabilities.

- **Path Traversal Protection** — Hardened file operations in extension persistence layer to prevent directory traversal attacks.

- **Input Validation** — Comprehensive validation of extension log queue capacity (2000-entry cap) and screenshot rate limiter bounds to prevent unbounded memory growth.

### Performance

- **Smart HTTP Timeouts** — 5s default timeout for localhost operations, extended to 30s+ only when accessibility features are requested. Reduces false positives while respecting slow connections.

- **Atomic File Writes** — Log rotation uses temp + rename pattern, preventing partial writes and data loss on disk full.

- **Efficient Deduplication** — SeenMessages pruning optimized for large event volumes.

### Testing

- **99.4% Pass Rate** — 154 out of 155 smoke tests pass (one known edge case with watermark on rapid navigation).
- **Comprehensive UAT Suite** — 140 tests covering recording, permissions, security, performance, and WebSocket capture.
- **Full TypeScript Strict Mode** — No implicit any, zero Codacy security issues.

### Breaking Changes

- **Extension v5.x → v6.0.0** — Auto-update via Chrome. Manual re-add may be required if permissions are denied.
- **MCP Server** — Same 4-tool interface; no API changes.

## Upgrade

### Browser Extension

Direct CRX download for this historical beta build is no longer distributed from this docs site.
Use the current installer flow instead:

- [Full installation guide →](/downloads/)

### MCP Server

```bash
npm install -g kaboom-agentic-browser@6.0.0
kaboom-agentic-browser --help
```

Or via pip:
```bash
pip install kaboom-agentic-browser==6.0.0
```

## Known Limitations

- **Recording audio on muted tabs** — Tab audio capture requires tab to have sound playing. Silent tabs record video only.
- **Watermark on rapid navigation** — Watermark may not re-appear if user navigates during recording. Next navigation resets correctly.
- **Chrome 120+ only** — Manifest v3 (MV3) requires Chrome 120 or later. No Safari/Firefox support in v6.

## What's Next

- **File Upload API** — Automated file form handling for bulk uploads to no-API platforms.
- **Replay System** — Event playback with timeline scrubbing.
- **Deployment Integration** — Capture git-linked deploy events for post-incident correlation.

## Full Changelog

[v5.8.0...v6.0.0](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/compare/v5.8.0...v6.0.0)
