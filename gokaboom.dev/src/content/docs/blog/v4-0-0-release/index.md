---
title: "KaBOOM v4.0.0 Released"
description: "Refinement phase - polishing the core, adding developer experience features"
date: 2025-12-22T21:15:00Z
authors: [brennhill]
tags: [release]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['release', 'blog', 'v4']
---

## What's New in v4.0.0

Kaboom v4.0.0 is the refinement phase. The core architecture from v3 works well—this version focuses on making it robust and adding missing developer-focused features.

### Major Features

- **User Action Recording** — Record clicks, typing, navigation with smart selectors
- **Web Vitals Capture** — LCP, CLS, INP, FCP tracking
- **API Schema Inference** — Auto-detect OpenAPI patterns from network traffic
- **Error Aggregation** — Group and deduplicate similar errors
- **Session Checkpoints** — Save browser state, diff changes, detect regressions

### Performance

- **Optimized ring buffers** — 2000-event cap prevents unbounded memory growth
- **Efficient filtering** — Skip irrelevant logs (ads, tracking, etc.)
- **Smart deduplication** — Collapse repeated identical events
- **Rate limiting** — Respect browser and extension quotas

### Developer Experience

- **Better error messages** — Clear explanations of what went wrong and why
- **Command-line flags** — `--port`, `--server`, `--api-key` for flexibility
- **Health check endpoint** — `/health` for verifying setup
- **Example integration** — Claude Code, Cursor, Copilot configs included

### Stability

- **Comprehensive testing** — 80+ unit and integration tests
- **Error recovery** — Graceful handling of extension crashes/restarts
- **Logging improvements** — Debug mode for troubleshooting
- **TypeScript strict mode** — Zero implicit any

### Known Limitations

- Recording still in alpha (no video yet)
- No Safari/Firefox support (MV3 requirement)
- Replay system not yet implemented
- Limited to Chrome 120+

---

**Status:** This version proved the concept could handle real-world usage. Ready for broader testing.

Next: Production-grade recording system, broader browser support research, performance benchmarking.

See [GitHub](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP for source.
