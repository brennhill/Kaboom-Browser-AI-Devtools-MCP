---
status: proposed
scope: feature/performance-trace/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-06-29
doc_type: qa-plan
feature_id: feature-performance-trace
last_reviewed: 2026-07-05
last_verified_version: 0.8.5
last_verified_date: 2026-06-29
---

# QA Plan: Performance Trace

> QA plan for the Performance Trace feature. Covers data leak analysis, LLM clarity, simplicity
> assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Kaboom runs on localhost and
data must never leave the machine.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Trace categories scope | Verify only `devtools.timeline`, `v8.execute`, and `blink.user_timing` are enabled, so no form input or storage categories are recorded. | high |
| DL-2 | Source URLs revealing tokens | Script-evaluation attribution shows source URLs; verify query strings carrying tokens are not echoed in insight labels. | medium |
| DL-3 | Raw trace file location | Verify the optional raw-trace save writes only to the user-specified local path. | high |
| DL-4 | External transmission | Verify trace data is delivered only to the local daemon over `127.0.0.1`. | critical |
| DL-5 | Insight cache lifetime | Verify cached insights clear on the next `start` and at session end. | medium |

### Negative Tests (must NOT leak)
- [ ] No trace data is transmitted to external servers
- [ ] Insight labels do not echo URL query tokens
- [ ] Raw-trace save writes only to the requested local path
- [ ] Cached insights are cleared on the next `start`

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the responses can act on them without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Action lifecycle | The agent understands the `start` -> `stop` -> `analyze` sequence and its ordering constraints. | [ ] |
| CL-2 | Insight severity | `severity` (high/moderate) guides prioritization; the agent acts on high-severity insights first. | [ ] |
| CL-3 | Main-thread breakdown | The agent reads `script_eval_ms`, `render_ms`, and `idle_ms` as a partition of the trace window. | [ ] |
| CL-4 | Long-task attribution | The agent maps a long task to its source via the insight title. | [ ] |
| CL-5 | `insight_id` drill-down | The agent uses `insight_id` from the summary to request details via `action: "analyze"`. | [ ] |
| CL-6 | Trace versus vitals distinction | The agent distinguishes `performance_trace` (on-demand, deep) from `observe({what: "vitals"})` (passive). | [ ] |

### Common LLM Misinterpretation Risks
- [ ] Agent calls `stop` or `analyze` before `start`
- [ ] Agent treats `cls_contribution` as milliseconds
- [ ] Agent conflates the trace summary with passive Web Vitals

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load.

**Complexity Score:** Low (start/stop), Medium (start/stop plus insight drill-down)

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Trace a page load | 2 steps: `start` (reload), then `stop` | `auto_stop` collapses to effectively 1 step |
| Trace with auto-stop | 1 step: `start` with `auto_stop: true` | No -- single call captures the load |
| Drill into an insight | 1 step: `analyze` with `insight_id` | No -- single parameter |

### Default Behavior Verification
- [ ] `start` defaults to `reload: true` and `auto_stop: true`
- [ ] `stop` returns a structured summary with insights and a main-thread breakdown
- [ ] Insights are cached automatically for drill-down

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Long-task detection | Trace with a 320ms main-thread block | Long-task insight with duration | must |
| UT-2 | Layout-shift extraction | Trace with a layout shift | Insight with `cls_contribution` | must |
| UT-3 | Forced-reflow detection | Trace with style recalc after mutation | Forced-reflow count | must |
| UT-4 | Script-eval attribution by source | Trace with multiple scripts | Eval time grouped by source URL | must |
| UT-5 | Main-thread breakdown | Parsed trace | `script_eval_ms`, `render_ms`, `idle_ms` partition | must |
| UT-6 | Summary under size budget | Large trace | Response under 5KB | must |
| UT-7 | Insight cache for drill-down | Parsed insights + `insight_id` | Detail returned synchronously | should |
| UT-8 | `stop` without `start` | No active trace | "no active trace" error | must |
| UT-9 | Unknown `insight_id` | `analyze` with bad id | "no such insight" error listing valid ids | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Full trace round trip | Daemon -> async start/stop -> extension Tracing -> parser | Summary returned via MCP | must |
| IT-2 | Auto-stop on page load | `start` with `auto_stop: true` | Trace ends after load completes | should |
| IT-3 | Insight drill-down | `stop` then `analyze` with `insight_id` | Detail from cached insight | must |
| IT-4 | Debugger attach conflict | DevTools open | Recovery-action error | must |
| IT-5 | Second `start` while active | `start` twice | Documented "already tracing" behavior | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Summary response size | Bytes | < 5KB typical | must |
| PT-2 | Trace parse time | Parse of a multi-MB trace | Off the synchronous path | should |
| PT-3 | `analyze` drill-down | Cached insight lookup | < 100ms | must |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Internal browser page | Tracked tab on `chrome://` | Cannot-attach error | must |
| EC-2 | Single-page-app navigation with no load event | `auto_stop` with no load | Falls back to maximum duration | should |
| EC-3 | Empty/quiet trace | Idle page | Zero long tasks with a note, not an error | should |
| EC-4 | `stop` without `start` | Out-of-order call | "no active trace" error | must |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A page with a known main-thread bottleneck (heavy script on load) loaded and tracked

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `analyze({what: "performance_trace", action: "start", reload: true})` | Page reloads | Trace begins; queued/started status | [ ] |
| UAT-2 | `analyze({what: "performance_trace", action: "stop"})` | -- | Summary with long tasks, layout shifts, main-thread breakdown | [ ] |
| UAT-3 | `analyze({what: "performance_trace", action: "analyze", insight_id: "insight-1"})` | -- | Detailed breakdown for that insight | [ ] |
| UAT-4 | `analyze({what: "performance_trace", action: "start", auto_stop: true})` | Page loads then trace ends | Single-call capture of the load | [ ] |
| UAT-5 | Open DevTools, then `action: "start"` | DevTools attached | Attach-conflict error with recovery action | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Localhost-only traffic | Monitor network during trace | Only `127.0.0.1:7890` traffic | [ ] |
| DL-UAT-2 | Limited trace categories | Inspect Tracing.start call | Only timeline, V8 execute, user timing categories | [ ] |

### Regression Checks
- [ ] Existing `observe({what: "vitals"})` passive telemetry still works
- [ ] Existing `analyze({what: "performance"})` still works
- [ ] CDP-based interact actions still work after a trace detaches the debugger

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
