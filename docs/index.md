---
title: "Browser Observability for AI Coding Agents"
description: "Autonomously debug and fix issues in real time. Streams console logs, network errors, and exceptions to Claude Code, Copilot, Cursor, or any MCP-compatible assistant."
layout: home
permalink: /
status: reference
last_reviewed: 2026-08-06
---

<div class="hero">
  <div class="hero-content">
    <h1>Browser Observability for AI Coding Agents</h1>
    <p>Streams console logs, network errors, and exceptions to Claude Code, Copilot, Cursor, or any MCP-compatible assistant. Enterprise ready.</p>
    <p><em style="color: #fb923c;">Pouring fuel on the AI development fire</em></p>
    <p><a href="/setup/getting-started/" class="btn">Get Started</a></p>
  </div>
  <img src="/assets/images/product/sparky-working-laptop-web.webp" alt="Sparky coding" class="hero-sparky">
</div>

## Now You're Cooking

<div style="display: flex; align-items: center; gap: 1rem; margin-bottom: 2rem;">
  <img src="/assets/images/character/sparky-wave-web.webp" alt="Sparky waving" style="width: clamp(40px, 10vw, 80px); height: auto; border-radius: 6px;">
  <span style="font-size: 1.1em; color: #fb923c;">Welcome! Sparky is here to help you fire up your AI-powered browser debugging.</span>
</div>

One command. Your AI agent can see your browser.

```bash
npx kaboom-mcp
```



Kaboom is a **browser extension + local MCP server** that streams real-time browser data to autonomous coding agents. Console errors, failed API calls, uncaught exceptions, WebSocket traffic, live DOM state — your AI sees it all and fixes issues without you lifting a finger.

## Smart Teams Use KaBOOM!

<span style="color: #22c55e;">No more copy-paste debugging. Just code, fix, and ship.</span>

**No debug port required.** Other tools need Chrome launched with `--remote-debugging-port`, which disables security sandboxing and breaks your normal browser workflow. Kaboom uses a standard extension — your browser stays secure and unmodified.

**Single binary, zero runtime.** No Node.js, no Python, no Puppeteer, no package.json. One Go binary that runs anywhere. No supply chain risk. No `node_modules`.

**Captures what others can't.** WebSocket messages, full request/response bodies, user action recording, Web Vitals with regression detection, API schema inference, and Playwright test generation from real browser sessions — features no other MCP browser tool offers.

**Works with every MCP tool.** Claude Code, Cursor, Windsurf, Zed, Claude Desktop, VS Code + Continue. Switch AI tools without changing your debugging setup.

**Enterprise-safe by design.** Binds to `127.0.0.1` only. Auth headers are stripped automatically. No telemetry, no accounts, no cloud. Audit the source — it's AGPL-3.0.

## How Kaboom Compares

| | Kaboom | Chrome DevTools MCP | BrowserTools MCP | Cursor Browser |
|---|:---:|:---:|:---:|:---:|
| **Console logs** | ✅ | ✅ | ✅ | ✅ |
| **Network errors** | ✅ | ✅ | ✅ | ❌ |
| **Network bodies** | ✅ | ❌ | ❌ | ❌ |
| **WebSocket events** | ✅ | ❌ | ❌ | ❌ |
| **User action recording** | ✅ | ❌ | ❌ | ❌ |
| **DOM queries** | ✅ | ✅ | ✅ | ✅ |
| **Screenshots** | ✅ | ✅ | ✅ | ✅ |
| | | | | |
| **[Web Vitals](features/feature/web-vitals/)** | ✅ LCP, CLS, INP, FCP | ❌ | ❌ | ❌ |
| **[Regression detection](features/feature/push-regression/)** | ✅ Automatic | ❌ | ❌ | ❌ |
| **[API schema inference](features/feature/api-schema/)** | ✅ OpenAPI from traffic | ❌ | ❌ | ❌ |
| **[Accessibility audits](features/feature/sarif-export/)** | ✅ WCAG + SARIF | ❌ | ❌ | ❌ |
| **[Session checkpoints](features/feature/persistent-memory/)** | ✅ Named + auto | ❌ | ❌ | ❌ |
| **[Noise filtering](features/feature/noise-filtering/)** | ✅ Auto-detect | ❌ | ❌ | ❌ |
| | | | | |
| **[Test generation](features/feature/test-generation/)** | ✅ Playwright | ❌ | ❌ | ❌ |
| **[Reproduction scripts](features/feature/reproduction-scripts/)** | ✅ From actions | ❌ | ❌ | ❌ |
| **[PR summaries](features/feature/pr-preview-exploration/)** | ✅ Perf impact | ❌ | ❌ | ❌ |
| **[HAR export](features/feature/har-export/)** | ✅ | ❌ | ❌ | ❌ |
| | | | | |
| **Zero dependencies** | ✅ Single Go binary | ❌ Node.js + Chrome flags | ❌ Node.js + Puppeteer | ❌ Electron |
| **Vendor neutral** | ✅ Any MCP tool | ⚠️ Any MCP tool | ⚠️ Any MCP tool | ❌ Cursor only |
| **No debug port** | ✅ | ❌ `--remote-debugging-port` | ❌ `--remote-debugging-port` | N/A |
| **Privacy** | ✅ Localhost only | ✅ Local | ⚠️ Optional cloud | ❌ Cursor servers |
| **Performance overhead** | < 0.1ms | ~5ms | ~5ms | Unknown |
| **[Token efficiency](setup/token-efficiency.md)** | ~9,750 (5 tools) | ~30,000+ (20+ tools) | ~25,000+ (15+ tools) | Unknown |

[Project overview →](README.md)

## Absurdly Token Efficient

Every MCP tool your agent loads eats into its context window. Most browser MCP tools register 15–30 individual tools, each burning ~1,500 tokens of schema. That's **30,000+ tokens gone** before a single call.

Kaboom loads **5 tools in ~9,750 tokens** — full browser observability, automation, test generation, accessibility auditing, and more. Resources are lazy-loaded and cost zero tokens until read.

| | Kaboom | Typical 20-Tool MCP |
|---|---:|---:|
| **Init (schemas)** | ~9,750 | ~30,000+ |
| **Debug cycle (5 calls)** | ~11,000 | ~35,000+ |
| **With summary mode** | ~10,400 | N/A |

A complete debug-and-fix cycle — error triage, DOM inspection, code fix, reload, verification — stays **under 15,000 tokens total**. Enable `summary=true` and responses shrink another 60–70%.

Fewer tokens = more room for your codebase, faster responses, lower cost.

[Token efficiency deep dive →](setup/token-efficiency.md)

## Enterprise Ready — Zero Data Leakage

<div class="section-with-sparky">
<img src="/assets/images/safety/sparky-shield-web.webp" alt="Sparky with shield" class="section-sparky">
<div>

**No browser data is ever shared with any AI provider.** Kaboom runs entirely on your machine:

- **Localhost only** — the server binds to `127.0.0.1`, unreachable from the network
- **No cloud, no accounts, no telemetry** — nothing phones home, ever
- **Auth headers stripped** — tokens and API keys are automatically redacted
- **Open source (AGPL-3.0)** — audit every line your security team cares about

Your browser logs stay on your hardware. The AI reads a local file via stdio. At no point does debugging data touch a third-party server — making Kaboom safe for regulated environments, proprietary codebases, and enterprise security policies.

[Full Security Details →](/security/)

</div>
</div>

## Ecosystem Neutral — No Vendor Lock-In

Kaboom implements the open **[Model Context Protocol](https://modelcontextprotocol.io/)** standard. Swap AI tools without changing your debugging setup:

- **[Claude Code](/mcp-integration/claude-code/)** — `.mcp.json` in project root
- **[OpenAI Codex](/mcp-integration/codex/)** — `~/.codex/config.toml`
- **[Cursor](/mcp-integration/cursor/)** — `~/.cursor/mcp.json`
- **[Windsurf](/mcp-integration/windsurf/)** — `~/.codeium/windsurf/mcp_config.json`
- **[Claude Desktop](/mcp-integration/claude-desktop/)** — OS-specific config
- **[Zed](/mcp-integration/zed/)** — `~/.config/zed/settings.json`
- **VS Code + Continue** — `~/.continue/config.json`

Not tied to Anthropic. Not tied to Cursor. Not tied to anyone. If your agent speaks MCP, Kaboom fuels it.

## The Pipeline

```
[ Browser ] → [ Extension ] → [ localhost:7890 ] → [ Any MCP AI ]
                                    ↕
                              Stays on your machine
```

1. The extension passively captures your browser activity
2. Data flows to a local server — never the internet
3. Your AI tool reads it via MCP (stdio, not network)
4. **Nothing leaves your machine** — private by architecture

## What Gets Captured

- **Console Logs** — `console.log()`, `.warn()`, `.error()` with full arguments
- **Network Errors** — Failed API calls (4xx/5xx) with response bodies
- **Exceptions** — Uncaught errors with full stack traces
- **[WebSocket Events](features/capture/network-capture.md)** — Connection lifecycle and message payloads
- **[Network Bodies](features/capture/network-capture.md)** — Request/response payloads for API debugging
- **User Actions** — Click, type, navigate, scroll recording with smart selectors
- **Web Vitals** — LCP, CLS, INP, FCP with automatic regression detection
- **[Live DOM](features/feature/query-dom/)** — Query the page with CSS selectors via MCP
- **[Accessibility](features/feature/sarif-export/)** — WCAG audits with SARIF export
- **API Schema Inference** — Auto-discover OpenAPI from captured traffic
- **Session Checkpoints** — Save state, diff changes, detect regressions over time
- **[Test Generation](features/feature/test-generation/)** — Playwright tests and reproduction scripts from actions
- **Noise Filtering** — Auto-detect and dismiss irrelevant errors
- **[Context API](features/feature/custom-event-api/)** — Annotate errors with `window.__kaboom`

## Zero Bloat, Zero Risk

- **Single binary** — Go. No Node.js, no Python, no runtime deps.
- **< 0.1ms overhead** — Per console intercept. Your browsing stays fast.
- **20MB memory cap** — The extension never bloats your browser.
- **Zero network calls** — The binary never connects to the internet.
- **No dependencies** — No supply chain risk. One binary, auditable.

---

<div class="cta-footer">
  <img src="/assets/images/safety/sparky-firefighter-happy-web.webp" alt="Sparky ready to help" class="cta-sparky">
  <div>
    <h3>Ready to Fire It Up?</h3>
    <p><a href="/setup/getting-started/" class="btn">Get Started</a></p>
  </div>
</div>
