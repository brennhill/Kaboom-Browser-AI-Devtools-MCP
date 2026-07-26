---
title: "How to Connect KaBOOM to OpenAI Codex"
description: "Beginner guide to connect OpenAI Codex with KaBOOM Agentic Devtools and run your first browser-aware workflow."
date: 2026-07-26
authors: [brenn]
tags: [beginner, codex, openai, mcp, setup]
last_verified_version: 0.8.8
last_verified_date: 2026-07-26
normalized_tags: ['beginner', 'codex', 'openai', 'mcp', 'setup', 'articles', 'connect', 'kaboom']
---

OpenAI Codex is excellent for code changes. KaBOOM makes Codex workflows browser-aware.

Here is the fastest setup path.

<!-- more -->

## Quick Terms

- **MCP (Model Context Protocol):** Connects Codex to external tools. https://modelcontextprotocol.io/specification/
- **AGENTS.md:** Project-level instruction file used by Codex in your repository.

## Step 1: Confirm KaBOOM command is available

```bash
npx -y kaboom-agentic-browser --help
```

## Step 2: Add KaBOOM as an MCP server in Codex

Codex reads MCP servers from `~/.codex/config.toml`, which is TOML rather than JSON. Add this block:

```toml
[mcp_servers.kaboom]
command = "npx"
args = ["-y", "kaboom-agentic-browser"]
```

If you installed KaBOOM from npm, you can skip the hand-editing and let the Command Line Interface (CLI) write that block:

```bash
kaboom-agentic-browser --install codex
```

## Step 3: Restart Codex

Restart so Codex reloads MCP servers.

## Step 4: Run your first runtime checks

```js
observe({what: "errors"})
observe({what: "network_bodies", status_min: 400})
```

## Step 5: Turn findings into a reproducible artifact

```js
generate({what: "reproduction"})
```

Now you have a repeatable baseline for debugging instead of one-off console checks.

## Image and Diagram Callouts

> [Image Idea] `~/.codex/config.toml` showing the `mcp_servers.kaboom` table.

> [Diagram Idea] Codex prompt -> KaBOOM observe/analyze -> fix + verification loop.
