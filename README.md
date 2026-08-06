> ## Branch Policy (Read First)
> If you want something working, load code and run the server from `STABLE`.
> `UNSTABLE` makes zero promises on regressions or issues and is treated as work in progress.
> Stable builds are compressed, tagged, and moved to `STABLE`.

<div align="center">

<img src="docs/assets/images/chrome_store/branding/readme-banner-kaboom.svg" alt="KaBOOM! — Browser debugging, inspection, and verification for AI coding assistants via MCP" width="100%" />

[![License](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-0.9.0-green.svg)](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/releases)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8.svg?logo=go&logoColor=white)](https://go.dev/)
[![Chrome](https://img.shields.io/badge/Chrome-Manifest%20V3-4285F4.svg?logo=googlechrome&logoColor=white)](https://developer.chrome.com/docs/extensions/mv3/)
[![macOS](https://img.shields.io/badge/macOS-supported-000000.svg?logo=apple&logoColor=white)](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP)
[![Linux](https://img.shields.io/badge/Linux-supported-FCC624.svg?logo=linux&logoColor=black)](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP)
[![Windows](https://img.shields.io/badge/Windows-supported-0078D6.svg?logo=windows&logoColor=white)](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP)
[![Codacy Badge](https://app.codacy.com/project/badge/Grade/62158fcb044348c3bc51942787a9a535)](https://app.codacy.com/gh/brennhill/Kaboom-Browser-AI-Devtools-MCP/dashboard?utm_source=gh&utm_medium=referral&utm_content=&utm_campaign=Badge_grade)
[![Snyk Status](https://snyk.io/test/github/brennhill/Kaboom-Browser-AI-Devtools-MCP/badge.svg)](https://snyk.io/test/github/brennhill/Kaboom-Browser-AI-Devtools-MCP)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/pulls)
[![X Follow](https://img.shields.io/badge/follow-%40gokaboomdev-000000.svg?logo=x&logoColor=white)](https://x.com/gokaboomdev)
[![Fueling rapid development with AI](https://img.shields.io/badge/fueling%20rapid%20development%20with-AI-FF6B6B.svg)](https://gokaboom.dev)

**Kaboom is an AI debugger, inspector, and verification toolkit for local-first browser development workflows.** Stream console logs, network failures, exceptions, recordings, and browser evidence into any MCP-compatible coding assistant.

[Documentation](https://gokaboom.dev) •
[Quick Start](https://gokaboom.dev/getting-started/) •
[Features](https://gokaboom.dev/features/) •
[MCP Setup](https://gokaboom.dev/mcp-integration/)

</div>

---

<div align="center">

## 📦 Latest Release

Current version: **v0.9.0** — Structured telemetry, session analytics, KaBOOM! branding, and contract-compliant metrics reporting.

**macOS / Linux:**
```bash
curl -sSL https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/install.ps1 | iex
```

</div>

---

## Quick Start

**Fire up Kaboom (binary + extension + auto-config) in one command:**

**macOS / Linux:**
```bash
curl -sSL https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/install.ps1 | iex
```

This script automatically:
1.  **Downloads** the latest stable binary for your platform.
2.  **Installs** the browser extension files to `~/KaboomAgenticDevtoolExtension` (override with `KABOOM_EXTENSION_DIR`).
3.  **Auto-configures** all detected MCP clients (Claude Code, Cursor, Windsurf, Zed, etc.).

---

### Step 1: Finalize Browser Extension

1. Open `chrome://extensions`
2. Enable **Developer mode** (top right)
3. Click **Load unpacked**
4. Select the folder: `~/KaboomAgenticDevtoolExtension` (or wherever the script printed)

### Step 2: Restart Your AI Tool

Restart Claude Code, Cursor, Windsurf, or Zed. The Kaboom server will now start automatically when needed.

**[Full setup guide →](https://gokaboom.dev/getting-started/)** | **[Per-tool install guide →](docs/mcp-install-guide.md)**

### Uninstall

Clean removal of everything the installer created (binaries, extension files, autostart registration, PATH entries, MCP client configs, managed skills):

**macOS / Linux:**
```bash
curl -sSL https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/uninstall.sh | bash -s -- --yes
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/uninstall.ps1 -OutFile kaboom-uninstall.ps1; ./kaboom-uninstall.ps1 -Yes
```

Useful flags: `--dry-run` (preview), `--keep-data` (keep logs/recordings/project state). Afterwards, remove the extension from `chrome://extensions` — the script can't click browser UI for you.

---

## Why Teams Use Kaboom

**No debug port required.** Other tools need Chrome launched with `--remote-debugging-port`, which disables security sandboxing and breaks your normal browser workflow. Kaboom uses a standard extension, so your browser stays secure and unmodified.

**Single binary, zero runtime.** One Go binary that runs anywhere — no runtime dependencies, no Puppeteer, no framework.

**Captures what others can't.** WebSocket messages, full request/response bodies, user action recording, Web Vitals, automatic regression detection, visual annotations, and Playwright test generation from real browser sessions — features no other MCP browser tool offers.

**Works with every MCP tool.** Claude Code, OpenAI Codex, Cursor, Windsurf, Zed, Claude Desktop, VS Code + Continue. Switch AI tools without changing your debugging setup.

**Enterprise-safe by design.** Binds to `127.0.0.1` only. Auth headers are stripped automatically. No accounts, no cloud. Anonymous usage stats only (see Privacy). Audit the source — it's AGPL-3.0.

## What It Does

- **Console logs** — `console.log()`, `.warn()`, `.error()` with full arguments
- **Network errors** — Failed API calls (4xx, 5xx) with response bodies
- **Exceptions** — Uncaught errors with full stack traces
- **WebSocket events** — Connection lifecycle and message payloads
- **Network bodies** — Request/response payloads for API debugging
- **User actions** — Click, type, navigate, scroll recording with smart selectors
- **Web Vitals** — LCP, CLS, INP, FCP with regression detection
- **DOM inspection** — Query the page with CSS selectors via MCP
- **Accessibility audits** — WCAG checks with SARIF export
- **Security audits** — Credentials, PII, headers, cookies, third-party analysis
- **Browser automation** — Click, type, select, upload, navigate with semantic selectors
- **Visual annotations** — Draw mode for user feedback with computed style extraction
- **Test generation** — Playwright tests from context, self-healing selectors, failure classification
- **Reproduction scripts** — Playwright scripts from recorded user actions
- **Noise filtering** — Auto-detect and dismiss irrelevant errors
- **Developer API** — `window.__kaboom.annotate()` for custom context

**[Full feature list →](https://gokaboom.dev/features/)**

## Privacy

All captured data (logs, network, actions) stays 100% local — nothing leaves your machine. No cloud, no accounts.

We collect anonymous usage statistics (tool call frequency, session duration, error rates) using a random install identifier not linked to your identity. No URLs, prompts, file contents, browsing data, or personal information is collected. Disable with `KABOOM_TELEMETRY=off`.

**[Privacy details →](https://gokaboom.dev/privacy/)**

## Performance

See [latest benchmarks](docs/benchmarks/latest-benchmark.md) for current performance data.

Last benchmarked: 2026-02-09 on darwin/arm64

## Known Issues

See [docs/core/known-issues.md](docs/core/known-issues.md) for current known issues.

## Development

```bash
make test                              # Go server tests
node --test tests/extension/*.test.js  # Extension tests
make dev                               # Build for current platform
```

**[Release process & quality gates →](docs/core/release.md)** · **[Changelog →](CHANGELOG.md)**

## License

**AGPL-3.0** — Free and open source for all use cases.

Artwork, logos, and the Sparky mascot are **Copyright (c) Brenn Hill** and are not covered by the AGPL. See [LICENSE-ARTWORK](LICENSE-ARTWORK) for details.

---

<div align="center">

<img src="docs/assets/images/sparky-wave.png" alt="Sparky the Salamander" width="120" />

**[gokaboom.dev](https://gokaboom.dev)**

*Fueling rapid development with AI*

If you find Kaboom useful, please consider giving it a star.

[![Star on GitHub](https://img.shields.io/github/stars/brennhill/Kaboom-Browser-AI-Devtools-MCP.svg?style=social)](https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP)

</div>
