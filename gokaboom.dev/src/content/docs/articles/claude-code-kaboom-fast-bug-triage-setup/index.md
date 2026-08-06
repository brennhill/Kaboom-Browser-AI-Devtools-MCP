---
title: "Claude Code + KaBOOM: Fast Bug Triage Setup"
description: "A beginner-friendly setup guide for using Claude Code with KaBOOM Agentic Devtools for browser-aware bug triage."
date: 2026-03-03
authors: [brenn]
tags: [claude-code, mcp, debugging, setup]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['claude-code', 'mcp', 'debugging', 'setup', 'articles', 'claude', 'code', 'kaboom', 'fast', 'bug', 'bug-triage']
---

If you want your assistant to debug what is actually happening in the browser, this setup is for you.

This walkthrough helps first-time users connect **Claude Code** to **KaBOOM Agentic Devtools** and run a real bug triage loop.

<!-- more -->

## Quick Terms

- **Bug triage**: Sorting and diagnosing issues quickly by impact and cause.
- **MCP (Model Context Protocol)**: Standard for connecting AI assistants to tools. https://modelcontextprotocol.io/specification/
- **Tool call**: A structured request from assistant to an external capability.

## What You’ll Achieve

By the end, you can ask Claude Code:

“Find browser errors, inspect failing network calls, and suggest a fix.”

## Step-by-Step

### Step 1. Confirm KaBOOM is available

```bash
npx -y kaboom-agentic-browser --help
```

### Step 2. Configure Claude Code MCP entry

Use your Claude Code MCP config and add a server named `kaboom`.

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

### Step 3. Open your app and run first triage prompts

```js
observe({what: "errors"})
observe({what: "network_bodies", status_min: 400})
```

### Step 4. Turn diagnosis into artifact

```js
generate({what: "reproduction"})
generate({what: "test"})
```

## Easy First Prompt You Can Reuse

“Please triage this flow end to end: open page, reproduce issue, collect logs and failing requests, and propose a minimal fix.”

## Image and Diagram Callouts

> [Image Idea] Screenshot of MCP config snippet in Claude Code setup screen.

> [Diagram Idea] Triage loop: prompt -> observe -> analyze -> generate artifact.

## You’re Shipping with Better Feedback Loops

This setup turns your assistant into a practical debugging partner. **KaBOOM Agentic Devtools** gives Claude Code the runtime visibility it needs.
