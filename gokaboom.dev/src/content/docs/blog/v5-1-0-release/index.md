---
title: "KaBOOM v5.1.0: Single-Tab Tracking Isolation"
description: "v5.1.0 fixes a critical privacy vulnerability where all browser tabs were captured regardless of tracking state. Plus network schema improvements and PyPI distribution."
date: 2026-01-28T20:25:00Z
authors:
  - brenn
tags:
  - releases
  - security
  - privacy
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['--releases', 'releases', 'security', 'privacy', 'blog', 'v5', 'release']
---

Kaboom v5.1.0 is a security-focused release that fixes a critical privacy vulnerability in how the extension captures browser telemetry. If you're running any previous version, upgrade immediately.

## The Problem: All Tabs Were Captured

Prior to v5.1.0, the extension captured console logs, network requests, and other telemetry from **every open browser tab** — regardless of whether tracking was enabled for that tab. If you had 40 tabs open and clicked "Track This Page" on one of them, data from all 40 tabs was forwarded to the MCP server.

This was a privacy vulnerability. Tabs containing banking sites, personal email, or other sensitive sessions would leak telemetry into the AI assistant's context.

## The Fix: Single-Tab Tracking Isolation

v5.1.0 introduces **tab-scoped filtering** in the content script. The extension now:

1. **Only captures from the explicitly tracked tab.** All other tabs are completely isolated.
2. **Attaches a `tabId`** to every forwarded message for data attribution.
3. **Blocks Chrome internal pages** (`chrome://`, `about://`, `devtools://`) from being tracked.
4. **Clears tracking state on browser restart** — no stale tab references.

The button has been renamed from "Track This Page" to **"Track This Tab"** to reflect the actual behavior.

## "No Tracking" Mode

When no tab is tracked, the MCP server now prepends a warning to all `observe()` responses:

> WARNING: No tab is being tracked. Data capture is disabled. Ask the user to click 'Track This Tab' in the KaBOOM extension popup.

This prevents the AI assistant from silently operating on stale or missing data.

## Network Schema Improvements

API responses from `network_waterfall` and `network_bodies` now include:

- **Unit suffixes**: `durationMs`, `transferSizeBytes` instead of ambiguous `duration`, `size`
- **`compressionRatio`**: Computed field showing transfer efficiency
- **`capturedAt`** timestamps on all entries
- **`limitations`** array explaining what the data can and can't tell you

These changes help LLMs interpret network data without guessing units.

## PyPI Distribution

Kaboom is now available on PyPI alongside NPM:

```bash
pip install kaboom-agentic-browser
kaboom-agentic-browser
```

Same binary, same behavior. Platform-specific wheels for macOS (arm64, x64), Linux (arm64, x64), and Windows (x64).

## Known Issues

Five issues are deferred to v5.2. See [KNOWN-ISSUES.md](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/blob/stable/KNOWN-ISSUES.md) for details:

- `query_dom` not yet implemented
- Accessibility audit runtime error
- `network_bodies` returns no data in some cases
- Extension timeouts after several operations
- `observe()` responses missing `tabId` metadata

## Upgrade

```bash
npx kaboom-agentic-browser@5.1.0
```

Or update your `.mcp.json`:

```json
{
  "mcpServers": {
    "kaboom": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "kaboom-agentic-browser@5.1.0", "--port", "7890", "--persist"]
    }
  }
}
```

## Full Changelog

[GitHub Release](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/releases/tag/v5.1.0) · [CHANGELOG.md](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/blob/stable/CHANGELOG.md)
