---
title: Fuel Any Agent
description: "Connect KaBOOM to any MCP-compatible coding agent. Configuration guides for Claude Code, OpenAI Codex, Cursor, Windsurf, Claude Desktop, Zed, Gemini CLI, OpenCode, Antigravity, and VS Code with Continue."
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['mcp', 'integration']
---

KaBOOM is an open-source MCP server that implements the [Model Context Protocol](https://modelcontextprotocol.io/) — a standard for connecting AI assistants to external tools. Any MCP-compatible tool can tap into your browser state. Zero dependencies. Localhost only.

## Supported Tools

| Tool | Config Location | Guide |
|------|----------------|-------|
| Claude Code | `.mcp.json` (project root) | [Setup →](/mcp-integration/claude-code/) |
| OpenAI Codex | `~/.codex/config.toml` | [Setup →](/mcp-integration/codex/) |
| Cursor | `~/.cursor/mcp.json` | [Setup →](/mcp-integration/cursor/) |
| Windsurf | `~/.codeium/windsurf/mcp_config.json` | [Setup →](/mcp-integration/windsurf/) |
| Claude Desktop | OS-specific config file | [Setup →](/mcp-integration/claude-desktop/) |
| Zed | `~/.config/zed/settings.json` | [Setup →](/mcp-integration/zed/) |
| Gemini CLI | `~/.gemini/settings.json` | [Setup →](/mcp-integration/gemini/) |
| OpenCode | `~/.config/opencode/opencode.json` | [Setup →](/mcp-integration/opencode/) |
| Antigravity | `~/.gemini/antigravity/mcp_config.json` | [Setup →](/mcp-integration/antigravity/) |
| VS Code + Continue | `~/.continue/config.json` | [Below](#vs-code-with-continue) |

## How MCP Mode Works

KaBOOM runs as a dual-mode server by default:

- **HTTP server** — background daemon for the browser extension
- **stdio transport** — MCP JSON-RPC over stdin/stdout
- **Auto-managed** — your AI tool starts and stops the server

## What MCP tools does KaBOOM provide?

KaBOOM exposes **5 tools** — each with multiple sub-modes controlled by a single parameter.

| Tool | What it does | Key sub-modes |
|------|-------------|---------------|
| `observe` | Passive browser state | errors, logs, network_waterfall, network_bodies, websocket_events, websocket_status, actions, vitals, page, tabs, screenshot, timeline, error_bundles, saved_videos, recordings, recording_actions, playback_results, log_diff_report |
| `analyze` | Active analysis | dom, performance, accessibility, security_audit, third_party_audit, error_clusters, history, link_health, link_validation, page_summary, api_validation, annotations, annotation_detail, draw_history, draw_session |
| `generate` | Code and report generation | reproduction, test, pr_summary, sarif, har, csp, sri, visual_test, annotation_report, annotation_issues, test_from_context, test_heal, test_classify |
| `configure` | Session management | store, load, noise_rule, clear, health, streaming, recording_start, recording_stop, playback, log_diff, telemetry, diff_sessions, audit_log, describe_capabilities |
| `interact` | Browser control | navigate, click, type, select, check, key_press, execute_js, highlight, refresh, back, forward, new_tab, upload, draw_mode_start, screen_recording_start, screen_recording_stop, paste, save_state, load_state, get_readable, get_markdown, navigate_and_wait_for, fill_form_and_submit, run_a11y_and_export_sarif |

### observe

Read passive browser state. Use the `what` parameter to select:

| Mode | Returns |
|------|---------|
| `errors` | Console errors with deduplication and noise filtering |
| `error_bundles` | Pre-assembled debugging context per error (error + network + actions + logs) |
| `logs` | All console output (configurable level and limit) |
| `network_waterfall` | Resource timing from PerformanceObserver |
| `network_bodies` | Request/response payloads for API debugging |
| `websocket_events` | WebSocket messages — filter by connection ID, direction |
| `websocket_status` | Active WebSocket connections with message rates |
| `actions` | User interactions (click, input, navigate, scroll, select) |
| `vitals` | Core Web Vitals — FCP, LCP, CLS, INP |
| `page` | Current page URL and title |
| `tabs` | All browser tabs with URLs and titles |
| `screenshot` | Viewport screenshot |
| `timeline` | Merged session timeline (actions + network + errors) |
| `saved_videos` | Recorded browser session videos |
| `recordings` | Recording metadata |
| `recording_actions` | Actions captured during a recording |
| `playback_results` | Results from a recording playback |
| `log_diff_report` | Compare error states between recordings |

### analyze

Trigger active analysis. Use the `what` parameter to select:

| Mode | Returns |
|------|---------|
| `dom` | Live DOM queries with CSS selectors |
| `performance` | Performance snapshots with regression detection |
| `accessibility` | WCAG audit (axe-core) |
| `error_clusters` | Deduplicated error grouping |
| `history` | Navigation history |
| `security_audit` | Security checks (credentials, PII, headers, cookies) |
| `third_party_audit` | Third-party script and origin analysis |
| `link_health` | Browser-based link checker with CORS detection |
| `link_validation` | Server-side URL validation |
| `page_summary` | Page structure summary |
| `api_validation` | API contract validation from captured traffic |
| `annotations` | Draw mode annotations from user feedback |
| `annotation_detail` | Full computed styles for an annotation |
| `draw_history` | List past draw mode sessions |
| `draw_session` | Get a specific draw session |

### generate

Generate artifacts from your session. Use the `format` parameter:

| Format | Output |
|--------|--------|
| `reproduction` | Playwright script reproducing user actions |
| `test` | Playwright test with network/error assertions |
| `pr_summary` | Markdown performance impact summary |
| `sarif` | SARIF accessibility report (standard format) |
| `har` | HTTP Archive export |
| `csp` | Content Security Policy from observed origins |
| `sri` | Subresource Integrity hashes for scripts/stylesheets |
| `visual_test` | Visual regression test from annotation session |
| `annotation_report` | Report from draw mode annotations |
| `annotation_issues` | Extracted issues from annotations |
| `test_from_context` | Test generated from error/interaction/regression context |
| `test_heal` | Repair broken Playwright selectors |
| `test_classify` | Classify test failures |

### configure

Manage session state and settings. Use the `action` parameter:

| Action | Effect |
|--------|--------|
| `store` / `load` | Persistent key-value storage (save/load/list/delete/stats) |
| `noise_rule` | Manage noise filtering rules (add/remove/list/auto_detect) |
| `clear` | Clear buffers (network, websocket, actions, logs, all) |
| `health` | Server health and memory stats |
| `streaming` | Enable/disable real-time event streaming |
| `recording_start` | Start capturing a browser session |
| `recording_stop` | Stop recording |
| `playback` | Replay a recording |
| `log_diff` | Compare error states between recordings |
| `telemetry` | Configure telemetry metadata mode |
| `diff_sessions` | Capture and compare session snapshots |
| `audit_log` | View MCP tool usage history |
| `describe_capabilities` | List available actions and capabilities |

### interact

Control the browser. Use the `action` parameter:

| Action | Effect |
|--------|--------|
| `navigate` | Navigate to a URL |
| `click` | Click an element (CSS or semantic selector) |
| `type` | Type text into an element |
| `select` | Select a dropdown option |
| `check` | Check/uncheck a checkbox |
| `key_press` | Press a key (Enter, Tab, Escape, etc.) |
| `execute_js` | Run JavaScript in the page context |
| `highlight` | Highlight a DOM element |
| `refresh` | Refresh the current page |
| `back` / `forward` | Browser history navigation |
| `new_tab` | Open a new tab |
| `upload` | File upload for native file dialogs |
| `draw_mode_start` | Activate visual annotation overlay |
| `screen_recording_start` / `screen_recording_stop` | Start/stop video recording |
| `paste` | Paste text at current focus |
| `save_state` / `load_state` | Save and restore browser state snapshots |
| `get_readable` / `get_markdown` | Extract page content as readable text or markdown |
| `navigate_and_wait_for` | Navigate to URL and wait for a selector to appear |
| `fill_form_and_submit` | Fill multiple form fields and submit in one action |
| `run_a11y_and_export_sarif` | Run accessibility audit and export SARIF in one step |

## Custom Port

If port 7890 is occupied:

```json
{
  "mcpServers": {
    "kaboom": {
      "command": "npx",
      "args": ["-y", "kaboom-agentic-browser", "--port", "7891"]
    }
  }
}
```

Update the extension's Server URL in Options to match.

## VS Code with Continue

Add to `~/.continue/config.json`:

```json
{
  "experimental": {
    "modelContextProtocolServers": [
      {
        "transport": {
          "type": "stdio",
          "command": "npx",
          "args": ["-y", "kaboom-agentic-browser"]
        }
      }
    ]
  }
}
```

## How do I verify KaBOOM is connected?

1. Restart your AI tool
2. KaBOOM starts automatically
3. Extension popup shows "Connected"
4. Ask your AI: _"What browser errors do you see?"_
