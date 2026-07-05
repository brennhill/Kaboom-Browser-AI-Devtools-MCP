---
status: proposed
scope: feature/lighthouse-report/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-06-29
doc_type: qa-plan
feature_id: feature-lighthouse-report
last_reviewed: 2026-07-05
last_verified_version: 0.8.4
last_verified_date: 2026-06-29
---

# QA Plan: Lighthouse Report

> QA plan for the Lighthouse Report feature. Covers data leak analysis, LLM clarity,
> simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Kaboom runs on localhost and
data must never leave the machine.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Lighthouse subprocess network egress | Verify the Lighthouse CLI audits only the local debugging endpoint and does not transmit the report to any external service. | critical |
| DL-2 | Raw report written to disk | If `save_path` saves the full report, verify it is written only to the user-specified local path and contains no credentials. | high |
| DL-3 | URL query parameters in scores payload | Verify the returned `url` field does not echo session tokens from the audited page's query string. | medium |
| DL-4 | Subprocess argument injection | Verify the audited URL and debugging endpoint are passed as controlled arguments, not interpolated into a shell string. | critical |
| DL-5 | Localhost-only transport | Verify all command and result traffic flows over `127.0.0.1` only. | critical |

### Negative Tests (must NOT leak)
- [ ] No audit data is transmitted to external servers
- [ ] The trimmed response contains no page form values or credentials
- [ ] Subprocess arguments are not vulnerable to shell injection from page content

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool response can act on it without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Score semantics | Scores are 0-100 integers per category. The agent understands higher is better. | [ ] |
| CL-2 | Audit versus heuristic distinction | The agent distinguishes `lighthouse_report` (real Lighthouse) from `audit` (heuristic estimate). | [ ] |
| CL-3 | Opportunity savings units | `savings_ms` is milliseconds saved; the agent does not confuse it with score points. | [ ] |
| CL-4 | Device and mode fields | `device` and `mode` echo the request so the agent knows which configuration produced the scores. | [ ] |
| CL-5 | Core Web Vitals naming | Metric keys (`fcp_ms`, `lcp_ms`, `cls`, `tbt_ms`, `si_ms`, `tti_ms`) map to standard metrics. | [ ] |
| CL-6 | Attach-conflict error | The error clearly states the recovery action (close DevTools). | [ ] |

### Common LLM Misinterpretation Risks
- [ ] Agent treats `audit` and `lighthouse_report` as interchangeable
- [ ] Agent reads `cls` (unitless) as milliseconds
- [ ] Agent assumes snapshot mode returns navigation-timing metrics

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load.

**Complexity Score:** Low (single audit), Medium (audit plus opportunity follow-up)

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Run a full audit | 1 step: `analyze({what: "lighthouse_report"})` | No -- already minimal |
| Audit a single category | 1 step: add `categories: ["performance"]` | No -- single parameter |
| Audit mobile profile | 1 step: add `device: "mobile"` | No -- single parameter |
| Audit without reload | 1 step: add `mode: "snapshot"` | No -- single parameter |

### Default Behavior Verification
- [ ] Works with zero parameters (all categories, desktop, navigation mode)
- [ ] Default response is a trimmed, actionable report

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Report trimmer extracts scores | Raw Lighthouse JSON | Four category scores as integers | must |
| UT-2 | Report trimmer extracts Core Web Vitals | Raw Lighthouse JSON | FCP, LCP, CLS, TBT, SI, TTI values | must |
| UT-3 | Opportunities sorted by savings | Raw report with multiple opportunities | Top opportunities by `savings_ms` | must |
| UT-4 | Category filtering | `categories: ["performance"]` | Only performance score present | should |
| UT-5 | Device parameter validation | `device: "tablet"` | Validation error (only desktop/mobile) | must |
| UT-6 | Mode parameter validation | `mode: "invalid"` | Validation error (only navigation/snapshot) | must |
| UT-7 | Snapshot mode omits navigation metrics | `mode: "snapshot"` | No load-only Core Web Vitals | should |
| UT-8 | Trimmed response under size budget | Large raw report | Response under 5KB | must |
| UT-9 | Missing Lighthouse CLI (Option A) | CLI not in PATH | Actionable install error | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Full audit round trip | Daemon handler -> async command -> extension/CLI -> trimmer | Structured report returned via MCP | must |
| IT-2 | Asynchronous polling | Daemon enqueues command, agent polls `command_result` | Result delivered after audit completes | must |
| IT-3 | Debugger attach conflict | DevTools open on tracked tab | Recovery-action error | must |
| IT-4 | Timeout handling | Audit exceeds 60s budget | Failed-command error | must |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Navigation audit duration | Wall-clock time | 10-30s typical | should |
| PT-2 | Response size | Bytes of trimmed response | < 5KB typical | must |
| PT-3 | Daemon parse time | Raw report parse | < 200ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Internal browser page | Tracked tab on `chrome://extensions` | Cannot-attach error | must |
| EC-2 | Debugger already attached | DevTools open | Attach-conflict error with recovery | must |
| EC-3 | Audit of a slow page | Page exceeds load timeout | Partial or timeout result, clearly flagged | should |
| EC-4 | All categories excluded | Empty `categories` array | Validation error or default-all behavior | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] Lighthouse CLI installed in PATH (for Option A)
- [ ] A real web page loaded and tracked

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `analyze({what: "lighthouse_report"})` | Page is reloaded by the audit | Trimmed report with four scores, Core Web Vitals, opportunities | [ ] |
| UAT-2 | `analyze({what: "lighthouse_report", device: "mobile"})` | Mobile emulation during audit | `device: "mobile"` echoed in response | [ ] |
| UAT-3 | `analyze({what: "lighthouse_report", categories: ["performance"]})` | Faster audit | Only the performance score present | [ ] |
| UAT-4 | `analyze({what: "lighthouse_report", mode: "snapshot"})` | No page reload | Report without navigation-timing metrics | [ ] |
| UAT-5 | Open DevTools, then run an audit | DevTools attached | Attach-conflict error with recovery action | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Localhost-only traffic | Monitor network during audit | Only `127.0.0.1:7890` plus the local debugging endpoint | [ ] |
| DL-UAT-2 | No external report upload | Inspect Lighthouse invocation | No upload flags; report stays local | [ ] |

### Regression Checks
- [ ] Existing `analyze({what: "audit"})` heuristic still works
- [ ] Existing `analyze({what: "performance"})` still works
- [ ] CDP-based interact actions still work after an audit detaches the debugger

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
