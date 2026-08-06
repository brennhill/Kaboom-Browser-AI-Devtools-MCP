---
title: "How to Connect KaBOOM to Windsurf"
description: "Simple beginner setup for connecting Windsurf to KaBOOM Agentic Devtools through MCP."
date: 2026-03-05
authors: [brenn]
tags: [beginner, windsurf, mcp, setup]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['beginner', 'windsurf', 'mcp', 'setup', 'articles', 'connect', 'kaboom']
---

If you use Windsurf and want real browser evidence during development, this guide is for you.

<!-- more -->

## Quick Terms

- **MCP (Model Context Protocol):** A tool integration standard for AI assistants. https://modelcontextprotocol.io/specification/
- **Runtime evidence:** What actually happened in the browser (errors, requests, page state).

## Step 1: Check KaBOOM availability

```bash
npx -y kaboom-agentic-browser --help
```

## Step 2: Add KaBOOM to your Windsurf MCP servers

```json
{
  "mcpServers": {
    "kaboom": {
      "command": "npx",
      "args": ["-y", "kaboom-agentic-browser"]
    }
  }
}
```

## Step 3: Restart Windsurf

Windsurf needs a restart to load new MCP entries.

## Step 4: Run your first checks

```js
observe({what: "errors"})
observe({what: "logs", min_level: "error"})
```

## Step 5: Create a concrete artifact

```js
generate({what: "reproduction"})
```

This gives your team a reproducible script instead of vague notes.

## Image and Diagram Callouts

> [Image Idea] Windsurf MCP config with KaBOOM server entry.

> [Diagram Idea] Windsurf request -> KaBOOM tools -> bug artifact output.
