---
doc_type: product-spec
feature_id: feature-auto-fix
status: proposed
owners: []
last_reviewed: 2026-07-05
links:
  index: ./index.md
---

# Auto-Fix Product Spec

## TL;DR

- Problem: A developer can see that a page is broken but has no single, low-friction way to hand an AI agent a complete, prioritized picture of what is wrong. Existing primitives (console logs, network failures, accessibility violations, security findings) are scattered across separate tool calls.
- User value: One `Audit` action turns a tracked browser tab into a prioritized, product-shaped report — scores, top findings, fast wins, and ship blockers — that an AI agent can act on.
- Surfaces: `analyze(what="page_issues")` evidence sweep, the repo-owned `/kaboom/audit` command, the bundled `audit` skill, and the tracked-site `Audit` entrypoints (popup and hover launcher).

## Problem

When a developer notices something wrong on a page, the path to a fix is fragmented. The browser telemetry already exists — console errors, failed network requests, accessibility violations, security findings — but each lives behind its own tool call and its own output shape. The developer (or their AI agent) must know which tool to call, call each one, and then merge and prioritize the results by hand.

There is no single "tell me everything wrong with this page, ranked" affordance, and no shared trigger that behaves the same whether it is launched from the popup, the on-page hover surface, or an MCP tool call.

## Solution

Auto-fix is the Phase 1 audit workflow. It is built from one shared evidence primitive and one shared trigger:

1. **`analyze(what="page_issues")`** runs every detectable check against the tracked tab in parallel — console errors, network failures (4xx/5xx), accessibility violations, and security findings — and returns a unified, severity-tagged report. A `summary: true` mode collapses the report to the top findings for roughly an 80% token reduction.

2. **The `/kaboom/audit` command and `audit` skill** turn those raw primitives into one product-shaped report. They require a tracked site, run a six-lane review (Functionality, UX Polish, Accessibility, Performance, Release Risk, SEO), and return a polished local markdown report.

3. **Tracked-site `Audit` entrypoints** — the popup button and the hover-surface action — both call one shared `requestAudit` helper so the experience is identical regardless of entry point. The helper opens the terminal side panel, sends the existing `qa_scan_requested` runtime bridge, and falls back to a daemon-stored intent when terminal injection is unavailable.

The bundled `qa` skill is retained as a compatibility alias that redirects older QA requests into the same audit workflow.

## User Stories

- As a developer, I want one `Audit` button on a tracked tab so that I can hand my AI agent a complete picture of what is wrong without naming individual tools.
- As an AI coding agent, I want a single `page_issues` call that aggregates console, network, accessibility, and security findings so that I do not have to orchestrate four separate tools.
- As an AI agent working under a token budget, I want a `summary` mode so that I can triage the most important findings cheaply before pulling full detail.
- As a developer, I want the same audit behavior whether I start from the popup, the on-page hover launcher, or a slash command so that there is one mental model.

## Product Contract

### Evidence sweep — `analyze(what="page_issues")`

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `summary` | boolean | `false` | Collapse to the top findings for a large token reduction |
| `categories` | string[] | all | Subset of `console_errors`, `network_failures`, `accessibility`, `security` |
| `limit` | integer | 50 | Per-section cap on returned issues |

The response carries `total_issues`, a `by_severity` map, per-category `sections`, `checks_completed`, `checks_skipped`, the `page_url`, and a `timestamp`. Severities are normalized to a single scale (`critical`, `high`, `medium`, `low`, `info`) across all categories.

### Audit workflow output

The `/kaboom/audit` command and `audit` skill produce one local markdown report with a fixed structure: Overall Score, Lane Scores, Executive Summary, Top Findings, Fast Wins, Ship Blockers, and Coverage And Limits.

### Trigger contract

- Popup and hover entrypoints both call `requestAudit`.
- `requestAudit` opens the terminal panel first, then sends `qa_scan_requested`.
- If terminal injection is unavailable, the daemon stores a `qa_scan` intent and the next MCP tool response prepends an `ACTION REQUIRED` nudge pointing at `/kaboom/audit` (or `/audit` where namespaced commands are unsupported).
- User-facing copy is `Audit`, not `Find Problems`.

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | Aggregate console errors, network failures, accessibility, and security into one severity-tagged report | must |
| R2 | Require a tracked tab; return an actionable error with a recovery tool call when none is tracked | must |
| R3 | Run checks in parallel with a per-check timeout; report completed vs skipped checks | must |
| R4 | Support a `summary` mode that returns the top findings only | must |
| R5 | Support category and per-section limit parameters | should |
| R6 | One shared `requestAudit` trigger across popup and hover entrypoints | must |
| R7 | Terminal-first behavior, with a daemon-intent fallback and an `ACTION REQUIRED` nudge | must |
| R8 | One fixed report contract shared by the command and the bundled skill | must |
| R9 | Keep `qa` as a compatibility alias for the audit workflow | should |

## Non-Goals

- This is a local-only Phase 1 workflow: no watch mode, no history, no hosted reports, no team workflow.
- The workflow does not automatically apply code fixes; it produces a prioritized report for an agent or developer to act on.
- It does not replace the individual primitives (`observe`, `analyze`) — it composes them.
- It does not transmit any telemetry off the local machine.

## Assumptions

- A1: A tab is being tracked by the extension (required precondition for every page check).
- A2: The telemetry buffers (console, network, accessibility, security) reflect the current page state.
- A3: The terminal side panel is the primary delivery surface; the daemon intent store is the fallback.
- A4: The repo-owned command and bundled skill are kept aligned on one report structure.
