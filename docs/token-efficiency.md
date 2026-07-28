---
title: "Token Efficiency — Why Your MCP Tools' Footprint Matters"
description: "Kaboom loads in ~9,750 tokens — less than most MCP tools use on a single schema. See baseline metrics, per-workflow costs, and optimization tips."
keywords: "MCP token usage, token efficiency, AI context window, MCP browser tools comparison, kaboom token cost"
permalink: /token-efficiency/
toc: true
toc_sticky: true
status: reference
last_reviewed: 2026-03-06
---

<img src="/assets/images/sparky-wave-web.webp" alt="Sparky" style="float: right; width: 100px; margin: 0 0 1rem 1.5rem; border-radius: 6px;">

Every token your MCP tools consume is a token your AI agent can't use for reasoning, code generation, or understanding your codebase. Most developers don't think about this — but when an MCP server eats 30,000+ tokens just to register its tools, you've lost a meaningful chunk of your context window before anything happens.

Kaboom was designed from the ground up to minimize its token footprint while maximizing the data your agent can access.

---

## Baseline: What Kaboom Costs on Init

When your AI agent connects to Kaboom, it loads server instructions and five tool schemas. That's it — resources are lazy-loaded and don't consume tokens until explicitly read.

| Component | Chars | ~Tokens |
|---|---:|---:|
| Server instructions | 2,392 | ~600 |
| `observe` schema | 5,764 | ~1,440 |
| `analyze` schema | 5,302 | ~1,325 |
| `generate` schema | 5,349 | ~1,337 |
| `configure` schema | 7,972 | ~1,993 |
| `interact` schema | 12,218 | ~3,054 |
| **Total** | **39,007** | **~9,750** |

**~9,750 tokens.** That's the entire cost of having full browser observability, automation, test generation, accessibility auditing, performance analysis, and more — available to your AI agent.

### Why five tools instead of thirty?

Most MCP browser tools register 15–30 individual tools: one for screenshots, one for console logs, one for clicking, one for typing, one for network requests, and so on. Each tool schema costs ~1,000–2,000 tokens. Twenty tools at 1,500 tokens each = **30,000 tokens** — three times Kaboom's total — just for the schemas.

Kaboom uses a **five-tool architecture** where each tool dispatches on a `what` parameter. This isn't a limitation — it's a deliberate design choice. The `what` parameter's enum values tell the AI exactly what's available, and the conditional schema descriptions guide parameter usage. The result: full capability at one-third the token cost.

---

## Per-Workflow Token Costs

Init cost is only part of the story. Every tool call produces a response that consumes tokens. Here's what typical workflows cost in practice.

### Scenario 1: Fix a CSS Bug (Design Tweaks)

A typical visual debugging flow:

| Step | Tool Call | ~Response Tokens |
|---:|---|---:|
| 1 | `observe(what="screenshot")` | 200–400 |
| 2 | `analyze(what="dom", query=".broken-element")` | 300–600 |
| 3 | *Agent edits CSS* | — |
| 4 | `interact(what="refresh")` | 150–300 |
| 5 | `observe(what="screenshot")` | 200–400 |

**Total incremental: ~850–1,700 tokens** for a complete visual fix cycle.

### Scenario 2: Debug a Failing API Call

| Step | Tool Call | ~Response Tokens |
|---:|---|---:|
| 1 | `observe(what="error_bundles")` | 400–1,200 |
| 2 | `observe(what="network", url_filter="/api")` | 300–800 |
| 3 | *Agent identifies root cause, edits code* | — |
| 4 | `interact(what="refresh")` | 150–300 |
| 5 | `observe(what="network", url_filter="/api")` | 300–800 |

**Total incremental: ~1,150–3,100 tokens.** Error bundles pre-correlate errors with related network requests, logs, and user actions — so the agent gets full context in one call instead of making five separate requests.

### Scenario 3: Generate a Playwright Test

| Step | Tool Call | ~Response Tokens |
|---:|---|---:|
| 1 | `interact(what="screen_recording_start")` | 50–100 |
| 2 | *User performs actions in browser* | — |
| 3 | `interact(what="screen_recording_stop")` | 50–100 |
| 4 | `observe(what="actions")` | 200–600 |
| 5 | `generate(what="playwright_test")` | 500–1,500 |

**Total incremental: ~800–2,300 tokens** for a complete test from recorded user actions.

### Scenario 4: Full Debug Cycle (Complex Issue)

| Step | Tool Call | ~Response Tokens |
|---:|---|---:|
| 1 | `observe(what="error_bundles")` | 400–1,200 |
| 2 | `observe(what="logs", level="error")` | 200–600 |
| 3 | `analyze(what="dom", query="#app")` | 300–800 |
| 4 | `observe(what="network")` | 300–1,000 |
| 5 | *Agent identifies and fixes the issue* | — |
| 6 | `interact(what="refresh")` | 150–300 |
| 7 | `observe(what="error_bundles")` | 100–300 |
| 8 | `observe(what="screenshot")` | 200–400 |

**Total incremental: ~1,650–4,600 tokens** for a thorough debug-and-verify cycle.

### Summary: Total Session Costs

| Workflow | Tool Calls | Init + Incremental |
|---|---:|---:|
| Design tweak | 4–5 | ~10,600–11,450 |
| API debugging | 4–5 | ~10,900–12,850 |
| Test generation | 4–5 | ~10,550–12,050 |
| Full debug cycle | 7–8 | ~11,400–14,350 |

Even the most complex single-issue debug cycle stays **under 15,000 tokens total** — init included. That's less than half of what some MCP tools spend just loading their schemas.

---

## Optimization: The `summary` Flag

Every `observe` and `analyze` call accepts a `summary=true` parameter that returns compact responses — **60–70% smaller** than full output. Set it once per session:

```
configure(what="store", store_action="save", namespace="session",
          key="response_mode", data={"summary": true})
```

With summary mode, the scenarios above drop significantly:

| Workflow | Standard | With `summary=true` |
|---|---:|---:|
| Design tweak | ~10,600–11,450 | ~10,200–10,750 |
| API debugging | ~10,900–12,850 | ~10,400–11,400 |
| Full debug cycle | ~11,400–14,350 | ~10,600–11,850 |

Summary mode is especially effective for iterative workflows where the agent makes many observe calls. Over a multi-issue session with 20–30 tool calls, the savings compound to thousands of tokens.

---

## Why Token Efficiency Matters

### 1. Context window is finite

Claude's context window — whether 100K or 200K tokens — must fit your codebase, conversation history, tool schemas, and tool responses. Every token an MCP tool wastes is a token your agent can't use to understand your code.

### 2. Speed scales with tokens

More output tokens = longer response times. Compact responses from Kaboom mean your agent thinks faster and iterates quicker.

### 3. Cost scales with tokens

If you're on a usage-based plan, every token has a dollar cost. A 3x schema overhead compounds across every conversation, every developer, every day.

### 4. More headroom = better reasoning

AI agents perform better with more available context. When tools consume less, the agent has more room to hold your codebase in memory, reason about complex issues, and produce better fixes.

---

## Design Decisions That Reduce Token Cost

| Decision | Impact |
|---|---|
| **5-tool dispatch architecture** | ~9,750 tokens vs ~30,000+ for 20-tool alternatives |
| **Lazy-loaded resources** | Zero token cost until explicitly read |
| **`summary=true` mode** | 60–70% response size reduction |
| **Error bundles** | Pre-correlated context in one call vs 3–5 separate calls |
| **Noise filtering** | Auto-suppresses irrelevant errors before they reach the agent |
| **Pagination with cursors** | Large result sets paginated, not dumped |
| **Structured JSON responses** | Precise data vs. raw text or screenshot-only approaches |
| **`limit` parameter** | Cap returned elements on interactive queries |

---

## How Kaboom Compares on Token Cost

| | Kaboom | Typical 20-tool MCP Server |
|---|---:|---:|
| **Init (schemas)** | ~9,750 | ~30,000+ |
| **5-call debug cycle** | ~11,000–13,000 | ~15,000–25,000 |
| **With summary mode** | ~10,400–11,400 | N/A (most lack this) |
| **Session (10 calls)** | ~12,000–18,000 | ~35,000–55,000 |

The difference isn't marginal — it's **2–3x** across every metric. Over a full development session, that's tens of thousands of tokens saved for actual reasoning.

---

<div class="cta-footer">
  <div>
    <h3>Ready to Fire It Up?</h3>
    <p>One command. ~9,750 tokens. Full browser observability.</p>
    <p><a href="/getting-started/" class="btn">Get Started</a></p>
  </div>
</div>
