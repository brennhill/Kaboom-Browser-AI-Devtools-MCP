---
status: proposed
scope: feature/push-alerts/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-push-alerts
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Push-Based Alerts (Passive + Active)

> QA plan for the Push Alerts feature (passive alert piggyback on observe responses and active MCP notification streaming). Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Kaboom runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses and MCP notifications.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Alert detail text contains raw error messages with PII | Verify alert `detail` field uses normalized or summarized text, not raw console output with user data | critical |
| DL-2 | CI webhook payload stored with secrets | Verify CI result storage strips sensitive fields (tokens, env vars) and stores only status, summary, failures | critical |
| DL-3 | Performance regression alert reveals internal API paths | Verify alert `summary` uses normalized URL paths, not full URLs with auth tokens or internal hostnames | high |
| DL-4 | MCP notification context contains auth headers | Verify `emitNotification` applies redaction before emission -- `SENSITIVE_HEADERS` list headers become `[REDACTED]` | critical |
| DL-5 | Notification context leaks request/response bodies | Verify notification `context` object contains only method, URL, status, duration -- no bodies | critical |
| DL-6 | CI webhook `url` field exposes private repo URLs | Verify CI result `url` (GitHub Actions URL) is stored but does not expose repo secrets; it is just a link | medium |
| DL-7 | Anomaly detection alert reveals user activity volume | Verify anomaly alert says "error spike detected" not "user performed 50 actions generating 15 errors" | medium |
| DL-8 | Correlation groups expose timing patterns | Verify compound alerts describe co-occurring issues, not detailed user interaction sequences | medium |
| DL-9 | Alert buffer retains stale data | Verify alert buffer drains on observe call and old alerts are evicted after cap (50), not accumulated indefinitely | medium |
| DL-10 | URL filter in streaming config reveals monitored paths | Verify `url_filter` is stored server-side only and not echoed in notification payloads | low |
| DL-11 | Notification dedup_key exposes URL + status patterns | Verify `dedup_key` format (e.g., `POST:/api/users:500`) does not embed auth tokens or query params | high |

### Negative Tests (must NOT leak)
- [ ] No auth headers (Authorization, Cookie, Bearer) in notification context
- [ ] No request/response bodies in any alert or notification payload
- [ ] No CI tokens or environment variables in CI webhook alerts
- [ ] No raw console error messages with user PII in alert detail text
- [ ] No query-string parameters containing tokens in alert URL references
- [ ] No internal hostnames or IP addresses in regression alert summaries
- [ ] No streaming config details (url_filter, severity_min) in outbound notifications

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Alert presence vs absence | AI understands single content block = no alerts; two content blocks = alerts present | [ ] |
| CL-2 | Alert severity ordering | AI understands errors appear first (highest priority), then warnings, then info | [ ] |
| CL-3 | Alert category meaning | AI distinguishes regression/anomaly/ci/noise/threshold categories as different alert sources | [ ] |
| CL-4 | Compound alert (correlated) | AI understands a compound alert groups co-occurring issues, not that they share a root cause | [ ] |
| CL-5 | Deduplication count | AI understands `count: 3` on an alert means same issue detected 3 times, not 3 different issues | [ ] |
| CL-6 | Summary prefix for 4+ alerts | AI correctly interprets "3 alerts: 1 regression, 1 anomaly, 1 CI failure" as a summary, not a 4th alert | [ ] |
| CL-7 | Alert drain semantics | AI understands alerts appear once, then are cleared -- not repeated on subsequent observe calls | [ ] |
| CL-8 | "No alerts" is normal | AI understands absence of alerts section is the default, not an error | [ ] |
| CL-9 | Active vs passive mode distinction | AI understands passive alerts ride on observe responses; active notifications are proactive without tool calls | [ ] |
| CL-10 | Throttle semantics | AI understands throttled notifications are delayed, not dropped | [ ] |
| CL-11 | Streaming disable confirmation | AI understands `configure_streaming(action: "disable")` immediately stops notifications | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI might think alerts are persistent (shown every time) -- verify they drain after delivery
- [ ] AI might confuse alert `category` with `severity` -- verify both fields are distinct and clear
- [ ] AI might interpret compound alert as single issue -- verify description separates co-occurring items
- [ ] AI might assume streaming is enabled by default -- verify it requires explicit opt-in
- [ ] AI might not realize rate-limited notifications are batched -- verify batch format is documented
- [ ] AI might think the CI webhook URL is a Kaboom endpoint to visit -- verify it is an external link

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low (passive mode) / Medium (active mode)

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Receive passive alerts | 0 steps: alerts attached to next `observe` call automatically | No -- already zero-config |
| Enable active streaming | 1 step: `configure_streaming(action: "enable")` | No -- already minimal |
| Filter streaming by category | 1 step: `configure_streaming(action: "enable", events: ["errors"])` | No -- already one call |
| Disable streaming | 1 step: `configure_streaming(action: "disable")` | No -- already minimal |
| Check streaming status | 1 step: `configure_streaming(action: "status")` | No -- already minimal |
| Post CI result | 1 step: HTTP POST to `/ci-result` with JSON body | No -- already minimal |

### Default Behavior Verification
- [ ] Passive alerts work with zero configuration (always enabled)
- [ ] Active streaming is disabled by default (requires opt-in)
- [ ] Alert buffer cap (50) enforced automatically
- [ ] Alert deduplication works automatically
- [ ] Alert priority ordering applied automatically
- [ ] CI webhook available without authentication (localhost-only)
- [ ] Throttling defaults (5s gap, 12/min, 30s dedup) applied automatically

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | No alerts: observe response format | observe call with empty alert buffer | Single content block, no alerts section | must |
| UT-2 | Alerts present: observe response format | 2 alerts in buffer, then observe call | Two content blocks: normal output + alerts JSON | must |
| UT-3 | Alert drain on observe | 3 alerts in buffer, observe called | Alerts returned in response, buffer now empty | must |
| UT-4 | Alert drain verified | After drain, call observe again | No alerts section (buffer was cleared) | must |
| UT-5 | Alert buffer cap (50) | Add 55 alerts | Only newest 50 retained, oldest 5 evicted | must |
| UT-6 | Performance regression alert generation | Snapshot with LCP 30% over baseline | Alert generated: severity "warning", category "regression" | must |
| UT-7 | Anomaly detection: error spike | 4x average error rate in 10-second window | Alert generated: severity "error", category "anomaly" | must |
| UT-8 | Anomaly detection: normal rate | Error rate within 3x average | No anomaly alert | must |
| UT-9 | CI webhook: valid POST | Valid JSON body with status/source/summary | 200 OK, alert generated: category "ci" | must |
| UT-10 | CI webhook: invalid body | Malformed JSON | 400 error, no alert generated | must |
| UT-11 | CI webhook: same commit twice | Same commit+status posted twice | Updated existing, no duplicate alert | must |
| UT-12 | CI webhook: body size limit | POST body >1MB | Rejected by MaxBytesReader | must |
| UT-13 | CI result cap (10) | Post 12 CI results | Only newest 10 retained | must |
| UT-14 | Situation synthesis: deduplication | Same regression detected 3 times | Single alert with count: 3 | must |
| UT-15 | Situation synthesis: priority ordering | 1 info + 1 warning + 1 error alert | Ordered: error, warning, info | must |
| UT-16 | Situation synthesis: correlation | Regression + error spike within 5 seconds | Single compound alert with both details | must |
| UT-17 | Situation synthesis: summary prefix | 4 alerts accumulated | Summary line: "4 alerts: ..." prepended | must |
| UT-18 | Threshold breach alert | Memory pressure transition normal -> soft | Alert generated: category "threshold" | must |
| UT-19 | Noise auto-detect alert | New noise patterns found | Info alert generated: category "noise" | should |
| UT-20 | Alert mutex independence | Alert generation concurrent with observe drain | No deadlock (alertMu separate from server.mu/capture.mu) | must |
| UT-21 | Enable streaming | `configure_streaming(action: "enable")` | StreamConfig.Enabled = true, confirmation returned | must |
| UT-22 | Disable streaming | `configure_streaming(action: "disable")` | Enabled = false, pending cleared, confirmation with pending_cleared count | must |
| UT-23 | Streaming status check | `configure_streaming(action: "status")` | Returns current config + stats | must |
| UT-24 | Category filter | `events: ["errors"]` | Only error-category notifications emitted | must |
| UT-25 | URL filter | `url_filter: "/api/"` | Only /api/ URLs in notifications | must |
| UT-26 | Severity filter | `severity_min: "error"` | Only error severity, no warnings/info | must |
| UT-27 | Throttling | 3 events in 1 second, throttle=5s | First emitted, others batched for 5s | must |
| UT-28 | Rate limit | 15 events in 1 minute | Only first 12 emitted | must |
| UT-29 | Dedup within window | Same error 5 times in 10 seconds | Only first emitted within 30s window | must |
| UT-30 | Notification redaction | Notification with Authorization header in context | Header value replaced with [REDACTED] | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Extension error -> anomaly alert -> observe | Extension sends burst of errors -> anomaly detected -> AI calls observe | Alert appears in observe response | must |
| IT-2 | Performance snapshot -> regression -> observe | Extension sends snapshot -> regression detected -> AI calls observe | Regression alert in observe response | must |
| IT-3 | CI webhook -> alert -> observe | External POST to /ci-result -> alert generated -> AI calls observe | CI alert in observe response | must |
| IT-4 | Streaming: error -> MCP notification | Streaming enabled, extension sends error | MCP notification written to stdout | must |
| IT-5 | Streaming: redaction applied | Error with Authorization header, streaming enabled | Notification context has header redacted | must |
| IT-6 | Streaming + passive: both work simultaneously | Streaming enabled, alerts also in observe | Notifications emitted AND alerts in observe response | should |
| IT-7 | CI webhook idempotency | POST same commit twice | No duplicate alert, existing updated | must |
| IT-8 | Full alert lifecycle | Generate alert -> observe drains it -> subsequent observe has no alerts | Complete drain lifecycle | must |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Alert generation speed | Time to append one alert to buffer | O(1), < 0.1ms | must |
| PT-2 | Alert drain speed | Time to copy and clear 50 alerts | O(n) where n<=50, < 1ms | must |
| PT-3 | CI webhook response time | Time from POST receipt to 200 response | < 5ms | must |
| PT-4 | Anomaly detection overhead | Time for error frequency check on new log entry | O(1), < 0.1ms | must |
| PT-5 | Notification emission speed | Time to serialize and write one MCP notification | < 1ms | must |
| PT-6 | Throttle/dedup check overhead | Time to check throttle/dedup state | < 0.1ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Alert buffer full, new alert arrives | 50 alerts in buffer + 1 new | Oldest evicted, no meta-alert about eviction | must |
| EC-2 | No observe call ever made | Alerts accumulate but no observe called | Buffer fills to 50, oldest evicted, no crash | must |
| EC-3 | CI webhook empty failures array | `{"status": "success", "failures": []}` | Valid, alert generated with success status | must |
| EC-4 | Correlation window edge | Regression at T=0, error at T=5.1s | NOT correlated (outside 5s window) | must |
| EC-5 | Correlation window match | Regression at T=0, error at T=4.9s | Correlated into compound alert | must |
| EC-6 | Alert with all fields null/empty | Minimal alert structure | Alert stored, no crash on serialization | should |
| EC-7 | Streaming enabled with no events matching filter | `events: ["ci"]` but no CI webhooks | No notifications emitted, no errors | must |
| EC-8 | Streaming disable clears pending batch | 5 events pending in batch, disable called | All pending cleared, confirmation shows pending_cleared: 5 | must |
| EC-9 | Rate limit resets at minute boundary | 12 events in minute 1, then 1 event in minute 2 | Event in minute 2 emitted (counter reset) | must |
| EC-10 | Dedup window expires | Same event at T=0s and T=35s | Both emitted (30s dedup window expired) | must |
| EC-11 | Very rapid alert generation (1000/sec) | Error storm generating alerts | Buffer capped at 50, no memory growth, no crash | must |
| EC-12 | Concurrent CI webhook + observe | Webhook receipt during observe drain | alertMu prevents race condition | must |
| EC-13 | Streaming with url_filter matching nothing | `url_filter: "/nonexistent/"` | No notifications, no errors | should |
| EC-14 | MCP notification write failure | stdout closed or blocked | Error logged, notification dropped gracefully | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test web app capable of generating errors and performance regressions
- [ ] curl or httpie available for CI webhook testing

### Step-by-Step Verification

#### Passive Alerts (Default Mode)

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "observe", "arguments": {"what": "errors"}}` | No browser activity | Single content block, no alerts section | [ ] |
| UAT-2 | Trigger a performance regression (add heavy script, reload) | Page loads slowly | Extension captures slow performance snapshot | [ ] |
| UAT-3 | `{"tool": "observe", "arguments": {"what": "errors"}}` | AI receives response | Two content blocks: normal errors + "--- ALERTS (1) ---" with regression alert | [ ] |
| UAT-4 | `{"tool": "observe", "arguments": {"what": "errors"}}` again | AI receives response | Single content block (alerts were drained on previous call) | [ ] |
| UAT-5 | Trigger error spike (5+ errors in 2 seconds) | Console shows rapid errors | Extension sends error burst | [ ] |
| UAT-6 | `{"tool": "observe", "arguments": {"what": "errors"}}` | AI receives response | Anomaly alert present: "error spike detected" | [ ] |
| UAT-7 | Post CI result: `curl -X POST http://localhost:7890/ci-result -H 'Content-Type: application/json' -d '{"status":"failure","source":"github-actions","summary":"2 tests failed","failures":[{"name":"test_login","message":"Expected 200, got 401"}]}'` | Human runs curl command | 200 OK response | [ ] |
| UAT-8 | `{"tool": "observe", "arguments": {"what": "errors"}}` | AI receives response | CI failure alert present with test summary | [ ] |

#### Active Streaming Mode

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-9 | `{"tool": "configure", "arguments": {"what": "configure_streaming", "streaming": {"action": "enable", "events": ["errors", "performance"]}}}` | AI receives confirmation | Streaming enabled for errors and performance | [ ] |
| UAT-10 | Trigger a JavaScript error in the test app | Console shows error | MCP notification appears on stdout without observe call | [ ] |
| UAT-11 | `{"tool": "configure", "arguments": {"what": "configure_streaming", "streaming": {"action": "status"}}}` | AI receives status | Config shows enabled=true, events=["errors","performance"], notification count > 0 | [ ] |
| UAT-12 | `{"tool": "configure", "arguments": {"what": "configure_streaming", "streaming": {"action": "disable"}}}` | AI receives confirmation | Streaming disabled, pending_cleared count shown | [ ] |
| UAT-13 | Trigger another error | Console shows error | NO MCP notification (streaming disabled) | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | No auth headers in alerts | Trigger network error with auth headers, check alert content | No Authorization, Cookie, or Bearer values in alert detail | [ ] |
| DL-UAT-2 | No request bodies in alerts | Trigger POST failure with JSON body, check alert | No request body content in alert | [ ] |
| DL-UAT-3 | CI webhook stores no secrets | Post CI result with `url` field, inspect stored alert | Only public-facing URL stored, no tokens | [ ] |
| DL-UAT-4 | Notification context redacted | Enable streaming, trigger network error with auth header | Notification context has `Authorization: [REDACTED]` | [ ] |
| DL-UAT-5 | Alert summary uses normalized URLs | Trigger regression on URL with query params | Alert summary shows clean path, no `?token=...` | [ ] |

### Regression Checks
- [ ] Existing `observe` response format unchanged when no alerts (single content block)
- [ ] Existing observe functionality (errors, network, etc.) unaffected by alert mechanism
- [ ] Alert generation does not slow down error/network processing
- [ ] CI webhook does not interfere with other HTTP endpoints
- [ ] Streaming disable works immediately (no lingering notifications)
- [ ] Server memory stable under sustained alert generation

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
