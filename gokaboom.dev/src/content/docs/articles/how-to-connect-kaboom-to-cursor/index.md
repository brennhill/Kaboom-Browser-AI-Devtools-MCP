---
title: "How to Connect KaBOOM to Cursor"
description: "Beginner guide to connect Cursor with KaBOOM Agentic Devtools and run your first browser-aware workflow."
date: 2026-03-05
authors: [brenn]
tags: [beginner, cursor, mcp, setup]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['beginner', 'cursor', 'mcp', 'setup', 'articles', 'connect', 'kaboom']
---

Cursor is excellent for writing code. KaBOOM makes Cursor workflows browser-aware.

Here is the fastest setup path.

<!-- more -->

## Quick Terms

- **MCP (Model Context Protocol):** Connects Cursor to external tools. https://modelcontextprotocol.io/specification/
- **Regression:** A bug that returns after being fixed.

## Step 1: Confirm KaBOOM command is available

```bash
npx -y kaboom-agentic-browser --help
```

## Step 2: Add KaBOOM as an MCP server in Cursor

Use this config block:

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

## Step 3: Restart Cursor

Restart so Cursor reloads MCP servers.

## Step 4: Run your first runtime checks

```js
observe({what: "errors"})
observe({what: "network_bodies", status_min: 400})
```

## Step 5: Save the workflow for next time

```js
generate({what: "test", test_name: "first-cursor-kaboom-check"})
```

Now you have a repeatable baseline, not a one-off debugging session.

## Image and Diagram Callouts

> [Image Idea] Cursor MCP settings showing a connected `kaboom` server.

> [Diagram Idea] Cursor edit loop with runtime checks between each change.
