---
title: "Privacy & Security"
description: "Kaboom keeps captured browser data local and sends only anonymous product-command usage telemetry. Logs never leave your machine. Auth headers are automatically stripped."
keywords: "local browser debugging, privacy first developer tools, anonymous usage telemetry, localhost debugging tool, secure browser debugging"
permalink: /privacy/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Your browser data stays local. Anonymous product usage helps improve Kaboom."
toc: true
toc_sticky: true
status: reference
last_reviewed: 2026-07-30
---

Kaboom is designed with privacy as a core principle, not an afterthought.

## <i class="fas fa-home"></i> Browser Data Stays Local

- **Logs never leave your machine** — everything stays on localhost
- **No cloud services** — no accounts, no sign-ups, no data uploads
- **No browser-data analytics** — URLs, prompts, page content, file contents,
  captured logs/network data, credentials, and personal data are never sent
- **Local product server** — browser capture and MCP traffic bind to `127.0.0.1`

## <i class="fas fa-chart-line"></i> Anonymous Product Usage

Kaboom sends a narrow product-usage envelope to `t.gokaboom.dev` so we can
measure install activity and learn which product commands are used. It contains
random install/session identifiers, version/platform, command identifiers,
outcomes, timing, and aggregate counts. It does not contain captured browser or
user data.

Disable product telemetry with `KABOOM_TELEMETRY=off`.

## <i class="fas fa-shield-alt"></i> Sensitive Data Protection

- **Authorization headers stripped** — tokens, API keys, and bearer tokens are automatically removed from captured network logs
- **No cookie capture** — cookies are not included in log entries
- **No form values by default** — input values in user actions are redacted unless explicitly enabled

## <i class="fas fa-lock"></i> Localhost Only

The Kaboom server binds exclusively to `127.0.0.1`:

- Not accessible from your local network
- Not accessible from the internet
- Other devices on your WiFi cannot reach it
- Firewall rules are not required

## <i class="fab fa-github"></i> Open Source

The entire codebase is open source under AGPL-3.0:

- **Audit the code** — verify exactly what gets captured and where it goes
- **Build from source** — compile the Go binary yourself
- **No obfuscation** — extension code is vanilla JavaScript, readable in Chrome DevTools

## <i class="fas fa-recycle"></i> Data Lifecycle

1. Browser extension captures events in-page
2. Events are sent to `localhost:7890` via HTTP POST
3. Server appends entries to a local JSONL file
4. Your AI tool reads the file via MCP (stdio, not network)
5. Log rotation removes old entries automatically

Captured browser and user data never leaves your machine. Only the anonymous
product-usage envelope described above is transmitted externally.

## <i class="fas fa-key"></i> Extension Permissions

The Chrome extension requests only the minimum permissions needed:

- **activeTab** — to inject capture scripts into the current tab
- **storage** — to persist extension settings locally
- **Host permission (localhost)** — to communicate with the local server

No permissions for browsing history, bookmarks, downloads, or cross-origin requests.
