---
title: "Kaboom + OpenAI Codex"
description: "Configure Kaboom as an MCP server for OpenAI Codex. Give Codex access to browser console logs, network errors, and DOM state."
keywords: "OpenAI Codex MCP server, Codex browser errors, Codex debugging, Codex MCP integration"
permalink: /mcp-integration/codex/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Fuel OpenAI Codex with live browser data."
toc: true
toc_sticky: true
status: reference
last_reviewed: 2026-07-26
---

Kaboom is an open-source MCP server that gives OpenAI Codex access to browser console logs, network errors, exceptions, WebSocket events, and live DOM state.

Codex stores MCP servers in `~/.codex/config.toml` (TOML), not JSON like the other supported clients.

## <i class="fas fa-bolt"></i> Auto-Install

The npm CLI can write the Codex config for you:

```bash
kaboom-agentic-browser --install codex
```

This writes a managed `[mcp_servers.kaboom-browser-devtools]` block to `~/.codex/config.toml` (or `$CODEX_HOME/config.toml`) and sets whole-server tool approval. The edit is section-scoped: your other settings and comments are left byte-for-byte intact.

**The `install.sh` / `install.ps1` bootstrap does not cover Codex.** That script runs the native binary's `--install`, which configures JSON-based clients only. If you installed that way, use the manual configuration below.

## <i class="fas fa-file-code"></i> Manual Configuration

Add to `~/.codex/config.toml`:

```toml
[mcp_servers.kaboom-browser-devtools]
command = "npx"
args = ["kaboom-agentic-browser"]
```

To trust every Kaboom tool so Codex never prompts for approval:

```toml
[mcp_servers.kaboom-browser-devtools]
command = "npx"
args = ["kaboom-agentic-browser"]
default_tools_approval_mode = "approve"
```

`default_tools_approval_mode` is a whole-server setting. Only `approve` suppresses prompts unconditionally — `auto` and `writes` can still prompt for tools that carry a risk hint.

If you installed a standalone binary, replace `command = "npx"` with the full binary path and drop `args`.

## <i class="fas fa-book"></i> Repository Instructions

For Codex project-level instructions, use `AGENTS.md` in your repository root.

## <i class="fas fa-fire-alt"></i> Usage

After configuring, restart Codex and ask:

- _"What browser errors do you see?"_
- _"Check failed network requests for this page."_
- _"Run an accessibility audit and summarize issues."_

Codex gets all 5 Kaboom tools: `observe`, `analyze`, `generate`, `configure`, and `interact`.

## <i class="fas fa-cog"></i> Custom Port

If port 7890 is occupied:

```toml
[mcp_servers.kaboom-browser-devtools]
command = "npx"
args = ["kaboom-agentic-browser", "--port", "7891"]
```

Update the extension's Server URL in Options to match.

## <i class="fas fa-wrench"></i> Troubleshooting

1. **Restart Codex** after editing `config.toml`
2. **Verify extension** shows "Connected" in popup
3. **Verify MCP tools** by asking Codex what tools are available
4. **Avoid duplicate servers** — stop manually started Kaboom processes before using MCP-managed mode
