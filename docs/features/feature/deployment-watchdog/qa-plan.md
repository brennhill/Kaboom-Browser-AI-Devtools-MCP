---
status: shipped
scope: feature/deployment-watchdog/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-deployment-watchdog
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Deployment Watchdog

> QA plan for the Deployment Watchdog feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. The Deployment Watchdog captures pre/post-deploy baselines, monitors telemetry continuously, and fires alerts. Baseline snapshots, alert details, and deployment summaries could expose server configurations, API endpoint structures, and production error details.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Baseline snapshot contains sensitive URLs with query-string secrets | Verify that URLs captured in the baseline snapshot have sensitive query parameters stripped (e.g., `?api_key=`, `?token=`, `?secret=`) by the extension's privacy layer | critical |
| DL-2 | Network body content in baseline captures tokens/secrets | Verify that captured network request/response bodies in the baseline do not contain API tokens, session cookies, or authentication credentials in JSON payloads | critical |
| DL-3 | Deployment label contains secrets | Verify that `deployment_label` freeform string is treated as display-only and does not get used in any context where it could be interpreted as code or reveal internal deployment infrastructure | medium |
| DL-4 | Alert details expose internal server error stacks | Verify that `alerts[].details.new_errors[].message` does not include internal server stack traces, file paths, or database connection strings from console errors | high |
| DL-5 | Watchdog summary reveals internal API endpoint topology | Verify that `baseline_vs_final` comparison does not expose a full list of internal API endpoints that could be used for attack surface mapping | high |
| DL-6 | Auth headers in captured network state | Verify that the baseline and regression-check network captures strip Authorization, Cookie, Set-Cookie, and other sensitive headers identically to `observe({what: "network"})` | critical |
| DL-7 | WebSocket connection details expose auth tokens | Verify that WebSocket connection URLs and handshake headers captured in baseline do not include authentication tokens (e.g., `wss://api.example.com?token=secret`) | high |
| DL-8 | Alert recommendation text leaks internal data | Verify that the `recommendation` field in alerts contains generic guidance and does not reference specific internal infrastructure, IPs, or credentials | medium |
| DL-9 | Named snapshot persisted to disk contains unredacted data | Verify that `wd-baseline-{label}` snapshots saved to the session manager apply the same redaction rules as all other Kaboom captures | high |
| DL-10 | Audit trail of watchdog events exposes sensitive context | Verify that watchdog start/stop/alert audit entries do not include raw network body content or unredacted error messages | medium |

### Negative Tests (must NOT leak)
- [ ] API keys in URL query strings must not appear in baseline snapshot `network_error_rate` endpoint data
- [ ] Authentication cookies must not appear in baseline `console_errors` messages
- [ ] Internal IP addresses and port numbers (beyond localhost:7890) must not appear in alert details
- [ ] WebSocket auth tokens must not appear in watchdog status responses
- [ ] Response bodies with Bearer tokens must be redacted in watchdog summary `baseline_vs_final`
- [ ] Deployment infrastructure URLs (e.g., internal CI/CD dashboards) must not appear in MCP responses
- [ ] Server config paths (e.g., `/etc/nginx/`, `/var/log/`) must not appear in watchdog error captures

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Watchdog state machine clarity | Verify that `status` field values (`capturing_baseline`, `monitoring`, `alert`, `completed`, `stopped`) are unambiguous and the AI can determine current state without additional context | [ ] |
| CL-2 | Alert severity classification | Verify that `severity` values (`high`, `medium`) are clearly defined in responses and the AI can distinguish "must rollback" (high) from "investigate further" (medium) | [ ] |
| CL-3 | Verdict interpretation | Verify that `verdict` values (`healthy`, `degraded`, `regressed`, `inconclusive`, `unchanged`) are unambiguous and the AI knows what action each implies | [ ] |
| CL-4 | Latency regression percentage vs factor | Verify that `latency_vs_baseline: "+12%"` is clearly a percentage change, not a factor (the AI should not confuse "+12%" with "12x baseline") | [ ] |
| CL-5 | Pre-existing errors excluded from alerts | Verify that when the baseline already has errors, the alert response clearly states "2 NEW console errors" (not "2 console errors total"), preventing the AI from confusing pre-existing errors with regressions | [ ] |
| CL-6 | Time remaining clarity | Verify that `remaining_minutes` is clearly labeled and the AI understands the watchdog will auto-stop when it reaches 0, not that it "has 0 minutes of data" | [ ] |
| CL-7 | Stop reason disambiguation | Verify that `reason` values (`rollback_triggered`, `soak_complete`, `manual_stop`) are unambiguous and the AI knows the difference between a planned completion and a forced stop | [ ] |
| CL-8 | Multiple concurrent watchdogs | Verify that when multiple watchdogs are active, `observe({what: "deployment_status"})` clearly tags each alert with its `watchdog_id` and `deployment_label` so the AI does not confuse alerts from different deployments | [ ] |
| CL-9 | Regression self-resolution signal | Verify that when a regression self-resolves (alert -> monitoring transition), the response clearly communicates that the condition cleared, not that the alert was a false positive | [ ] |
| CL-10 | Inconclusive vs healthy distinction | Verify that `inconclusive` (no telemetry received) is clearly distinct from `healthy` (telemetry received, no regressions) so the AI does not incorrectly declare a deployment successful when no data was collected | [ ] |
| CL-11 | Threshold null semantics | Verify that passing `null` for a threshold dimension (e.g., `"max_new_errors": null`) is clearly documented as "skip this check" not "use default" or "zero tolerance" | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI might interpret `verdict: "unchanged"` as "deployment had no effect" when it actually means "no telemetry changes observed" (possibly because no traffic was generated) -- verify the distinction is clear
- [ ] AI might interpret `alerts: []` during monitoring as "deployment is healthy" when monitoring just started -- verify elapsed time context is included
- [ ] AI might trigger rollback on a `medium` severity alert -- verify severity-to-action mapping is explicit in recommendations
- [ ] AI might start a new watchdog without stopping the existing one for the same deployment -- verify concurrent session limits are communicated
- [ ] AI might confuse `monitoring_until` timestamp with "deploy completed at" -- verify field naming is unambiguous

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Medium

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Start watchdog and monitor deployment | 3 steps: (1) start watchdog with label/thresholds, (2) trigger deployment externally, (3) poll status or wait for changes alert | No -- the separation of "start watchdog" and "trigger deploy" is intentional (baseline capture must precede deploy) |
| Check watchdog status | 1 step: `configure({action: "watchdog", watchdog_action: "status"})` | No -- already minimal |
| Passive alert via changes polling | 1 step: `observe({what: "changes"})` includes `deployment_alerts` key | No -- already integrated into existing polling pattern |
| Check deployment overview | 1 step: `observe({what: "deployment_status"})` | No -- already minimal |
| Stop watchdog and get summary | 1 step: `configure({action: "watchdog", watchdog_action: "stop"})` returns summary | No -- already minimal |
| Full deploy-monitor-decide workflow | 5 steps: start watchdog, deploy, poll status periodically, read alert details, stop or wait for completion | Yes -- auto-detect deployment completion could eliminate explicit polling; however, keeping polling explicit reduces magic behavior |

### Default Behavior Verification
- [ ] Feature works with zero-threshold configuration: `configure({action: "watchdog", watchdog_action: "start", deployment_label: "v1.0"})` uses all default thresholds
- [ ] Default `duration_minutes: 10` is applied when not specified
- [ ] Default `max_new_errors: 0` is conservative and catches any new error
- [ ] Default `latency_factor: 1.5` is reasonable for typical deployment scenarios
- [ ] Default `max_network_error_rate: 0.05` (5%) is reasonable
- [ ] Default `vitals_regression_pct: 20` is reasonable for UX degradation detection
- [ ] Watchdog auto-stops after duration without requiring explicit stop call
- [ ] Alerts automatically appear in `observe({what: "changes"})` without additional configuration

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Start watchdog with all defaults | `configure({action: "watchdog", watchdog_action: "start", deployment_label: "v1.0"})` | Watchdog created with default thresholds, status `capturing_baseline`, baseline snapshot captured | must |
| UT-2 | Start watchdog with custom thresholds | `{max_new_errors: 5, latency_factor: 2.0, duration_minutes: 30}` | Thresholds applied as specified, duration set to 30 minutes | must |
| UT-3 | Reject invalid duration (0 minutes) | `{duration_minutes: 0}` | Error: "duration must be between 1 and 60 minutes" | must |
| UT-4 | Reject invalid duration (negative) | `{duration_minutes: -5}` | Error: "duration must be between 1 and 60 minutes" | must |
| UT-5 | Reject invalid duration (>60 minutes) | `{duration_minutes: 120}` | Error: "duration must be between 1 and 60 minutes" | must |
| UT-6 | Baseline captures pre-existing errors | Baseline state has 2 console errors | Baseline records 2 errors with fingerprints for exclusion | must |
| UT-7 | New error detection (not in baseline) | Post-deploy: new TypeError not in baseline fingerprints | Alert fired with `type: "new_errors"`, severity based on count | must |
| UT-8 | Pre-existing error not flagged | Post-deploy: same error as baseline appears again | No alert fired for this error | must |
| UT-9 | Latency regression at 1.5x threshold | Baseline load_time: 1000ms, current: 1600ms (1.6x) | Alert fired with `type: "latency_regression"`, severity `medium` | must |
| UT-10 | Latency regression at 3x threshold | Baseline load_time: 1000ms, current: 3100ms (3.1x) | Alert fired with severity `high` | must |
| UT-11 | Latency within threshold | Baseline load_time: 1000ms, current: 1400ms (1.4x < 1.5x) | No alert fired | must |
| UT-12 | Network error rate exceeds threshold | Baseline rate: 0.01, current: 0.06 (> 0.05 default) | Alert fired with `type: "network_failure"` | must |
| UT-13 | Specific endpoint status change (2xx -> 5xx) | Endpoint `/api/users` was 200, now 500 | Alert fired with severity `high` | must |
| UT-14 | Specific endpoint status change (2xx -> 4xx) | Endpoint `/api/users` was 200, now 404 | Alert fired with severity `medium` | must |
| UT-15 | Web vitals CLS regression | Baseline CLS: 0.05, current CLS: 0.20 (delta > 0.1) | Alert fired with `type: "vitals_regression"`, severity `medium` | should |
| UT-16 | Web vitals CLS within tolerance | Baseline CLS: 0.05, current CLS: 0.12 (delta 0.07 < 0.1) | No alert fired | should |
| UT-17 | LCP percentage regression | Baseline LCP: 800ms, current LCP: 1000ms (+25% > 20%) | Alert fired with `type: "vitals_regression"` | should |
| UT-18 | Null threshold disables check | `{max_new_errors: null}` | Error monitoring skipped, no error alerts possible | must |
| UT-19 | All thresholds null | All threshold fields set to null | Watchdog runs, no alerts fire, summary shows baseline vs final comparison | should |
| UT-20 | Auto-stop after duration | Watchdog started with 1 minute duration, no alerts | After 60 seconds, status changes to `completed` with `reason: "soak_complete"` and `verdict: "healthy"` | must |
| UT-21 | Manual stop with reason | `configure({action: "watchdog", watchdog_action: "stop", reason: "rollback_triggered"})` | Status changes to `stopped`, summary generated with specified reason | must |
| UT-22 | Status query for unknown watchdog_id | `watchdog_id: "wd-nonexistent"` | Error: "watchdog session not found" | must |
| UT-23 | Regression self-resolves | Alert fires at check 3, condition clears at check 5 | Status transitions: monitoring -> alert -> monitoring, alert remains in history | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Watchdog baseline uses diff_sessions capture | Watchdog start + SessionManager.Capture() | Baseline snapshot matches what `diff_sessions({session_action: "capture"})` would produce | must |
| IT-2 | Alert appears in changes stream | Watchdog alert + `observe({what: "changes"})` | `deployment_alerts` key contains the alert with correct watchdog_id and summary | must |
| IT-3 | Push regression suppression during watchdog | Watchdog active + push regression fires for same metric | Watchdog alert takes precedence, push regression alert suppressed | should |
| IT-4 | Behavioral baseline integration | Saved behavioral baseline + watchdog start for same URL scope | Watchdog loads behavioral baseline for richer comparison, alerts include `type: "behavioral_regression"` | could |
| IT-5 | Performance budget integration | Budget configured + watchdog active | Budget violation reported as watchdog alert with `type: "budget_violation"` | could |
| IT-6 | Named snapshot saved for later diffing | Watchdog starts with label "v2.3.0" | Snapshot `wd-baseline-v2.3.0` available via `diff_sessions({session_action: "compare"})` | should |
| IT-7 | Watchdog summary includes session diff | Watchdog completes | Summary contains `baseline_vs_final` with same structure as `diff_sessions` compare result | must |
| IT-8 | Audit trail records watchdog lifecycle | Watchdog start + alert + stop | `configure({action: "audit_log"})` shows entries for start, each alert, and stop with timestamps | should |
| IT-9 | Concurrent watchdogs (max 3) | Start 3 watchdogs, attempt 4th | First 3 succeed, 4th returns error "maximum concurrent watchdog sessions reached" | could |
| IT-10 | observe({what: "deployment_status"}) returns active and recent | 1 active watchdog + 1 completed watchdog | Response shows both under `active_watchdogs` and `recent_completions` respectively | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Baseline capture time | Wall clock time for `capturing_baseline` -> `monitoring` transition | < 50ms | must |
| PT-2 | Per-check evaluation time | Time for a single 30-second regression check across all 4 dimensions | < 5ms | must |
| PT-3 | Alert generation and storage | Time from threshold breach detection to alert availability in status query | < 1ms | must |
| PT-4 | Memory per watchdog session | Memory footprint of one active watchdog including baseline, alerts, history | < 200KB | must |
| PT-5 | Three concurrent watchdog sessions | Combined memory of 3 sessions with active monitoring | < 600KB | should |
| PT-6 | Status query response time | Time for `watchdog_action: "status"` to return | < 10ms | must |
| PT-7 | Summary generation at stop | Time to generate the full deployment summary with baseline_vs_final diff | < 50ms | should |
| PT-8 | 30-second check interval does not drift | After 10 minutes of monitoring (20 checks), verify timing accuracy | Each check starts within +/- 1 second of the expected 30-second interval | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | No telemetry arrives during monitoring | Browser tab closed or navigated away | Watchdog completes with `verdict: "inconclusive"` and note about insufficient data | must |
| EC-2 | Extension disconnects mid-monitoring | WebSocket connection to extension drops | Watchdog continues but cannot detect regressions; reports `inconclusive` | must |
| EC-3 | Server restarts during monitoring | Kaboom process killed and restarted | Watchdog session lost; `observe({what: "deployment_status"})` returns empty list | must |
| EC-4 | Multiple deployments overlap | Second watchdog started while first is active | Both run independently with separate baselines and thresholds | should |
| EC-5 | Baseline captured during error state | Pre-deploy state already has 5 console errors | Pre-existing errors recognized by fingerprint, not flagged as regressions | must |
| EC-6 | Rapid consecutive deployments | Deploy v1, start watchdog, deploy v2 within soak period | First watchdog's baseline becomes stale; AI should stop first and start new watchdog | should |
| EC-7 | Regression self-resolves (latency spike) | Latency spikes at check 3, returns to normal at check 5 | Alert fired, transitions to monitoring, final summary records alert and recovery | should |
| EC-8 | All threshold dimensions disabled (null) | Every threshold set to null | Watchdog runs without alerts, produces summary with baseline vs final comparison | should |
| EC-9 | Duration exactly 1 minute (minimum) | `duration_minutes: 1` | Watchdog runs 2 checks (at 0s and 30s), then completes | should |
| EC-10 | Duration exactly 60 minutes (maximum) | `duration_minutes: 60` | Watchdog runs 120 checks, completes after 60 minutes | should |
| EC-11 | Watchdog started but no deployment happens | Baseline captured, AI never triggers deploy | Watchdog completes with `verdict: "unchanged"` after duration | should |
| EC-12 | High-frequency error burst post-deploy | 100 new errors in first 30 seconds | Single alert with count, not 100 separate alerts | must |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A web application loaded in the tracked tab (e.g., `http://localhost:3000`)
- [ ] Application has a way to simulate a "deployment" (e.g., restart with a code change that introduces an error)

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | AI starts watchdog: `{"tool":"configure","arguments":{"action":"watchdog","watchdog_action":"start","deployment_label":"v1.0-test","duration_minutes":5,"thresholds":{"max_new_errors":0,"latency_factor":1.5,"max_network_error_rate":0.05,"vitals_regression_pct":20}}}` | Server logs show watchdog starting | Response shows `status: "capturing_baseline"` with baseline data, transitions to `monitoring` | [ ] |
| UAT-2 | AI checks status: `{"tool":"configure","arguments":{"action":"watchdog","watchdog_action":"status","watchdog_id":"wd-xxx"}}` | No changes in browser | Response shows `status: "monitoring"`, `alerts: []`, `verdict: "healthy"` | [ ] |
| UAT-3 | Human "deploys" by introducing a console error in the app (e.g., add `throw new Error("PaymentService is not defined")`) and refreshes the page | Error visible in browser console | Error appears in Kaboom capture | [ ] |
| UAT-4 | AI waits 30+ seconds for next check, then checks status again | Watchdog check fires | Response shows `status: "alert"`, `alerts` array contains the new error, `verdict: "degraded"` | [ ] |
| UAT-5 | AI checks via observe: `{"tool":"observe","arguments":{"what":"deployment_status"}}` | N/A | `active_watchdogs` contains the watchdog with `status: "alert"` and `alerts_count: 1` | [ ] |
| UAT-6 | AI checks via changes: `{"tool":"observe","arguments":{"what":"changes"}}` | N/A | Response includes `deployment_alerts` key with the watchdog alert | [ ] |
| UAT-7 | AI stops watchdog: `{"tool":"configure","arguments":{"action":"watchdog","watchdog_action":"stop","watchdog_id":"wd-xxx","reason":"rollback_triggered"}}` | Server logs show watchdog stopping | Response shows `status: "stopped"`, `reason: "rollback_triggered"`, `summary.verdict: "regressed"`, `summary.baseline_vs_final.new_errors: 1` | [ ] |
| UAT-8 | AI verifies clean completion by starting new watchdog, no changes made, wait for duration | 5-minute wait | Watchdog auto-completes with `verdict: "healthy"`, `reason: "soak_complete"` | [ ] |
| UAT-9 | AI checks deployment_status after completion | N/A | `recent_completions` contains the completed watchdog | [ ] |
| UAT-10 | AI attempts to start watchdog with duration 0 | N/A | Error response: duration must be between 1 and 60 minutes | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Baseline does not expose auth headers | Start watchdog while app makes authenticated API calls, inspect baseline in start response | No Authorization, Cookie, or token header values in baseline | [ ] |
| DL-UAT-2 | Alert details do not expose internal paths | Trigger an error containing a file path (e.g., `/home/user/app/src/index.js:42`), check alert | Error message included but no server-side paths beyond what was in the console error | [ ] |
| DL-UAT-3 | Watchdog summary does not expose full endpoint list | Complete a watchdog session, inspect `baseline_vs_final` | Network data is summarized (error rates, latency changes) not listed as individual endpoints | [ ] |
| DL-UAT-4 | Named snapshot is redacted | After watchdog runs, use `diff_sessions` to compare against the saved `wd-baseline-*` snapshot | Snapshot data has same redaction as all other Kaboom captures | [ ] |

### Regression Checks
- [ ] Existing `diff_sessions` capture/compare functionality works independently of watchdog
- [ ] Existing `observe({what: "changes"})` works with no active watchdog (no deployment_alerts key or empty)
- [ ] Existing push regression alerts fire when no watchdog is active
- [ ] Performance budget checks work independently of watchdog
- [ ] Extension telemetry capture is not affected by watchdog monitoring
- [ ] Server memory returns to pre-watchdog levels after watchdog stops

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
