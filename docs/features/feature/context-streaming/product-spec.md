---
feature: Context Streaming
status: proposed
tool: observe
mode: real-time, push-notifications
version: 0.7.12
doc_type: product-spec
feature_id: feature-context-streaming
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Context Streaming (#5)

## Problem Statement

Current Kaboom workflow is "polling-based": AI calls `observe()`, waits for response, analyzes stale data.

### Limitations:
- **Latency:** AI can't react to errors as they happen; must wait for next observation cycle
- **Token Waste:** Each observation fetches full state; 80% redundant data between calls
- **Missed Context:** Transient errors (brief network spike, fleeting DOM state) lost between polls
- **No Event Ordering:** Hard to establish causal chains ("X changed → Y broke → Z surfaced")
- **Streaming Incompleteness:** LLM sees "error occurred" but misses the causal sequence

**Result:** AI diagnosis slow (multiple observe cycles) and incomplete (missed transient state).

## Solution

Context Streaming enables **real-time push notifications** instead of polling:

1. **Server-Pushed Events:** Kaboom server pushes errors, network failures, log anomalies to MCP as they happen
2. **Event Subscriptions:** AI subscribes to specific event types (errors, perf warnings, API failures, selector mismatches)
3. **Causal Ordering:** Events include timestamps, correlation IDs for ordering
4. **Efficiency:** Only new events sent; no redundant state polling
5. **Interactivity:** AI can react in real-time to events without waiting for next cycle

### Benefits:
- **3-5x Faster Diagnosis:** AI sees events as they happen, not after polling
- **90% Less Noise:** Only relevant events streamed; historical data muted
- **Complete Context:** Transient errors captured; causal chains preserved
- **Token Efficiency:** Event summary << full buffer snapshot

## Requirements

### Core Event Types
- **Error Events:** Console errors, assertion failures, uncaught exceptions
- **Network Events:** Failed requests, slow responses, API timeouts
- **Performance Events:** Layout thrashing, long tasks, paint delays
- **Selector Events:** Broken selectors, stale elements, accessibility violations
- **Anomaly Events:** Unusual patterns (repeated 404s, memory spikes, infinite loops)

### Subscription Model
- **Event Channels:** Subscribe to channels: `errors`, `network_errors`, `performance`, `anomalies`
- **Filtering:** Include/exclude patterns (e.g., "only errors from /api/cart")
- **Throttling:** Rate-limit notifications to prevent token overflow (default: 1/sec per channel)
- **Correlation IDs:** Link related events (network call → log → error)

### Event Format
- **Timestamp:** ISO 8601 with millisecond precision
- **Event Type:** error, network_error, performance, selector_broken, anomaly
- **Severity:** critical, high, medium, low, info
- **Summary:** 1-2 sentence description
- **Context:** Minimal details (error message snippet, status code, URL)
- **Correlation ID:** UUID for linking related events

### Integration
- **observe({what: "events"})** — Fetch recent streamed events (if polling fallback needed)
- **configure({what: "streaming", events: ["errors", "network_errors"], throttle_ms: 1000})** — Subscribe to streams
- **Bidirectional:** MCP client can mute channels dynamically based on noise

## Out of Scope

- **Video Replay:** Visual debugging with video; use screenshots instead
- **Real-Time Collaboration:** Multiple AI clients on same session (Phase 2)
- **Historical Analysis:** Long-term event storage; focus on live diagnostics
- **Custom Event Types:** Domain-specific events deferred; focus on universal types

## Success Criteria

- ✅ **Real-Time Delivery:** Events pushed <100ms from occurrence
- ✅ **Completeness:** 100% of actionable events captured (errors, failures, anomalies)
- ✅ **Efficiency:** Average event size <500 bytes; no redundant data
- ✅ **Ordering:** Causal chains preserved; correlation IDs correct
- ✅ **Responsiveness:** AI diagnoses failures 3-5x faster than polling
- ✅ **Subscription Control:** AI can mute/unmute channels dynamically
- ✅ **Reliability:** No lost events; connection recovery automatic

## User Workflow

### Scenario 1: Real-Time Error Detection

#### Before (Polling):
```
1. Error occurs: POST /api/cart failed
2. AI waits (default poll interval 5-10 seconds)
3. AI calls observe({what: 'logs'}) → fetches 1000 lines
4. AI parses to find error
5. AI diagnoses (total: 10-15 seconds latency)
```

#### After (Streaming):
```
1. Error occurs: POST /api/cart failed
2. Kaboom pushes error event immediately (<100ms)
3. AI receives: { type: 'network_error', status: 500, url: '/api/cart', timestamp: '...' }
4. AI diagnoses immediately (total: <500ms latency)
```

### Scenario 2: Subscription Management

#### Subscribe to Specific Channels:
```javascript
await kaboom.configure({
  action: 'streaming',
  events: ['errors', 'network_errors'],  // Only errors
  exclude_patterns: ['/analytics', '/metrics'],  // Ignore noise
  throttle_ms: 500  // Max 2 events/second
});

// AI now receives real-time stream of errors + network failures
// Ignores analytics/metrics noise
```

#### React to Event:
```javascript
// Kaboom pushes: { type: 'network_error', status: 401, url: '/api/user' }
// AI immediately diagnoses: "Auth token expired or missing"
// AI proposes: "Refresh session" OR "Reload with new credentials"
```

### Scenario 3: Causal Chains

**Scenario:** API call fails → no response → UI state not updated → test fails

#### Without Streaming (Polling):
1. Test fails with "Expected 'Success', got 'Loading'"
2. AI calls observe({what: 'logs'}) → sees log entries from last 5 seconds
3. AI searches for errors → finds POST /api/save failed
4. AI diagnoses: "API call failed; state didn't update"

#### With Streaming (Real-Time):
1. API call made: network event {type: 'network', url: '/api/save', correlationId: 'xyz'}
2. API fails: network_error event {type: 'network_error', status: 500, correlationId: 'xyz'}
3. Error logged: error event {type: 'error', msg: 'Failed to save', correlationId: 'xyz'}
4. DOM not updated: selector event {type: 'selector_broken', selector: '[data-status="success"]', correlationId: 'xyz'}
5. Test fails: error event {type: 'assertion_failed', expected: 'Success', got: 'Loading', correlationId: 'xyz'}
6. AI receives 5 linked events instantly; causal chain clear without parsing logs

## Integration Points

- **Self-Healing Tests (#1):** Streams error events for real-time diagnosis
- **Kaboom CI Infrastructure (#2):** Streams events during CI test runs
- **MCP Server:** WebSocket or polling-driven delivery over existing `/mcp` bridge
- **LLM:** Receives event stream; reacts to failures in real-time

## Dependencies

- ✅ **Kaboom Core:** observe(), configure() tools
- ✅ **WebSocket Support:** For real-time event delivery
- ✅ **Buffer Architecture:** Existing buffers + streaming layer
- ⏳ **Self-Healing Tests (#1):** Consumes streaming events for diagnosis
- ⏳ **CI Infrastructure (#2):** Uses streaming for test diagnostics

## Technical Approach

### Server-Side (Go)
- **Event Emission:** Log events as they occur (errors, network calls, anomalies)
- **Subscription Manager:** Track active subscriptions, apply filters
- **WebSocket Handler:** Maintain client connection, push events
- **Correlation Tracking:** Link related events via IDs

### Client-Side (MCP)
- **Event Handler:** Receive real-time events from server
- **Stream Subscription:** Configure channels, filters, throttling
- **Fallback:** If streaming unavailable, fall back to polling observe()

### Event Correlation
- **Network Calls:** Assign unique ID on POST, include in response
- **Log Entries:** Include correlation ID in log context
- **Errors:** Include correlation ID from triggering event

## Phase Breakdown

### Phase 1 (Week 1-2): Core Streaming
- WebSocket + polling infrastructure
- Event emission for core types (errors, network failures)
- Subscription API
- Client-side stream handler

### Phase 2 (Week 2-3): Rich Events
- Event correlation IDs
- Performance anomaly detection
- Selector mismatch detection
- Throttling + filtering

### Phase 3 (Week 3-4): Integration
- Self-Healing Tests integration
- CI Infrastructure integration
- Fallback logic (polling if streaming fails)
- UI feedback (event counts, stream status)

## Success Metrics

- **Latency:** <100ms from event occurrence to AI notification
- **Completeness:** 100% of actionable events captured
- **Efficiency:** Average event <500 bytes; 10-to-100 times less data than polling
- **Diagnosis Speed:** 3-5x faster failure diagnosis vs polling
- **Reliability:** 99.9% delivery rate; no lost events

---

## Related Features

- [Self-Healing Tests (#33)](../self-healing-tests/product-spec.md) — Primary consumer of event stream
- [Kaboom CI Infrastructure](../ci-infrastructure/product-spec.md) — Uses streaming for CI diagnostics
- [Agentic E2E Repair (#34)](../agentic-e2e-repair/product-spec.md) — Streams API contract changes

---

## v6.0 Wave 1 Priority

**Tier:** Core enabler for Self-Healing Tests and CI Infrastructure
**Effort:** 4-6 weeks (parallel with #33, #2)
**Blocks:** Faster, more efficient autonomous repair loops
