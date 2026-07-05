---
status: proposed
scope: feature/rate-limiting/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-rate-limiting
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Rate Limiting & Circuit Breaker

> QA plan for the Rate Limiting & Circuit Breaker feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification. This feature encompasses server-side rate limiting, server-side circuit breaker, extension-side exponential backoff, and extension-side circuit breaker.

---

## 1. Data Leak Analysis

**Goal:** Verify the rate limiting feature does NOT expose data it shouldn't. Rate limit responses and health endpoints must not leak captured telemetry data or internal server state beyond what is operationally necessary.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | 429 response body exposes captured data | 429 JSON response must contain only rate limit metadata, NOT any buffered telemetry data | high |
| DL-2 | Health endpoint exposes buffer contents | Circuit breaker health response must report `current_rate` and `memory_bytes`, NOT buffer entries | high |
| DL-3 | Rate limit error leaks internal thresholds to adversary | 429 response includes `current_rate` and `threshold` -- verify this is acceptable for localhost-only deployment | medium |
| DL-4 | Extension backoff state leaks to page context | Extension backoff state (`consecutiveFailures`, `circuitOpen`) must not be accessible from page-level JavaScript | medium |
| DL-5 | Buffered data during backoff exposed to extension content scripts | Data buffered in extension during backoff must stay in `background.js` context, not content scripts | medium |
| DL-6 | Retry-After header exposes server timing | `Retry-After` value reveals server load characteristics -- acceptable for localhost but verify no additional info leaked | low |
| DL-7 | Per-tool rate limit responses leak tool existence | Rate limit error for a tool includes tool name -- verify this is acceptable when combined with tool allowlisting | medium |
| DL-8 | Circuit breaker reason exposes memory details | Health `reason: "memory_exceeded"` with `memory_bytes` reveals server memory usage -- verify acceptable for localhost | low |

### Negative Tests (must NOT leak)
- [ ] 429 response body contains only `error`, `message`, `retry_after_ms`, `circuit_open`, `current_rate`, `threshold` -- no captured data
- [ ] Health endpoint circuit breaker state contains only `circuit_open`, `opened_at`, `current_rate`, `memory_bytes`, `reason` -- no buffer contents
- [ ] Extension backoff state is not accessible via `window.kaboom` or any injected global
- [ ] Data buffered during extension backoff is not exposed to content scripts or page JavaScript
- [ ] Per-tool rate limit error does not reveal names of OTHER tools or their limits

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading rate limit errors and health data can understand the situation and respond appropriately (back off, retry, alert the user).

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | 429 error message is actionable | Message includes rate, threshold, and retry time | [ ] |
| CL-2 | Per-tool rate limit error includes tool name | Error data has `tool` field so LLM knows which tool is limited | [ ] |
| CL-3 | Per-tool rate limit includes retry timing | `retry_after_seconds` field tells LLM exactly how long to wait | [ ] |
| CL-4 | MCP error code is standard | Code `-32029` is in MCP application error range, distinct from method-not-found (-32601) | [ ] |
| CL-5 | Circuit breaker state is clearly communicated | Health response `circuit_open: true/false` is unambiguous | [ ] |
| CL-6 | Circuit breaker reason is one of known enums | `reason`: `"rate_exceeded"`, `"memory_exceeded"` -- not free-form text | [ ] |
| CL-7 | Distinction between HTTP rate limit and MCP rate limit | HTTP 429 (extension ingestion) vs MCP error -32029 (tool calls) are clearly different | [ ] |
| CL-8 | Rate limit window is documented | Error includes `window: "1m"` so LLM knows the limit period | [ ] |
| CL-9 | Global vs per-tool rate limit distinction | Per-tool errors reference specific tool; global 429 references overall event rate | [ ] |
| CL-10 | Health endpoint reports rate limit configuration | LLM can check configured limits via health metrics | [ ] |

### Common LLM Misinterpretation Risks
- [ ] Risk: LLM interprets HTTP 429 from extension ingestion as "tool call failed" -- verify MCP tool calls are unaffected by HTTP rate limits
- [ ] Risk: LLM interprets per-tool rate limit as "server is down" -- verify error message says "rate limit exceeded" not "service unavailable"
- [ ] Risk: LLM retries immediately instead of waiting for `retry_after_seconds` -- verify retry hint is prominent in error
- [ ] Risk: LLM confuses `circuit_open: true` (server health) with extension circuit breaker -- verify they are in different response contexts
- [ ] Risk: LLM tries to "fix" rate limiting by calling configure tool to change limits -- verify this requires server restart
- [ ] Risk: LLM assumes data is lost during rate limiting -- verify that extension buffers data locally during backoff

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low (mostly automatic behavior)

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Default rate limiting | 0 steps: enabled by default at 1000 events/sec | No -- zero-config |
| Custom global rate limit | 1 step: `--rate-limit=2000` | No -- already minimal |
| Per-tool rate limits | 1 step: `--rate-limits="analyze=10,generate=5"` | No -- already minimal |
| Check circuit breaker state | 1 step: extension polls `/health` OR AI calls `configure({action:"health"})` | No -- already minimal |
| Recovery from rate limiting | 0 steps: automatic (extension backoff + server window reset) | No -- fully automatic |
| Recovery from circuit breaker | 0 steps: automatic (wait for conditions to clear) | No -- fully automatic |
| Diagnose rate limiting issues | 1 step: AI calls `configure({action:"health"})` | No -- already minimal |

### Default Behavior Verification
- [ ] Global rate limit is 1000 events/sec by default with zero configuration
- [ ] Circuit breaker opens after 5 consecutive seconds over threshold
- [ ] Circuit breaker closes when rate below threshold for 10 seconds AND memory below 30MB
- [ ] Extension backoff starts at 100ms and escalates: 100ms, 500ms, 2000ms, circuit break at 30s
- [ ] Extension continues capturing data locally during backoff (no data loss)
- [ ] A single successful POST resets extension backoff to zero
- [ ] Per-tool rate limits are not applied unless configured
- [ ] Non-ingest endpoints (GET queries, MCP tools) are never subject to HTTP rate limiting

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Under threshold: all accepted | 999 events in 1 second | All return 200 | must |
| UT-2 | At threshold + 1: rate limited | 1001 events in 1 second | 1001st returns 429 | must |
| UT-3 | Rate counter resets after 1 second | 1001 events (limited), wait 1s, send 1 event | Event accepted (200) | must |
| UT-4 | Event count increments by batch size | POST with 10 events in array | Counter increments by 10, not 1 | must |
| UT-5 | Circuit breaker stays closed on single spike | 2000 events/sec for 1 second only | Circuit remains closed | must |
| UT-6 | Circuit breaker opens after 5 consecutive seconds | >1000 events/sec for 5 consecutive seconds | `circuitOpen: true` | must |
| UT-7 | Circuit breaker: all ingest returns 429 | Circuit open + any ingest POST | Immediate 429 (no processing) | must |
| UT-8 | Circuit breaker closes: rate recovery | Rate below threshold for 10s + memory below 30MB | `circuitOpen: false` | must |
| UT-9 | Circuit breaker opens: memory exceeded | Memory > 50MB | `circuitOpen: true`, `reason: "memory_exceeded"` | must |
| UT-10 | Circuit breaker requires BOTH conditions to close | Rate below for 10s but memory > 30MB | Circuit stays open | must |
| UT-11 | 429 response format | Rate limited request | JSON body with all required fields + `Retry-After` header | must |
| UT-12 | `Retry-After` header value | Rate limited at T | Seconds until next window (rounded up) | must |
| UT-13 | Health endpoint circuit state | Circuit open | Health returns `circuit_open: true` with reason and timestamps | must |
| UT-14 | Health endpoint normal state | Circuit closed | Health returns `circuit_open: false` with current rate | must |
| UT-15 | Rate limit applies across all ingest endpoints | Mixed POSTs to /websocket-events, /network-bodies, /enhanced-actions | Combined count against threshold | must |
| UT-16 | Non-ingest endpoints NOT rate limited | GET /health during rate limiting | Returns 200, not 429 | must |
| UT-17 | MCP tool calls NOT affected by HTTP rate limit | Circuit open + MCP observe call | Tool works normally | must |
| UT-18 | Per-tool rate limit: under limit | 19 calls for 20/min limit | All succeed | must |
| UT-19 | Per-tool rate limit: at limit + 1 | 21 calls for 20/min limit | 21st returns MCP error -32029 | must |
| UT-20 | Per-tool rate limit: error format | Rate limited MCP call | Error with code, message, tool, limit, window, retry_after_seconds | must |
| UT-21 | Per-tool rate limit: independent per tool | analyze at limit, observe not | analyze fails, observe succeeds | must |
| UT-22 | Per-tool rate limit: window reset | Hit limit, wait 60s | Calls accepted again | must |
| UT-23 | Sliding window counter: monotonic time | Counter uses monotonic time | No clock skew issues | must |
| UT-24 | Custom global threshold | `--rate-limit=500` | Rate limiting at 500 events/sec | must |
| UT-25 | Custom per-tool limits | `--rate-limits="analyze=10"` | analyze limited at 10/min | must |

### 4.2 Integration Tests (Extension)

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Extension backoff on first 429 | background.js + server | Wait 100ms before next attempt | must |
| IT-2 | Extension backoff escalation | background.js + repeated 429s | 100ms -> 500ms -> 2000ms escalation | must |
| IT-3 | Extension circuit breaker opens | background.js + 5 consecutive failures | No POSTs for 30 seconds | must |
| IT-4 | Extension probe after circuit timeout | background.js + 30s timer | Single probe batch sent | must |
| IT-5 | Extension probe success closes circuit | background.js + probe returns 200 | Circuit closes, buffer drains | must |
| IT-6 | Extension probe failure re-opens circuit | background.js + probe returns 429 | Circuit re-opens for 30s | must |
| IT-7 | Extension successful POST resets backoff | background.js + 200 response | consecutiveFailures = 0, backoffMs = 0 | must |
| IT-8 | Extension buffers data during backoff | background.js local buffer | Data captured, not discarded | must |
| IT-9 | Extension retry budget: 3 per batch | background.js + 3 failures | Batch abandoned after 3rd failure | must |
| IT-10 | Extension buffer drain after recovery | background.js + circuit close | Buffered data sent in batches | must |
| IT-11 | Extension handles network error same as 429 | Server unavailable (connection refused) | Same backoff behavior as 429 | must |
| IT-12 | Extension new background script: fresh state | Extension upgrade during circuit break | New script starts with no backoff | should |
| IT-13 | Multiple tabs: independent backoff | Two tabs sending data | Each tab has own backoff state | must |
| IT-14 | Multiple tabs: shared server rate limit | 10 tabs each sending 150 events/sec | Server rate limits at 1500 total | must |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Rate check overhead per request | Latency added | < 0.01ms | must |
| PT-2 | 429 response generation | Latency | < 0.1ms | must |
| PT-3 | Circuit state check | Latency | < 0.01ms | must |
| PT-4 | Sustained 1000 events/sec (at threshold) | CPU usage | < 5% overhead | must |
| PT-5 | Extension backoff timer | CPU while waiting | 0% (setTimeout, no polling) | must |
| PT-6 | Memory for rate tracking | Bytes | < 100 bytes | must |
| PT-7 | Per-tool rate limit check | Latency | < 0.01ms (atomic counter) | must |
| PT-8 | Server under heavy load (5000 events/sec) | Response time for 429 | < 1ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Burst then calm | 5000 events/sec for 2s, then silence | Rate limiting during burst, no circuit open (< 5 consecutive seconds) | must |
| EC-2 | Slow trickle at memory limit | 500 events/sec (under rate) with 10KB bodies each | Memory enforcement handles this, not rate limiting | must |
| EC-3 | Server restart during extension backoff | Extension in circuit break + server restarts | Extension timer expires, probe succeeds, normal flow resumes | must |
| EC-4 | Exactly at threshold (boundary) | Exactly 1000 events in 1 second | All accepted (limit is >1000) | must |
| EC-5 | Zero events then burst | 0 events for 60s, then 2000 in 1s | Rate limiting starts immediately, no warm-up needed | must |
| EC-6 | Circuit open for extended period | Circuit stays open for 5 minutes | No resource leak, clean state tracking | must |
| EC-7 | Extension upgrade during circuit break | New background.js loaded | Fresh state, no backoff | should |
| EC-8 | Multiple ingest endpoints simultaneously | Parallel POSTs to all 3 endpoints totaling > 1000 | Combined count triggers rate limit | must |
| EC-9 | Single large batch (1000 events in one POST) | POST with 1000 events | Counter increments by 1000; if threshold hit, returns 429 | must |
| EC-10 | Rate window boundary race | Events arrive exactly at window reset boundary | No missed counting or double counting | must |
| EC-11 | Memory at exactly 50MB | Memory exactly at hard limit | Circuit opens | must |
| EC-12 | Memory at exactly 30MB during recovery | Rate below threshold + memory exactly 30MB | Circuit should close (30MB is the closing threshold) | must |
| EC-13 | Per-tool rate limit: tool with no explicit limit | Tool not in rate-limits config | No per-tool limit (unlimited) | must |
| EC-14 | Per-tool rate limit: all tools configured | Every tool has a limit | All limits enforced independently | must |
| EC-15 | Extension retry budget exhausted | 3 retries fail for one batch | Batch abandoned, next batch starts fresh budget | must |
| EC-16 | Extension buffer overflow during extended backoff | 60+ seconds of backoff with high data volume | Oldest entries evicted from local buffer (not memory leak) | must |
| EC-17 | Global circuit breaker + per-tool rate limit | Both active simultaneously | Both enforced independently (different paths) | must |
| EC-18 | Per-tool rate limit across server restart | Limits configured, server restarts | Counters reset (fresh start) | must |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A web application that generates significant telemetry (console logs, network requests)
- [ ] A tool to generate high-volume HTTP traffic (e.g., `ab`, `wrk`, or a simple script)

### Server-Side Rate Limiting UAT

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | AI checks health: `{"tool": "observe", "params": {"category": "health"}}` | MCP response | `circuit_open: false`, current_rate shown | [ ] |
| UAT-2 | Human generates high-volume traffic: `for i in $(seq 1 2000); do curl -s -X POST http://localhost:7890/v4/websocket-events -d '{"events":[{"type":"test"}]}' & done` | Terminal | Some requests return 429 | [ ] |
| UAT-3 | Human inspects a 429 response body | Terminal output | JSON with `error: "rate_limited"`, `retry_after_ms`, `current_rate`, `threshold` | [ ] |
| UAT-4 | AI checks health again | MCP response | `current_rate` elevated, possibly circuit_open if sustained | [ ] |
| UAT-5 | AI calls observe (MCP tool) | MCP response | Works normally -- MCP is not affected by HTTP rate limits | [ ] |
| UAT-6 | Wait 15 seconds for recovery | Timer | Server rate drops | [ ] |
| UAT-7 | Human sends single request | Terminal | Returns 200 (rate recovered) | [ ] |

### Circuit Breaker UAT

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-8 | Human generates sustained high traffic for 6+ seconds | Terminal | Multiple 429s | [ ] |
| UAT-9 | AI checks health | MCP response | `circuit_open: true`, `reason` field present | [ ] |
| UAT-10 | Human sends single request while circuit is open | Terminal | Immediate 429 (no processing) | [ ] |
| UAT-11 | Wait for circuit to close (~10s of low rate + memory ok) | Timer | Circuit recovers | [ ] |
| UAT-12 | AI checks health | MCP response | `circuit_open: false` | [ ] |
| UAT-13 | Human sends request | Terminal | Returns 200 | [ ] |

### Extension Backoff UAT

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-14 | Trigger server rate limiting while extension is sending data | Extension status + server | Extension begins receiving 429s | [ ] |
| UAT-15 | Observe extension behavior | Chrome DevTools (background.js console) | Backoff messages: increasing delays logged | [ ] |
| UAT-16 | Wait for server to recover | Timer | Server stops sending 429s | [ ] |
| UAT-17 | Observe extension recovery | Extension status | Extension resumes sending, backoff resets | [ ] |
| UAT-18 | AI calls observe | MCP response | Data flows normally again; buffered data during backoff may now be visible | [ ] |

### Per-Tool Rate Limiting UAT

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-19 | Restart server with: `./dist/kaboom --port 7890 --rate-limits="observe=5"` | Server startup | Per-tool limits active | [ ] |
| UAT-20 | AI calls observe 5 times rapidly | MCP responses | All 5 succeed | [ ] |
| UAT-21 | AI calls observe a 6th time | MCP response | Error: `-32029`, `"Rate limit exceeded for tool 'observe': 5/min"` | [ ] |
| UAT-22 | AI checks retry_after_seconds in error | Error data | `retry_after_seconds` field present with seconds until reset | [ ] |
| UAT-23 | AI calls a different tool (e.g., generate) | MCP response | Works normally (different tool, different limit) | [ ] |
| UAT-24 | Wait 60 seconds for window reset | Timer | Window resets | [ ] |
| UAT-25 | AI calls observe again | MCP response | Works normally | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | 429 response has no captured data | Inspect 429 JSON body | Only rate limit metadata fields | [ ] |
| DL-UAT-2 | Health endpoint has no buffer contents | Call `configure({action:"health"})` | Memory sizes and counts only | [ ] |
| DL-UAT-3 | Extension backoff state not in page context | Open browser console on page, type `window.kaboom` | Undefined or no backoff state exposed | [ ] |
| DL-UAT-4 | Per-tool error does not reveal other tools' limits | Inspect -32029 error | Only the limited tool's info shown | [ ] |
| DL-UAT-5 | Circuit breaker health has no buffer data | Inspect health response during circuit open | Only circuit state + operational metrics | [ ] |

### Regression Checks
- [ ] Normal operation (under threshold) has zero observable impact on request latency
- [ ] Extension data capture continues during backoff (no data loss, only delay)
- [ ] MCP tool calls are completely unaffected by HTTP rate limiting
- [ ] Circuit breaker does not affect non-ingest endpoints (GET /health)
- [ ] Per-tool rate limits do not interfere with HTTP rate limits (independent systems)
- [ ] Extension backoff resets correctly on first success after failure
- [ ] Server with no `--rate-limits` flag has no per-tool limits (default: unlimited per tool)
- [ ] Rate limiting uses monotonic time (no clock skew issues)

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
