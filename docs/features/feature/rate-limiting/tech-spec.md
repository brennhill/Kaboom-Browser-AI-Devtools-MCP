---
status: shipped
scope: feature/rate-limiting/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-rate-limiting
last_reviewed: 2026-11-21
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-rate-limiting.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Rate Limiting Review](./index.md).

# Technical Spec: Rate Limiting & Circuit Breaker

## Purpose

A busy browser app can overwhelm the Kaboom server. A React app in development mode might emit hundreds of console logs per second, a trading dashboard might push thousands of WebSocket messages per second, or a runaway network polling loop might flood the server with body captures. Without protection, the server's memory grows unbounded and response latency spikes.

Rate limiting and circuit breaker logic ensures the server degrades gracefully under load — rejecting excess data with 429 responses rather than consuming it — and that the extension backs off intelligently rather than hammering a struggling server.

---

## How It Works

### Server-Side: Rate Limiting

The server tracks ingest rate using a sliding window counter. Each HTTP ingest endpoint (WebSocket events, network bodies, enhanced actions) increments a counter. Every second, the counter resets.

When the counter exceeds the threshold (1000 events/second), the server responds with HTTP 429 and a `Retry-After` header. The response body contains a JSON object explaining the rejection so the extension can log it.

The threshold applies globally across all ingest endpoints combined — not per-endpoint. This prevents a scenario where WebSocket floods are ignored because only the WS endpoint is rate-limited while network bodies still flow freely.

### Server-Side: Circuit Breaker

The circuit breaker is a separate mechanism from rate limiting, implemented by `CircuitBreaker` in `internal/ingestion/circuit/breaker.go`:

1. **Rate**: More than 1000 events/second (`RateLimitThreshold`) sustained for 5 consecutive seconds (`CircuitOpenStreakCount`) → circuit opens with reason `rate_exceeded`
2. **Forced open**: Other subsystems may call `ForceOpen(reason)`; memory pressure is handled by memory enforcement, not by a breaker threshold
3. **Recovery**: Rate below threshold for 10 seconds (`CircuitCloseSeconds`) → circuit closes

When the circuit is open, ALL ingest endpoints return 429 immediately (no processing). This protects the server from cascading failures where rate-limited requests still consume parsing resources.

The circuit state is exposed in the server's health response so the extension can detect it without waiting for individual 429s.

### Extension-Side: Exponential Backoff

When the extension receives a 429 response (or a network error suggesting the server is down), it enters backoff mode:

1. First failure: no delay
2. From the second consecutive failure: wait `1000ms × 2^(failures−2)` (1s, 2s, 4s, ...), capped at 30s
3. After 5 consecutive failures: circuit opens and sending pauses for 30 seconds (`createCircuitBreaker` in `src/background/sync/circuit-breaker.ts`)

During backoff, the extension continues capturing data into its local buffers. It does not discard — it buffers up to its memory cap and starts evicting oldest entries if needed.

Recovery: A single successful POST resets the backoff to zero. The extension drains any buffered data in batches.

### Extension-Side: Circuit Breaker

If the extension hits 5 consecutive failures (429s or network errors), it opens its own circuit breaker:

- Stops all POST attempts for 30 seconds
- After 30 seconds, sends a single "probe" batch
- If the probe succeeds: close the circuit, drain buffer
- If the probe fails: re-open the circuit for another 30 seconds

The extension never retries indefinitely. It has a total retry budget of 3 attempts per batch before moving on.

---

## Data Model

### Server State

The `Capture` struct tracks:
- `eventCount`: Events received in the current 1-second window
- `rateResetTime`: When the current window started
- `rateLimitStreak`: Consecutive seconds where rate exceeded threshold
- `circuitOpen`: Boolean, whether the circuit breaker is currently open
- `circuitOpenedAt`: When the circuit was opened (for recovery timing)
- `lastRateLimitTime`: When the server last returned a 429

### Extension State

The background script tracks:
- `consecutiveFailures`: Count of back-to-back POST failures
- `backoffMs`: Current backoff delay (100, 500, 2000, or "paused")
- `circuitOpen`: Boolean
- `circuitOpenedAt`: Timestamp for the 30-second pause
- `retryBudget`: Remaining retries for current batch (starts at 3)

---

## Server Behavior

### Ingest Endpoints Affected

All three ingest endpoints participate in rate limiting:
- `POST /websocket-events`
- `POST /network-bodies`
- `POST /enhanced-actions`

Each POST increments the global event counter by the number of events in the batch (not just "1 per request").

### 429 Response Format

```json
{
  "error": "rate_limited",
  "message": "Server receiving >1000 events/sec. Retry after backoff.",
  "retry_after_ms": 1000,
  "circuit_open": false,
  "current_rate": 1247,
  "threshold": 1000
}
```

The `Retry-After` HTTP header is also set (fixed at `1` second).

### Health Endpoint

`GET /health` returns circuit breaker state (`GetHealthStatus`):

```json
{
  "circuit_open": true,
  "opened_at": "2026-01-24T10:30:00Z",
  "current_rate": 2400,
  "reason": "rate_exceeded"
}
```

The extension can poll this cheaply to detect circuit state changes without waiting for a batch to fail.

---

## Edge Cases

- **Burst then calm**: A 2-second burst of 5000 events/sec followed by silence. The circuit should NOT open (requires 5 consecutive seconds). Rate limiting kicks in during the burst but resets quickly.
- **Slow trickle at memory limit**: 500 events/sec (under rate threshold) but each event is large (10KB bodies). Memory enforcement (separate spec) handles this, not rate limiting.
- **Server restart during backoff**: Extension's circuit breaker timer expires, probe succeeds, normal flow resumes. No special handling needed.
- **Multiple browser tabs**: Each tab's extension instance has its own backoff state. The server's rate limit is global across all tabs. This is correct — if 10 tabs each send 150 events/sec, the server correctly rate-limits at 1500 total.
- **Extension upgrade during circuit break**: New background script starts fresh with no backoff. This is fine — the server still rate-limits if needed.
- **Clock skew**: The 1-second window is based on server monotonic time, not wall clock. No skew risk.

---

## Performance Constraints

- Rate check overhead per request: under 0.01ms (atomic counter read + compare)
- 429 response generation: under 0.1ms (pre-formatted JSON template)
- Circuit state check: under 0.01ms (boolean read)
- Extension backoff timer: uses `setTimeout`, zero CPU while waiting
- Memory overhead for rate tracking: under 100 bytes (two integers + timestamp)

---

## Test Scenarios

### Server Tests

1. Under 1000 events/sec → all requests accepted (200)
2. Exactly 1001 events in one second → 1001st returns 429
3. Rate resets after 1 second → requests accepted again
4. 5 consecutive seconds over threshold → circuit opens
5. Circuit open → all ingest endpoints return 429 immediately
6. Rate drops below threshold for 10 seconds → circuit closes
7. 429 response contains correct JSON body and Retry-After header
8. Event count increments by batch size, not request count
9. Health endpoint returns circuit state accurately
10. Circuit doesn't open on a single spike (< 5 consecutive seconds)
11. Rate limit applies to all three ingest endpoints combined
12. Non-ingest endpoints (GET queries, MCP tools) are never rate-limited

### Extension Tests

13. Single 429 → no backoff
14. Second consecutive 429 → backoff 1000ms
15. Third consecutive 429 → backoff 2000ms (doubling, capped at 30s)
16. Fifth consecutive failure → circuit opens, 30-second pause
17. Successful POST after failure → backoff resets to 0
18. Circuit open → no POSTs for 30 seconds
19. After 30 seconds → single probe sent
20. Probe succeeds → circuit closes, buffer drains
21. Probe fails → circuit re-opens for another 30 seconds
22. During backoff → data still captured to local buffer
23. Retry budget of 3 per batch → after 3 failures, batch is abandoned (not retried forever)

---

## File Locations

Server implementation: `internal/ingestion/circuit/breaker.go` (owned by `internal/composition/capture/capture.go`, checked in `internal/ingestion/http/handlers.go`) with tests in `internal/ingestion/circuit/breaker_test.go`.

Extension implementation: `src/background/sync/circuit-breaker.ts` with tests in `tests/extension/reliability/circuit-breaker.test.js` and `tests/extension/performance/rate-limit.test.js`.
