---
title: "How to Install KaBOOM Agentic Devtools in 5 Minutes (Mac, Windows, Linux)"
description: "A beginner-friendly install guide for KaBOOM Agentic Devtools, including extension setup and first-run checks."
date: 2026-03-05
authors: [brenn]
tags: [beginner, setup, installation]
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['beginner', 'setup', 'installation', 'articles', 'install', 'kaboom', 'minutes']
---

If this is your first time using **KaBOOM Agentic Devtools**, this guide is the fastest way to get running.

No jargon. Just the steps.

<!-- more -->

## Quick Terms

- **CLI (Command Line Interface):** A text-based way to run commands.
- **MCP (Model Context Protocol):** A standard that lets AI tools talk to external tools. https://modelcontextprotocol.io/specification/

## What You Will Have at the End

- KaBOOM installed
- Chrome extension loaded
- Your first successful health check

## Step 1: Run the installer

Use one command based on your system.

### macOS or Linux

```bash
curl -sSL https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/install.sh | bash
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/install.ps1 | iex
```

## Step 2: Load the extension in Chrome

1. Open `chrome://extensions/`
2. Turn on **Developer mode**
3. Click **Load unpacked**
4. Select `~/KaboomAgenticDevtoolExtension`

## Step 3: Verify installation

Run:

```bash
~/.kaboom/bin/kaboom-agentic-browser --doctor
```

You want to see a healthy result, not missing dependency errors.

## Step 4: Do a first real check

Open any webpage and try a simple call from your AI tool:

```js
observe({what: "errors"})
```

If it returns data, you are live.

## Common Fixes

- Installer command fails: retry with stable internet and no VPN/proxy blocking GitHub.
- Extension does not appear: reload extensions page and confirm Developer mode is on.
- Tool cannot connect: rerun `--doctor` and confirm local daemon is running.

## Image and Diagram Callouts

> [Image Idea] Installer command shown in terminal on macOS, Windows, and Linux.

> [Image Idea] Chrome extension "Load unpacked" step.

> [Diagram Idea] Install flow: install binary -> load extension -> run doctor -> first observe call.
