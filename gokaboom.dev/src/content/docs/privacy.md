---
title: Privacy & Data Collection
description: Kaboom Privacy Policy — what we collect, what we don't, and how to opt out
last_verified_version: 0.9.0
last_verified_date: 2026-04-19
normalized_tags: ['privacy']
---

# Privacy & Data Collection

**Last updated:** April 19, 2026

## TL;DR

**Your browser data stays on your machine.** Console logs, network requests, screenshots, and everything Kaboom captures from your browser never leaves localhost.

**We collect anonymous usage metrics** — which tools you use and how often — to improve Kaboom. No personal information, no URLs, no code. You can opt out with one environment variable.

---

## Two Separate Things

Kaboom handles two types of data very differently:

### 1. Browser Telemetry (YOUR data — stays local)

Everything Kaboom captures from your browser stays on your machine:

- Console logs, errors, exceptions
- Network requests and responses
- WebSocket events
- User interactions (clicks, form submissions)
- Screenshots (when requested)
- DOM snapshots
- Performance metrics

**Where it goes:** `http://localhost:7890` only. The Kaboom server runs on YOUR machine. We cannot access this data.

**Verification:** Check browser DevTools Network tab — you'll only see `localhost:7890` requests.

### 2. Anonymous Product Metrics (OUR data — sent to us)

We collect anonymous usage counters to understand how Kaboom is used:

| What we collect | Example | Why |
|----------------|---------|-----|
| Random install ID | `f7a2c1e9b4d8` | Correlate usage over time without identifying you |
| Tool usage and activation events | `first_tool_call`, `usage_summary` | Know which features matter and whether installs activate |
| OS and version | `darwin-arm64`, `0.8.1` | Know what to support |
| Error categories | `bridge_connection_error` | Fix common failure patterns |

**What we DON'T collect:**

| Never collected | Why not |
|----------------|---------|
| IP addresses | Not stored, not logged — our endpoint discards them |
| Your name, email, or identity | No accounts, no sign-up |
| URLs you're debugging | Not included in any beacon |
| Your code or file paths | Not included in any beacon |
| Error messages from your app | Could contain PII — only error *categories* sent |
| Screenshots or page content | Never leaves localhost |
| Project names or descriptions | Could identify you — excluded |
| Machine fingerprints | Install ID is pure random, not derived from hardware |

---

## The Install ID

When the Kaboom daemon first starts, it generates a random 12-character hex string (e.g., `f7a2c1e9b4d8`) and saves it at `~/.kaboom/install_id`. This is:

- **Randomly generated** — `crypto/rand`, not derived from your machine, username, or IP
- **Not reversible** — cannot be traced back to you
- **Used for** — "this install uses observe:errors a lot" not "this person does X"
- **Fail-closed** — if Kaboom cannot read or durably persist that file, it suppresses product metrics instead of minting a fresh replacement ID

---

## Usage Summary Beacon

When there is activity, Kaboom sends canonical daemon-owned events such as `first_tool_call`, `session_start`, `session_end`, `usage_summary`, and `app_error`.

One `usage_summary` event looks like:

```json
{
  "event": "usage_summary",
  "iid": "f7a2c1e9b4d8",
  "sid": "8510f6ce8ca743c2",
  "ts": "2026-04-19T08:10:00Z",
  "v": "0.8.5",
  "os": "darwin-arm64",
  "channel": "stable",
  "window_m": 5,
  "tool_stats": [
    {
      "tool": "observe:errors",
      "family": "observe",
      "name": "errors",
      "count": 12
    }
  ]
}
```

That's it. Tool names and counts. No URLs, no selectors, no content. If Kaboom is idle, no beacon is sent.

The installer and browser extension do not post product metrics directly; the daemon is the only analytics emitter.

---

## How to Opt Out

Set one environment variable:

```bash
export KABOOM_TELEMETRY=off
```

All beacons stop immediately. Kaboom works exactly the same — no features are degraded or locked.

Add it to your shell profile (`~/.zshrc`, `~/.bashrc`) to make it permanent.

---

## What We Automatically Redact (Browser Data)

Before browser telemetry reaches the localhost server, Kaboom redacts:

- **Passwords** → `[redacted]`
- **API keys, tokens, secrets** → `[redacted]`
- **Credit card numbers, SSNs** → `[redacted]`
- **Authorization headers, cookies** → `[redacted]`

This is defense-in-depth — the data never leaves your machine anyway, but we redact it before it even reaches the local server.

---

## Your Control

**Browser data (local):**
- You choose which tab to track (not automatic)
- You choose whether to enable AI Web Pilot (default: off)
- You choose whether to save logs to disk (default: in-memory only)
- Stop anytime — click "Stop Tracking" or uninstall

**Product metrics (remote):**
- Opt out anytime with `KABOOM_TELEMETRY=off`
- Delete your install ID: `rm ~/.kaboom/install_id`
- We cannot correlate your install ID to your identity

---

## Permissions Explained

### `tabs` Permission
**Why:** Track a specific browser tab
**What we DON'T do:** Track all tabs, read untracked tabs

### `storage` Permission
**Why:** Remember your settings across restarts
**What we store:** Tracked tab ID, toggle states, server URL

### `host_permissions` (localhost only)
**Why:** Send captured telemetry to your local Kaboom server
**What we DON'T do:** Access external websites, send data remotely

### Content Scripts (`<all_urls>`)
**Why:** Capture telemetry from any web app you're debugging
**What we do:** Inject into the ONE tab you explicitly track
**Why `<all_urls>`?** Kaboom is a developer tool — you need to debug apps on any domain.

---

## Compliance

### GDPR (EU)
- **Browser data:** Compliant — never leaves your device
- **Product metrics:** Compliant — no personal data collected. Random install ID is not PII (not linkable to a natural person). No IP addresses stored.
- **Right to deletion:** `rm ~/.kaboom/install_id` + `KABOOM_TELEMETRY=off`
- **Right to access:** All local data available via localhost API

### CCPA (California)
- No sale of personal information
- No personal data collected in product metrics

---

## Open Source

Everything is auditable:

- **Telemetry beacon code:** [internal/telemetry/beacon.go](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/blob/UNSTABLE/internal/telemetry/beacon.go)
- **Install ID generator:** [internal/telemetry/install_id.go](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/blob/UNSTABLE/internal/telemetry/install_id.go)
- **Telemetry endpoint:** [github.com/brennhill/kaboom-analytics](https://github.com/brennhill/kaboom-analytics) (the Cloudflare Worker that receives beacons)
- **Extension manifest:** [extension/manifest.json](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/blob/UNSTABLE/extension/manifest.json)
- **Redaction logic:** [extension/lib/serialize.js](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/blob/UNSTABLE/extension/lib/serialize.js)

**Don't take our word for it.** Read the source. The telemetry code is small and auditable. Every daemon beacon call site is searchable with `grep -rn 'BeaconEvent\|AppError\|fireStructuredBeacon'`.

---

## Changes to This Policy

We'll notify users via:
- GitHub release notes
- Extension update notes
- This page (check "Last updated" date)

---

## Contact

**Questions about privacy?**
- GitHub Issues: [github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/issues](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/issues)
- Email: [privacy@gokaboom.dev](mailto:privacy@gokaboom.dev)
