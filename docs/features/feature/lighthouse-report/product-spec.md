---
feature: lighthouse-report
status: proposed
version: null
tool: analyze
mode: lighthouse_report
authors: []
created: 2026-03-06
updated: 2026-03-06
doc_type: product-spec
feature_id: feature-lighthouse-report
last_reviewed: 2026-07-05
---

# Lighthouse Report

> Run a real Lighthouse audit via CDP and return structured results for AI consumption.

## Problem

Kaboom's existing performance tooling (`observe(what="vitals")`, the proposed `generate(what="performance_audit")`) analyzes passively captured telemetry. This gives good heuristic analysis, but it's not the same as a real Lighthouse audit — the industry-standard benchmark that developers, CI pipelines, and stakeholders actually reference.

Chrome DevTools MCP already exposes `lighthouse_audit` as a first-class tool. This is a gap: an AI agent using Kaboom cannot run a Lighthouse audit without also installing Chrome DevTools MCP.

## Solution

Add `analyze(what="lighthouse_report")` which runs a real Lighthouse audit via Chrome DevTools Protocol. The extension attaches the debugger to the tracked tab, invokes Lighthouse through CDP, and returns structured results (scores, audits, opportunities) as JSON.

This differs from the proposed `performance_audit` feature:
- **`generate(what="performance_audit")`** — Kaboom's own static analysis of captured telemetry. Fast (<50ms server-side), always available, no debugger attachment needed.
- **`analyze(what="lighthouse_report")`** — Real Lighthouse. Reloads the page, runs synthetic benchmarks, produces industry-standard scores. Slower (10-30s), requires debugger attachment.

Both are valuable. The performance audit is great for iterative development (fast feedback). The Lighthouse report is what you run before shipping (authoritative benchmark).

## MCP Interface

**Tool:** `analyze`
**Mode:** `lighthouse_report`

### Request

```json
{
  "tool": "analyze",
  "arguments": {
    "what": "lighthouse_report",
    "categories": ["performance", "accessibility", "best-practices", "seo"],
    "device": "mobile",
    "mode": "navigation"
  }
}
```

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `what` | string | yes | -- | Must be `"lighthouse_report"` |
| `categories` | string[] | no | all | Which audit categories to run. Valid: `performance`, `accessibility`, `best-practices`, `seo` |
| `device` | string | no | `"desktop"` | Device emulation: `"desktop"` or `"mobile"` |
| `mode` | string | no | `"navigation"` | `"navigation"` reloads and audits. `"snapshot"` audits current state without reload. |

### Response

```json
{
  "lighthouse_report": {
    "url": "https://example.com",
    "fetch_time": "2026-03-06T14:30:00Z",
    "device": "mobile",
    "mode": "navigation",
    "scores": {
      "performance": 72,
      "accessibility": 95,
      "best-practices": 88,
      "seo": 91
    },
    "metrics": {
      "fcp_ms": 1850,
      "lcp_ms": 3200,
      "cls": 0.18,
      "tbt_ms": 450,
      "si_ms": 2100,
      "tti_ms": 4500
    },
    "opportunities": [
      {
        "id": "render-blocking-resources",
        "title": "Eliminate render-blocking resources",
        "savings_ms": 800,
        "details_summary": "3 resources are blocking first paint"
      }
    ],
    "diagnostics": [
      {
        "id": "dom-size",
        "title": "Avoid an excessive DOM size",
        "value": "2,847 elements"
      }
    ],
    "passed_audits_count": 42,
    "duration_ms": 15200
  }
}
```

## Implementation Approach

### Extension Side (CDP)

The extension already has the `debugger` permission and `chrome.debugger.attach/sendCommand/detach` lifecycle in `cdp-dispatch.ts`. Lighthouse can be invoked through CDP by:

1. Attaching debugger to the tracked tab
2. Using the Lighthouse Node library's CDP connection mode, OR
3. Running Lighthouse categories natively through CDP protocol domains (Page, Performance, Accessibility, etc.)

**Option A (recommended):** Shell out to `lighthouse` CLI from the Go daemon with `--output=json --chrome-flags="--remote-debugging-port=PORT"`. The daemon already has the tab's debugger URL. This avoids bundling Lighthouse into the extension and keeps the extension's zero-deps constraint.

**Option B:** Use CDP domains directly in the extension to replicate Lighthouse's audits. Higher effort, but no external dependency. This is essentially what the `performance_audit` feature already proposes for a subset of categories.

**Recommendation:** Option A for a quick ship. The daemon invokes Lighthouse CLI against the already-debuggable tab, parses the JSON output, and returns a structured subset. The extension's role is just ensuring the debugger port is accessible.

### Go Side (Daemon)

- New handler in `tools_analyze.go` dispatch for `what="lighthouse_report"`
- Invokes Lighthouse CLI as a subprocess (similar pattern to how other tools invoke external processes)
- Parses Lighthouse JSON report, extracts scores/metrics/opportunities/diagnostics
- Returns structured response trimmed for token efficiency (full Lighthouse reports are 100KB+; we return only actionable data)

### Timeout and Lifecycle

- Lighthouse navigation audits take 10-30 seconds
- Must use the existing async command pattern: daemon sends the command, extension/CLI executes, result is polled via `observe(what="command_result")`
- Timeout: 60s (configurable)
- If the debugger is already attached (e.g., DevTools is open), return a clear error with recovery action

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | Run Lighthouse audit against the tracked tab | must |
| R2 | Return scores for performance, accessibility, best-practices, seo | must |
| R3 | Return Core Web Vitals metrics (FCP, LCP, CLS, TBT, SI, TTI) | must |
| R4 | Return top opportunities with estimated savings | must |
| R5 | Return diagnostics (informational audits) | should |
| R6 | Support category filtering (run only selected categories) | should |
| R7 | Support mobile/desktop device emulation | should |
| R8 | Support navigation vs. snapshot mode | should |
| R9 | Trim response to actionable data (<5KB typical) for token efficiency | must |
| R10 | Clear error when debugger cannot attach | must |
| R11 | Async execution with polling for results | must |

## Non-Goals

- Not a replacement for the proposed `generate(what="performance_audit")` — that feature provides fast heuristic analysis from captured telemetry. This feature provides authoritative Lighthouse benchmarks.
- Not exposing the full Lighthouse JSON report (~100KB). The response is trimmed to scores, metrics, top opportunities, and diagnostics.
- Not running Lighthouse in CI mode. This is for interactive development — the AI runs it against the live browser.

## Dependencies

- `debugger` permission (already in manifest)
- CDP attach/detach lifecycle (already in `cdp-dispatch.ts`)
- Async command infrastructure (shipped)
- Lighthouse CLI available in PATH (user's responsibility; common in Node.js environments)

## Competitive Context

Chrome DevTools MCP exposes `lighthouse_audit` with navigation/snapshot modes and desktop/mobile emulation. Adding this to Kaboom closes the last major capability gap for auditing.
