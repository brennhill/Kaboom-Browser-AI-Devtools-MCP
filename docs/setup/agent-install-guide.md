# Agent Install Guide

> **Audience:** LLM coding agents (Claude Code, Codex, Gemini CLI, etc.)
> **Purpose:** Step-by-step instructions an agent can follow to install Kaboom for a user with minimal human intervention.

You are installing **Kaboom Agentic Browser** — an MCP server + Chrome extension that gives you real-time browser telemetry (errors, logs, network, DOM, screenshots, automation).

## Prerequisites

- macOS, Linux, or Windows (WSL/Git Bash)
- Chrome or Chromium-based browser
- `curl` and `unzip` available in PATH

## Step 1: Run the installer

Run the one-liner. This downloads the binary, stages the extension, and configures all detected MCP clients automatically.

**macOS / Linux:**
```bash
curl -sSL https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/install.ps1 | iex
```

**What this does (no further action needed):**
- Downloads the platform binary to `~/.kaboom/bin/`
- Verifies SHA-256 checksum
- Extracts the Chrome extension to `~/KaboomAgenticDevtoolExtension/`
- Runs `--install` which auto-detects and configures: Claude Code, Claude Desktop, Cursor, Windsurf, VS Code, Gemini CLI, OpenCode, Antigravity, Zed
- Installs the canonical `kaboom-agentic-browser` command

**If the install script is unavailable**, fall back to npm:
```bash
npm install -g kaboom-agentic-browser && kaboom-agentic-browser --install
```

### Verify the install succeeded

Check the exit code was 0 and the binary exists:
```bash
~/.kaboom/bin/kaboom-agentic-browser --version
```

## Step 2: Ask the user to load the Chrome extension

This is the **one step that requires human interaction**. Chrome does not allow programmatic installation of unpacked extensions — the user must click through the Chrome UI.

**Tell the user:**

> To finish setup, load the Kaboom browser extension:
> 1. Open **chrome://extensions** in Chrome
> 2. Enable **Developer mode** (toggle in the top-right corner)
> 3. Click **Load unpacked**
> 4. Select the folder: **~/KaboomAgenticDevtoolExtension**
>
> The Kaboom icon should appear in your toolbar. Click it — it should show "Connected".

**Why this can't be automated:** `chrome://extensions` is a privileged Chrome-internal page. No external process, MCP tool, or browser automation framework can toggle Developer Mode or trigger "Load unpacked." This is a Chrome security boundary.

**Partial automation option:** If the user is willing to restart Chrome, the extension can be loaded via CLI flag without any UI clicks:
```bash
# macOS example — launches Chrome with the extension pre-loaded
open -a "Google Chrome" --args --load-extension="$HOME/KaboomAgenticDevtoolExtension"
```
Caveat: this only applies to that Chrome session. A full restart of Chrome without the flag will not retain the extension. For persistent installation, the manual Load Unpacked flow is required.

## Step 3: Restart the AI tool

The MCP config was written in Step 1, but the AI tool needs to restart to pick it up.

**Tell the user:**

> Restart your AI tool (quit and reopen Claude Code, Cursor, etc.) to activate the Kaboom Agentic Browser server.

For Claude Code specifically, no restart is needed if the installer used `claude mcp add-json` — it takes effect on the next conversation.

## Step 4: Verify end-to-end

Once the user confirms the extension is loaded and the AI tool is restarted, run the doctor command:

```bash
kaboom-agentic-browser --doctor
```

Expected output shows:
- Binary: OK with version number
- Port 7890: available (or in use by Kaboom)
- At least one client: status "ok"

Then verify the MCP connection is live by calling a Kaboom tool:

```
Use the observe tool with what: "page" to check if the extension is connected.
```

If the extension is connected, you'll get back page info (URL, title). If not, you'll get a message indicating no extension is connected — ask the user to check the extension is loaded and they have a tab open.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `--doctor` shows "binary not found" | PATH not updated | Run `export PATH="$HOME/.kaboom/bin:$PATH"` and add to shell profile |
| Port 7890 in use | Stale daemon | Run `kaboom-agentic-browser --stop --port 7890` then retry |
| Extension shows "Disconnected" | Daemon not running | The MCP client starts the daemon automatically — make sure the AI tool is running |
| `observe` returns no data | No tab open | User needs to have at least one Chrome tab open |
| Extension not visible in toolbar | Not pinned | User should click the puzzle-piece icon in Chrome toolbar and pin Kaboom |

## Summary

| Step | Who | Automatable? |
|------|-----|-------------|
| Download binary + verify checksum | Agent | Yes — `install.sh` handles it |
| Stage extension files | Agent | Yes — `install.sh` handles it |
| Configure MCP clients | Agent | Yes — `--install` handles it |
| Deploy agent skills | Agent | Yes — handled by postinstall |
| Load Chrome extension | **User** | **No** — Chrome security boundary |
| Restart AI tool | **User** | **No** — agent can't restart itself |
| Verify with `--doctor` | Agent | Yes |
