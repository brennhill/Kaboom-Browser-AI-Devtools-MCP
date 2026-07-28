---
title: "Fuel Any Agent"
description: "Connect Kaboom to any MCP-compatible coding agent. Configuration guides for Claude Code, OpenAI Codex, Cursor, Windsurf, Claude Desktop, Zed, and VS Code with Continue."
keywords: "MCP server configuration, Model Context Protocol, autonomous coding agent, agentic debugging, browser debugging MCP"
permalink: /mcp-integration/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "One config. Your AI tool fires up Kaboom automatically."
toc: true
toc_sticky: true
status: reference
last_reviewed: 2026-03-28
---

Kaboom implements the [Model Context Protocol](https://modelcontextprotocol.io/) — a standard for connecting AI assistants to external tools. Any MCP-compatible tool can tap into your browser state.

## <i class="fas fa-plug"></i> Supported Tools

| Tool | Config Location | Guide |
|------|----------------|-------|
| <i class="fas fa-terminal"></i> Claude Code | `.mcp.json` (project root) | [Setup →](/mcp-integration/claude-code/) |
| <i class="fas fa-code"></i> OpenAI Codex | `~/.codex/config.toml` | [Setup →](/mcp-integration/codex/) |
| <i class="fas fa-i-cursor"></i> Cursor | `~/.cursor/mcp.json` | [Setup →](/mcp-integration/cursor/) |
| <i class="fas fa-wind"></i> Windsurf | `~/.codeium/windsurf/mcp_config.json` | [Setup →](/mcp-integration/windsurf/) |
| <i class="fas fa-desktop"></i> Claude Desktop | OS-specific config file | [Setup →](/mcp-integration/claude-desktop/) |
| <i class="fas fa-bolt"></i> Zed | `~/.config/zed/settings.json` | [Setup →](/mcp-integration/zed/) |
| <i class="fas fa-code"></i> VS Code + Continue | `~/.continue/config.json` | [Below](#-vs-code-with-continue) |

## <i class="fas fa-fire"></i> How MCP Mode Works

**Critical:** Your AI tool spawns a SINGLE Kaboom process that handles both:

- <i class="fas fa-server"></i> **HTTP server** (port 7890) — for browser extension telemetry capture
- <i class="fas fa-exchange-alt"></i> **MCP stdio** — for AI tool commands (observe, analyze, generate, configure, interact)

**Both interfaces share the same browser state.** Do NOT manually start Kaboom with `npx kaboom-agentic-browser` or `go run` — let your AI tool's MCP system spawn and manage the process. If you have a manually-started Kaboom instance on port 7890, kill it first to avoid conflicts.

## <i class="fas fa-tools"></i> Available MCP Tools

Kaboom exposes **5 tools** — each with multiple sub-modes controlled by a single parameter.

| Tool | What it does | Key sub-modes |
|------|-------------|---------------|
| `observe` | Read captured browser state | errors, logs, network_waterfall, network_bodies, websocket_events, actions, vitals, page, tabs, timeline, error_bundles, screenshot |
| `analyze` | Active analysis and audits | dom, performance, accessibility, security_audit, third_party_audit, error_clusters, link_health, annotations |
| `generate` | Code and report generation | reproduction, csp, sarif, test_from_context, test_heal, visual_test, annotation_report |
| `configure` | Session management | store, load, noise_rule, clear, health, streaming, recording_start, recording_stop, log_diff |
| `interact` | Browser control and automation | navigate, click, type, select, execute_js, highlight, scroll_to, key_press, upload, draw_mode_start |

## <i class="fas fa-layer-group"></i> Token-Efficient MCP Resources

Kaboom also exposes MCP resources so agents can discover capabilities with minimal prompt overhead and load details only when needed.

| Resource URI | Purpose | When to Read |
|---|---|---|
| `kaboom://capabilities` | Compact capability index + routing hints | First step for workflow selection |
| `kaboom://guide` | Full usage guide for all tools | When broad reference is needed |
| `kaboom://quickstart` | Canonical short examples | When you need quick command patterns |

Playbooks are available via template:

- `kaboom://playbook/{capability}/{level}`
- Example: `kaboom://playbook/performance/quick`
- Example: `kaboom://playbook/accessibility/quick`
- Example: `kaboom://playbook/security/full`
- Levels: `quick` (default recommended), `full` (deep workflow)

Recommended agent behavior:

1. Read `kaboom://capabilities` first.
2. Choose a matching playbook by intent.
3. Read only that playbook level (quick/full) for the active task.

### observe

Read captured browser state. Use the `what` parameter to select:

| Mode | Returns |
|------|---------|
| `errors` | Console errors with deduplication and noise filtering |
| `logs` | All console output (configurable level and limit) |
| `extension_logs` | Internal extension debug logs |
| `network_waterfall` | Resource timing from PerformanceObserver |
| `network_bodies` | Request/response payloads for API debugging (fetch only) |
| `websocket_events` | WebSocket messages — filter by connection ID, direction |
| `websocket_status` | Active WebSocket connections with message rates |
| `actions` | User interactions (click, input, navigate, scroll, select) |
| `vitals` | Core Web Vitals — FCP, LCP, CLS, INP |
| `page` | Current page URL, title, tracked status, tab ID |
| `tabs` | All tracked browser tabs |
| `pilot` | AI Web Pilot status |
| `timeline` | Merged session timeline (actions + network + errors) |
| `error_bundles` | Pre-assembled debug context per error (error + network + actions + logs) |
| `screenshot` | Current page screenshot |
| `command_result` | Async command result (by correlation_id) |
| `pending_commands` | Commands awaiting execution |
| `failed_commands` | Commands that timed out or failed |
| `saved_videos` | Saved recording files |
| `recordings` | Active recording sessions |
| `recording_actions` | Actions captured during recording |
| `log_diff_report` | Error state comparison report |

### analyze

Trigger active analysis operations. Use the `what` parameter:

| Mode | Returns |
|------|---------|
| `dom` | Live DOM inspection with CSS selectors |
| `performance` | Performance snapshots with regression detection |
| `accessibility` | WCAG audit results (axe-core) |
| `error_clusters` | Deduplicated error grouping |
| `history` | Navigation history exploration |
| `security_audit` | Security checks (credentials, PII, headers, cookies) |
| `third_party_audit` | Third-party script and origin analysis |
| `link_health` | Link health validation for a domain |
| `link_validation` | Validate specific links on the page |
| `annotations` | Retrieve draw mode annotations (user visual feedback) |
| `annotation_detail` | Full computed styles and DOM detail for a specific annotation |

### generate

Generate artifacts from your session. Use the `format` parameter:

| Format | Output |
|--------|--------|
| `reproduction` | Playwright script reproducing user actions |
| `csp` | Content Security Policy from observed origins |
| `sarif` | SARIF accessibility report (standard format) |
| `visual_test` | Visual regression test from annotations |
| `annotation_report` | Markdown report of all annotations |
| `annotation_issues` | GitHub/Jira issues from annotations |
| `test_from_context` | Generate Playwright test from current browser context |
| `test_heal` | Auto-repair broken selectors in existing tests |
| `test_classify` | Classify test failures (flaky, broken, environment) |

### configure

Manage session state and settings. Use the `action` parameter:

| Action | Effect |
|--------|--------|
| `store` | Persistent key-value storage (save/load/list/delete/stats) |
| `load` | Load persisted session data |
| `noise_rule` | Manage noise filtering rules (add/remove/list/auto_detect) |
| `clear` | Clear buffers (network, websocket, actions, logs, all). `buffer:"all"` also resets annotation sessions/details/waiters. |
| `health` | Server health and memory stats |
| `streaming` | Enable/disable push event notifications |
| `test_boundary_start` | Mark the start of a test boundary |
| `test_boundary_end` | Mark the end of a test boundary |
| `recording_start` | Start a browser recording session |
| `recording_stop` | Stop a browser recording session |
| `playback` | Replay a recorded session |
| `log_diff` | Compare error states between two points |

### interact

Control the browser. Requires AI Web Pilot. Use the `action` parameter:

| Action | Effect |
|--------|--------|
| `navigate` | Navigate to a URL |
| `execute_js` | Run JavaScript in the page context |
| `highlight` | Highlight a DOM element by CSS selector |
| `subtitle` | Display persistent narration text (composable with any action) |
| `refresh` | Refresh the current page |
| `back` / `forward` | Browser history navigation |
| `new_tab` | Open a new tab |
| `click` | Click an element |
| `type` | Type text into an input |
| `select` | Select an option from a dropdown |
| `check` | Check/uncheck a checkbox |
| `get_text` / `get_value` / `get_attribute` | Read element state |
| `set_attribute` / `focus` / `scroll_to` | Modify element state |
| `wait_for` | Wait for an element to appear |
| `key_press` | Press a keyboard key (Enter, Tab, Escape, etc.) |
| `list_interactive` | List all interactive elements on the page |
| `upload` | Upload a file to a file input |
| `save_state` / `load_state` / `list_states` / `delete_state` | Browser state snapshots |
| `screen_recording_start` / `screen_recording_stop` | Start/stop browser recording |
| `draw_mode_start` | Activate annotation overlay for visual feedback |

## <i class="fas fa-cog"></i> Custom Port

If port 7890 is occupied:

```json
{
  "mcpServers": {
    "kaboom-browser-devtools": {
      "command": "npx",
      "args": ["kaboom-agentic-browser", "--port", "7891"]
    }
  }
}
```

Update the extension's Server URL in Options to match.

## <i class="fas fa-code"></i> VS Code with Continue

Add to `~/.continue/config.json`:

```json
{
  "experimental": {
    "modelContextProtocolServers": [
      {
        "transport": {
          "type": "stdio",
          "command": "npx",
          "args": ["kaboom-agentic-browser"]
        }
      }
    ]
  }
}
```

## <i class="fas fa-check-circle"></i> Verify the Connection

1. Restart your AI tool
2. Kaboom server ignites automatically
3. Extension popup shows "Connected"
4. Ask your AI: _"What browser errors do you see?"_

## <i class="fas fa-exclamation-triangle"></i> Troubleshooting

### "bind: address already in use" error

If MCP fails to start with a port conflict:

```bash
# Find and kill existing Kaboom process
ps aux | grep kaboom-agentic-browser | grep -v grep
kill <PID>

# Or if on macOS/Linux:
pkill -f kaboom-agentic-browser
```

Then reload your MCP connection. The MCP system will spawn a fresh instance.

### Extension shows "Disconnected"

- Check that your AI tool has started the MCP server (look for Kaboom in process list)
- Verify the extension's Server URL matches the port in your MCP config (default: `http://localhost:7890`)
- Try restarting your AI tool to re-initialize the MCP connection

### No browser data appearing in AI responses

1. Open the extension popup and verify "Connected" status
2. Check capture level is not set to "Errors Only" if you expect all logs
3. Refresh the browser page to ensure content script is injected
4. Ask your AI to run: `observe({what: "page"})` to verify MCP communication
