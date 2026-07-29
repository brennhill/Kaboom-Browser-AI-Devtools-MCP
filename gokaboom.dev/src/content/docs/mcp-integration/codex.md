---
title: KaBOOM + OpenAI Codex
description: "Configure KaBOOM as an MCP server for OpenAI Codex. Give Codex access to browser console logs, network errors, and DOM state."
last_verified_version: 0.8.8
last_verified_date: 2026-07-26
normalized_tags: ['mcp', 'integration', 'openai', 'codex']
---

KaBOOM is an open-source MCP server that gives OpenAI Codex access to browser console logs, network errors, exceptions, WebSocket events, and live DOM state. Zero dependencies.

## Auto-Install

The native binary and npm CLI can write the Codex config for you:

```bash
kaboom-agentic-browser --install codex
```

This writes a managed `[mcp_servers.kaboom-browser-devtools]` block to `~/.codex/config.toml` (or `$CODEX_HOME/config.toml`) and sets whole-server tool approval. Your existing settings and comments are preserved — only the KaBOOM block is replaced.

The `install.sh` / `install.ps1` bootstrap also uses the native Codex writer
when a Codex home exists. It honors `$CODEX_HOME`, preserves unrelated TOML and
comments, and replaces only Kaboom's managed MCP server block.

## Manual Configuration

Add to `~/.codex/config.toml`:

```toml
[mcp_servers.kaboom-browser-devtools]
command = "npx"
args = ["-y", "kaboom-agentic-browser"]
```

To trust every KaBOOM tool so Codex never prompts for approval:

```toml
[mcp_servers.kaboom-browser-devtools]
command = "npx"
args = ["-y", "kaboom-agentic-browser"]
default_tools_approval_mode = "approve"
```

`default_tools_approval_mode` applies to the whole server. Only `approve` suppresses prompts unconditionally — `auto` and `writes` can still prompt for tools that carry a risk hint.

If you installed a standalone binary, replace the `command` value with the full binary path and drop `args`.

## Repository Instructions

For Codex project-level instructions, use `AGENTS.md` in your repository root.

## Usage

After configuring, restart Codex and ask:

- _"What browser errors do you see?"_
- _"Check failed network requests for this page."_
- _"Run an accessibility audit and summarize issues."_

Codex gets all 5 KaBOOM tools: `observe`, `analyze`, `generate`, `configure`, and `interact`.

## Custom Port

If port 7890 is occupied:

```toml
[mcp_servers.kaboom-browser-devtools]
command = "npx"
args = ["-y", "kaboom-agentic-browser", "--port", "7891"]
```

Update the extension's Server URL in Options to match.

## Troubleshooting

1. **Restart Codex** after editing `config.toml`
2. **Verify the KaBOOM extension popup** shows "Connected"
3. **Check MCP visibility** by asking Codex which MCP tools are available
4. **Avoid duplicate servers** — stop manually started KaBOOM processes before using MCP-managed mode
