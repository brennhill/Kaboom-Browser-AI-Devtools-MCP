---
status: proposed
scope: feature/context-streaming/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-context-streaming
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Context Streaming

> QA plan for the Context Streaming feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Context Streaming pushes MCP notifications to stdout in real time. This changes the data flow from pull (AI calls observe) to push (server writes to stdout). The push path must apply the SAME redaction rules as the pull path.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Notification includes unredacted Authorization headers | When a network error triggers a notification, verify the notification `detail` and `data` fields do NOT contain Authorization, Cookie, or API key headers | critical |
| DL-2 | Notification includes unredacted request/response bodies | Network error notifications must NOT contain raw request/response bodies unless body capture is explicitly opt-in | critical |
| DL-3 | URL query parameters with sensitive keys are exposed | Notification URLs containing `?token=`, `?api_key=`, `?secret=` must have values masked (e.g., `?token=***`) | critical |
| DL-4 | Notification data written to stdout is captured by other processes | stdout is a local pipe between Kaboom and MCP client; verify no additional logging of notification content to disk files | high |
| DL-5 | Error message in notification contains sensitive stack trace data | Console error notifications may include stack traces with file paths, but should not include variable values or secrets | high |
| DL-6 | CI webhook data in notification leaks build secrets | CI event notifications from `/ci-result` must not include environment variables, secrets, or build tokens from the webhook payload | high |
| DL-7 | Notification `source` field leaks internal server details | The `source` field (e.g., `anomaly_detector`) should be a safe label, not an internal implementation path | low |
| DL-8 | Dedup cache stores sensitive event data in memory | `SeenMessages` cache stores dedup keys (`category:title`); verify titles are redacted before being used as keys | medium |
| DL-9 | PendingBatch stores unredacted events during throttle window | Events batched during throttle must be redacted at construction time, not at emission time | high |
| DL-10 | Streaming state persists after MCP client disconnect | When client disconnects, verify streaming state is cleaned up and no notifications leak to a subsequent client | medium |

### Negative Tests (must NOT leak)
- [ ] Notification `data` field must NOT contain raw HTTP headers (Authorization, Cookie, Set-Cookie, X-API-Key)
- [ ] Notification `data` field must NOT contain request/response bodies unless body capture is enabled
- [ ] Notification must NOT be written to the JSONL audit log file (only setting changes are audited, not streaming content)
- [ ] Dedup cache keys must NOT contain unredacted sensitive data
- [ ] `/ci-result` webhook-triggered notifications must NOT echo back the full webhook payload

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the notifications can unambiguously understand the data and take appropriate action.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Notification category is unambiguous | Each notification includes a `category` field mapping to a clear event type (e.g., `network_errors`, `performance`) | [ ] |
| CL-2 | Severity is actionable | `severity` field uses standard levels (`info`, `warning`, `error`) that map to urgency | [ ] |
| CL-3 | Title is a self-contained summary | The `title` field (e.g., "Server error: POST /api/users -> 500") is sufficient for the AI to understand the event without reading `detail` | [ ] |
| CL-4 | Enable response confirms configuration | After `streaming_action: "enable"`, response echoes back the full effective configuration | [ ] |
| CL-5 | Disable response confirms cleanup | After `streaming_action: "disable"`, response shows `pending_cleared` count so AI knows how many batched events were dropped | [ ] |
| CL-6 | Status response shows emission stats | `notify_count` and `pending` fields help the AI gauge whether streaming is working and how much activity is flowing | [ ] |
| CL-7 | Notification format matches MCP spec | Notifications use `notifications/message` method with `level`, `logger`, `data` — standard MCP fields | [ ] |
| CL-8 | Event category filter values are documented in error | If AI passes an invalid `events` value, error lists valid options | [ ] |
| CL-9 | Throttle behavior is transparent | The AI can infer from `pending` count in status that events are being batched during the throttle window | [ ] |
| CL-10 | Notification vs observe() data is distinguishable | Notifications carry `source: "anomaly_detector"` or similar; observe() data carries raw entries. The AI should not confuse them | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI may think streaming replaces observe() — verify notifications are summaries/alerts, not full data; AI still needs observe() for details
- [ ] AI may enable streaming with `events: ["all"]` and be overwhelmed — verify throttle and rate limit prevent flooding
- [ ] AI may not process notifications between tool calls if the MCP client buffers them — this is a client-side concern but test with Claude Code
- [ ] AI may confuse `throttle_seconds` (minimum gap between notifications) with `rate limit` (12/min hard cap) — verify both are documented in error responses when hit
- [ ] AI may interpret `pending_cleared: 0` on disable as "nothing was happening" when it actually means "no events were batched" — verify the semantics are clear

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Medium

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Enable streaming with defaults | 1 step: `configure(what: "streaming", streaming_action: "enable")` | No — already minimal |
| Enable with specific categories | 1 step: single call with `events` array | No — already batched |
| Disable streaming | 1 step: `configure(what: "streaming", streaming_action: "disable")` | No — already minimal |
| Check streaming status | 1 step: `configure(what: "streaming", streaming_action: "status")` | No — already minimal |
| Receive a notification | 0 steps: notification arrives automatically | No — this is the whole point |
| React to a notification | 1 step: AI calls `observe()` with relevant mode | Could auto-suggest observe calls in notification, but this is a future enhancement (OI-4) |

### Default Behavior Verification
- [ ] Streaming is OFF by default (AI must explicitly enable)
- [ ] Default throttle is 5 seconds (prevents flooding without configuration)
- [ ] Default severity minimum is "warning" (filters out info-level noise)
- [ ] Default events is `["all"]` (no category restriction by default when enabled)
- [ ] Passive alert mode (piggybacking on observe()) works independently of streaming state

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Enable streaming with defaults | `configure(what: "streaming", streaming_action: "enable")` | `StreamState.Enabled = true`, response shows config with defaults | must |
| UT-2 | Enable with specific events | `streaming_action: "enable", events: ["errors", "network_errors"]` | `StreamState.Events = ["errors", "network_errors"]` | must |
| UT-3 | Enable with custom throttle | `streaming_action: "enable", throttle_seconds: 10` | `StreamState.ThrottleSeconds = 10` | must |
| UT-4 | Enable with throttle out of range | `throttle_seconds: 0` or `throttle_seconds: 100` | Error: "throttle_seconds must be between 1 and 60" | must |
| UT-5 | Enable with invalid severity | `severity_min: "debug"` | Error listing valid severities | must |
| UT-6 | Enable with invalid event category | `events: ["invalid_cat"]` | Error listing valid event categories | must |
| UT-7 | Disable streaming | `streaming_action: "disable"` | `StreamState.Enabled = false`, pending batch cleared, dedup cache cleared | must |
| UT-8 | Status when enabled | `streaming_action: "status"` after enabling | Returns config, notify_count, pending count | must |
| UT-9 | Status when disabled | `streaming_action: "status"` before enabling | Returns `enabled: false` with zero counts | must |
| UT-10 | Severity filter: event below threshold | Warning event when `severity_min: "error"` | Event NOT emitted as notification | must |
| UT-11 | Severity filter: event meets threshold | Error event when `severity_min: "error"` | Event emitted as notification | must |
| UT-12 | Event category filter | Network error when `events: ["errors"]` only | Event NOT emitted (not in subscribed categories) | must |
| UT-13 | Throttle enforcement | Two events 1 second apart with `throttle_seconds: 5` | First emitted immediately; second added to PendingBatch | must |
| UT-14 | Rate limit enforcement (12/min) | Emit 13 events in 1 minute | First 12 emitted; 13th dropped | must |
| UT-15 | Dedup within 30-second window | Same event (`category:title`) twice within 30 seconds | First emitted; second suppressed | must |
| UT-16 | Dedup after window expires | Same event with 31-second gap | Both emitted | should |
| UT-17 | SeenMessages cache bounded at 500 | Insert 600 unique dedup keys | Cache stays at or below 500; oldest entries evicted | should |
| UT-18 | URL filter match | Network error with URL containing "api/users" when `url_filter: "api/users"` | Event passes filter | should |
| UT-19 | URL filter miss | Network error with URL "/api/products" when `url_filter: "api/users"` | Event filtered out | should |
| UT-20 | URL filter skipped for non-URL events | Console error (no URL) when `url_filter: "api/users"` | Event passes (URL filter not applied to errors) | must |
| UT-21 | Notification JSON format | Any qualifying event | Output is valid JSON-RPC 2.0 notification with `method: "notifications/message"` | must |
| UT-22 | Stdout serialization (no interleaving) | Concurrent notification and MCP response | Both are complete JSON lines; no partial writes | must |
| UT-23 | Redaction applied to notification data | Event with Authorization header | Header stripped before notification emission | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Enable streaming, trigger error, receive notification | MCP handler, StreamState, AlertBuffer, stdout writer | AI enables streaming; browser error triggers alert; alert passes filters; notification written to stdout | must |
| IT-2 | Passive and active modes coexist | AlertBuffer, StreamState, observe handler | Error triggers alert; alert added to AlertBuffer AND emitted as notification; observe() returns the alert via _alerts; streaming also emits it | must |
| IT-3 | Disable streaming mid-session | StreamState, PendingBatch | AI disables streaming; pending batch cleared; no further notifications emitted | must |
| IT-4 | Streaming survives extension reconnect | StreamState, extension polling | Extension disconnects and reconnects; streaming config unchanged; new events flow to notifications | should |
| IT-5 | MCP client disconnect stops streaming | StreamState, stdout pipe | stdin closes (client disconnects); stdout write fails; streaming goroutine stops cleanly | must |
| IT-6 | Batch flush after throttle window | StreamState, PendingBatch, throttle timer | Multiple events batch during throttle; batch flushed as notification when window expires | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Alert emission latency | Time from event received to notification written to stdout | < 5ms | must |
| PT-2 | Notification JSON serialization | Time to marshal notification to JSON | < 0.5ms | must |
| PT-3 | Configure tool response time | Time for enable/disable/status | < 10ms | must |
| PT-4 | StreamState memory overhead | Memory for config + SeenMessages cache + PendingBatch | < 500KB | must |
| PT-5 | Stdout write lock contention | Time the mcpStdoutMu is held for a single notification write | < 1ms | must |
| PT-6 | No regression on passive alert mode | observe() response time with streaming enabled vs disabled | No measurable difference | must |
| PT-7 | Dedup cache eviction performance | Time to prune 500-entry cache | < 1ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Enable streaming when already enabled | Call enable twice | Second call updates config (idempotent); no error | must |
| EC-2 | Disable streaming when already disabled | Call disable when not enabled | No-op; response shows `pending_cleared: 0` | must |
| EC-3 | 100 errors in 1 second | Burst of errors during throttle window | First triggers notification; rest batch (max 100 in PendingBatch); overflow silently dropped | must |
| EC-4 | No events for long period | Streaming enabled but no browser activity | No notifications emitted; status shows `notify_count: 0` | should |
| EC-5 | Rapid enable/disable/enable toggle | Three state changes in sequence | Each takes effect immediately; final state is enabled with fresh config | must |
| EC-6 | stdout pipe broken mid-write | Client crashes during notification write | Write error detected; streaming goroutine stops; no panic | must |
| EC-7 | Very long event title | Console error with 10KB message | Title truncated to reasonable length in notification | should |
| EC-8 | Extension disconnected, streaming enabled | No events arrive at server | No notifications emitted; streaming stays enabled waiting for events | should |
| EC-9 | SeenMessages overflow during error storm | 600+ unique error types in 30 seconds | Cache bounded at 500; oldest entries evicted; some dedup misses (acceptable) | should |
| EC-10 | URL filter with regex-like characters | `url_filter: "api/users?id=1"` | Treated as substring match (not regex); matches URLs containing that substring | should |
| EC-11 | Concurrent notification and tool response | Notification and observe() response written simultaneously | mcpStdoutMu ensures serialization; no JSON interleaving | must |
| EC-12 | Streaming across MCP reconnect (OI-3) | Client disconnects, new client connects | Streaming state should reset to off for new client (open item; verify behavior) | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test web application running (e.g., localhost:3000) with ability to trigger errors and network requests
- [ ] MCP client (Claude Code) that can receive and display `notifications/message`

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "configure", "arguments": {"what": "streaming", "streaming_action": "status"}}` | No visual change | AI receives `{ config: { enabled: false }, notify_count: 0, pending: 0 }` | [ ] |
| UAT-2 | `{"tool": "configure", "arguments": {"what": "streaming", "streaming_action": "enable", "events": ["errors", "network_errors"], "throttle_seconds": 5, "severity_min": "warning"}}` | No visual change | AI receives `{ status: "enabled", config: { enabled: true, events: [...], throttle_seconds: 5, ... } }` | [ ] |
| UAT-3 | Human triggers a JavaScript error on the page (e.g., `throw new Error("UAT streaming test")` in DevTools) | Error appears in DevTools console | Within 5 seconds, AI receives an MCP notification: `{ jsonrpc: "2.0", method: "notifications/message", params: { level: "warning", logger: "kaboom", data: { category: "errors", ... } } }` | [ ] |
| UAT-4 | Human triggers a 500 network error (e.g., fetch to a failing endpoint) | Network error in DevTools | AI receives a notification with `category: "network_errors"` | [ ] |
| UAT-5 | `{"tool": "configure", "arguments": {"what": "streaming", "streaming_action": "status"}}` | No visual change | `notify_count` is > 0; reflects notifications emitted in UAT-3 and UAT-4 | [ ] |
| UAT-6 | Human triggers the same error twice within 30 seconds | Two errors in DevTools | AI receives only ONE notification (second is deduped) | [ ] |
| UAT-7 | Human triggers 3 different errors within 2 seconds | Three errors in DevTools | AI receives first notification immediately; others are batched; batch flushed after throttle window | [ ] |
| UAT-8 | `{"tool": "configure", "arguments": {"what": "streaming", "streaming_action": "disable"}}` | No visual change | AI receives `{ status: "disabled", pending_cleared: N }` | [ ] |
| UAT-9 | Human triggers another error | Error in DevTools | AI does NOT receive a notification (streaming disabled) | [ ] |
| UAT-10 | `{"tool": "observe", "arguments": {"what": "errors"}}` | No visual change | AI receives the error from UAT-9 via normal observe (passive mode still works) | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Notification does not contain Authorization header | Trigger a network error for a request that had an Authorization header; check notification content | Notification `data` field contains URL and status but NOT the Authorization header value | [ ] |
| DL-UAT-2 | Notification URL has sensitive params masked | Trigger error for URL like `/api?token=secret123`; check notification | URL shows `/api?token=***` or similar masking | [ ] |
| DL-UAT-3 | Notification not written to disk | Check `~/.kaboom/` directory and JSONL log after streaming session | No notification content in any disk file (audit log only contains setting changes, not notification data) | [ ] |

### Regression Checks
- [ ] `observe()` returns the same data with streaming enabled vs disabled
- [ ] Passive alert piggybacking (`_alerts` in observe responses) works independently of streaming state
- [ ] Server startup and shutdown unaffected by streaming configuration
- [ ] Extension polling behavior unchanged by streaming state

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
