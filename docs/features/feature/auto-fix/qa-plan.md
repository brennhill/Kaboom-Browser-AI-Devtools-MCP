---
doc_type: qa-plan
feature_id: feature-auto-fix
status: proposed
owners: []
last_reviewed: 2026-08-07
links:
  index: ./index.md
  product: ./product-spec.md
  tech: ./tech-spec.md
relates-to: [product-spec.md, tech-spec.md]
tags: [testing, qa, audit]
---

# QA Plan: Auto-Fix (Audit Workflow)

> QA plan for the Phase 1 audit workflow: the `analyze(what="page_issues")` evidence sweep, the shared `requestAudit` trigger, the daemon-intent fallback, and the six-lane report. Covers data leak analysis, LLM clarity, simplicity, code-level testing, and step-by-step UAT.

---

## 1. Data Leak Analysis

**Goal:** The sweep reads browser telemetry and the workflow surfaces it to an agent. Verify everything stays on localhost and sensitive values are not over-exposed.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Telemetry transmitted off-host | The sweep reads already-captured buffers and returns them via MCP over localhost. Verify no external network calls are made by the workflow. | critical |
| DL-2 | Network bodies exposing secrets | `network_failures` returns URL, method, status, content type, and duration — not response bodies. Verify request/response payloads are not included. | high |
| DL-3 | Console messages containing PII | `console_errors` returns the message and stack trace. These may contain user data printed by the page. Document that this matches existing `observe(what="logs")` exposure and stays local. | medium |
| DL-4 | Security findings echoing credentials | The security checker returns `evidence` strings. Verify evidence is redacted/bounded and does not echo full secrets. | high |
| DL-5 | URL query parameters with tokens | Link/network URLs may carry tokens. Verify reporting does not amplify sensitive query strings beyond what the underlying tools already expose. | medium |
| DL-6 | Intent store persistence | The `qa_scan` intent persists a tracked URL for fallback pickup. Verify it stores only what is needed and stays local. | low |

### Negative Tests (must NOT leak)

- [ ] The workflow makes zero outbound (non-localhost) network connections.
- [ ] `network_failures` entries omit request and response bodies.
- [ ] Security `evidence` is bounded and does not contain full secret values.
- [ ] All telemetry travels over the local daemon only (127.0.0.1).

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent can act on the sweep and the report without misreading severity, coverage, or next steps.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Unified severity scale | All sections use `critical/high/medium/low/info`; the agent can rank across categories using one scale. | [ ] |
| CL-2 | Coverage signal | `checks_completed` and `checks_skipped` tell the agent which categories ran and which timed out. | [ ] |
| CL-3 | Summary vs full | `summary: true` clearly returns top findings only; the agent knows to re-request full detail when needed. | [ ] |
| CL-4 | No-tracking error | The `ErrNoData` response includes a recovery tool call so the agent self-corrects by checking health/tracking. | [ ] |
| CL-5 | ACTION REQUIRED nudge | The prepended nudge unambiguously points the agent at `/kaboom/audit` (or `/audit`). | [ ] |
| CL-6 | Report contract | The six-lane report's fixed sections (Overall Score, Lane Scores, Executive Summary, Top Findings, Fast Wins, Ship Blockers, Coverage And Limits) are stable and labeled. | [ ] |

### Common LLM Misinterpretation Risks

- [ ] The agent treats a skipped check as "no issues" instead of "not run" — verify `checks_skipped` is distinct from an empty section.
- [ ] The agent calls four separate tools instead of the one `page_issues` sweep — verify the index and skill steer toward the aggregate.
- [ ] The agent runs the audit without a tracked tab — verify the error response is actionable.
- [ ] The agent invents report sections — verify the contract is fixed across command and skill.

---

## 3. Simplicity Assessment

**Goal:** Count steps and cognitive load for the audit experience.

**Complexity Score:** Low (one trigger, one sweep) / Medium (full six-lane report).

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Get all page issues | 1 step: `analyze(what="page_issues")` | No — already a single aggregate |
| Cheap triage | 1 step: add `summary: true` | No |
| Launch a full audit | 1 step: click `Audit` (popup or hover) | No — one shared trigger |
| Recover when no tab tracked | 1 step: follow the recovery tool call | No |

### Default Behavior Verification

- [ ] With no parameters, all four categories run with a 50-issue per-section cap.
- [ ] Popup and hover both route through `requestAudit` (identical behavior).
- [ ] The workflow opens the terminal panel first, then requests the audit.
- [ ] When the terminal is unavailable, the daemon-intent fallback engages automatically.

---

## 4. Code Test Plan

### 4.1 Unit Tests

Server source: `cmd/browser-agent/internal/toolanalyze/pageissues/handler.go`. Tests: `handler_test.go` and `summary_test.go` in that package.

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | No tab tracked | tracking disabled | `ErrNoData` with a recovery tool call | must |
| UT-2 | All categories default | no `categories` | All four checks run | must |
| UT-3 | Category subset | `categories: ["console_errors"]` | Only the console checker runs | must |
| UT-4 | Console severity mapping | error + warn entries | error → `high`, warn → `medium` | must |
| UT-5 | Network severity mapping | 404 and 500 bodies | 4xx → `medium`, 5xx → `high` | must |
| UT-6 | Per-section limit | more issues than `limit` | Section capped at `limit` | must |
| UT-7 | Default limit | `limit` omitted/<=0 | Defaults to 50 | should |
| UT-8 | Severity tally | mixed-severity issues | `by_severity` counts correct | must |
| UT-9 | Summary mode | `summary: true` | Condensed top-findings result | must |
| UT-10 | a11y impact mapping | axe `critical/serious/moderate/minor` | → `critical/high/medium/low` | must |
| UT-11 | Security scanner absent | no scanner configured | Security section returns no issues, no failure | should |
| UT-12 | Timestamp + page_url | any successful sweep | RFC3339 timestamp and tracked URL present | should |

### 4.2 Integration Tests

| # | Test Case | Components | Expected Behavior | Priority |
|---|-----------|-----------|-------------------|----------|
| IT-1 | Parallel checks aggregate | all four checkers | One merged result with sections and totals | must |
| IT-2 | Per-check timeout | a slow check | Recorded under `checks_skipped`; others still return | must |
| IT-3 | Errored check (non-timeout) | a checker returns an error | Recorded under `checks_completed` with an `error` field | should |
| IT-4 | Shared prefetch | concurrent checkers | All read a single consistent snapshot (no TOCTOU) | should |
| IT-5 | requestAudit trigger | popup and hover | Both call `requestAudit`; terminal opened first, then bridge sent | must |
| IT-6 | Fallback nudge | PTY injection unavailable | `qa_scan` intent stored; next MCP response prepends `ACTION REQUIRED` | must |

Extension tests: `tests/extension/reliability/request-audit.test.js`, `tests/extension/content/message-handlers.test.js`. Response-policy test: `cmd/browser-agent/internal/mcpresponse/owner_test.go`.

### 4.3 Edge Case Tests

| # | Edge Case | Scenario | Expected Behavior | Priority |
|---|-----------|----------|-------------------|----------|
| EC-1 | All checks time out | extension unresponsive | All categories under `checks_skipped`; empty sections | should |
| EC-2 | No issues found | clean page | `total_issues: 0`, empty `by_severity` | should |
| EC-3 | Terminal open failure | side panel cannot open | `requestAudit` still sends the audit bridge | must |
| EC-4 | No tracked site at launch | user clicks Audit untracked | Copy instructs the user to track a site first | must |
| EC-5 | Namespaced command unsupported | `/kaboom/audit` unavailable | Operator falls back to `/audit` | should |

---

## 5. UAT Checklist (Human + AI)

### Prerequisites

- [ ] Kaboom server running and extension connected
- [ ] A tab is tracked
- [ ] A test page with at least one console error, one 4xx/5xx request, and one accessibility violation

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `analyze(what="page_issues")` | Page has known issues | One report with all four sections, severities, and totals | [ ] |
| UAT-2 | `analyze(what="page_issues", summary=true)` | Same page | Condensed top findings; markedly fewer tokens | [ ] |
| UAT-3 | `analyze(what="page_issues", categories=["network_failures"])` | Failed requests present | Only the network section populated | [ ] |
| UAT-4 | Untrack the tab, then call `page_issues` | No tab tracked | `ErrNoData` with a recovery tool call | [ ] |
| UAT-5 | Click `Audit` in the popup | Terminal panel opens, audit prompt appears | Six-lane workflow begins from a tracked site | [ ] |
| UAT-6 | Trigger `Audit` from the hover launcher | Same as popup | Identical behavior via `requestAudit` | [ ] |
| UAT-7 | With terminal injection unavailable, trigger Audit | Next MCP response | `ACTION REQUIRED` nudge points at `/kaboom/audit` | [ ] |
| UAT-8 | Run the full `/kaboom/audit` | Final report | Fixed sections present: Overall Score, Lane Scores, Executive Summary, Top Findings, Fast Wins, Ship Blockers, Coverage And Limits | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | All traffic local | Monitor network during a sweep | Only 127.0.0.1 traffic | [ ] |
| DL-UAT-2 | No response bodies in network section | Inspect a `network_failures` entry | URL/method/status/duration only | [ ] |
| DL-UAT-3 | Security evidence bounded | Inspect a security finding | Evidence is summarized, not a full secret | [ ] |

### Regression Checks

- [ ] Individual primitives (`observe(what="logs")`, `observe(what="network")`, `observe(what="accessibility")`) still work.
- [ ] The `qa` skill still redirects to the audit workflow.
- [ ] Tracking status and health checks behave as before.

---

## Sign-Off

| Area | Tester | Date | Pass/Fail |
|------|--------|------|-----------|
| Data Leak Analysis | | | |
| LLM Clarity | | | |
| Simplicity | | | |
| Code Tests | | | |
| UAT | | | |
| **Overall** | | | |
