---
title: "How to Connect KaBOOM to Claude Code"
description: "A step-by-step beginner guide to connect Claude Code with KaBOOM Agentic Devtools using MCP."
date: 2026-03-05
authors: [brenn]
tags: [beginner, claude-code, mcp, setup]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['beginner', 'claude-code', 'mcp', 'setup', 'articles', 'connect', 'kaboom', 'claude', 'code']
---

Want Claude Code to understand what is happening in the browser, not just in your source files?

This is the setup.

<!-- more -->

## Quick Terms

- **MCP (Model Context Protocol):** Standard for connecting AI assistants to tools. https://modelcontextprotocol.io/specification/
- **JSON (JavaScript Object Notation):** Text format used for tool config.

## Step 1: Confirm KaBOOM is installed

```bash
npx -y kaboom-agentic-browser --help
```

If this command works, you are ready to connect.

## Step 2: Add KaBOOM in Claude Code MCP config

Add this server entry:

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

## Step 3: Restart Claude Code

Reload so it picks up the new MCP server configuration.

## Step 4: Run a first browser-aware check

```js
observe({what: "errors"})
observe({what: "network_waterfall", status_min: 400})
```

## Step 5: Turn findings into action

```js
generate({what: "reproduction"})
```

Now Claude Code can help with bug triage using runtime evidence.

## Image and Diagram Callouts

> [Image Idea] Claude Code MCP config panel with the `kaboom` block highlighted.

> [Diagram Idea] Claude Code prompt -> KaBOOM observe/analyze -> fix suggestion loop.
