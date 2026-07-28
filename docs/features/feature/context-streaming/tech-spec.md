---
feature: context-streaming
status: proposed
doc_type: tech-spec
feature_id: feature-context-streaming
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Context Streaming

> Plain language only. No code. Describes HOW context streaming works at a high level.

## Architecture Overview

Context Streaming transforms Kaboom from a pull-based system (AI polls for data) to a push-based system (server pushes events as they occur). This reduces latency and enables real-time AI responses to browser events.

### Current model (pull-based):
```
Event occurs → Captured → Stored in ring buffer → AI polls observe() → Retrieves data (2-10s latency)
```

### New model (push-based):
```
Event occurs → Captured → Server pushes notification to AI → AI receives immediately (< 100ms latency)
```

The push mechanism uses MCP notifications (JSON-RPC 2.0 `notifications/message` method). The AI client subscribes to event types it wants (errors, network failures, performance degradations) and receives notifications as they occur.

## Key Components

### 1. Streaming Configuration (configure tool, action: streaming)

**Purpose:** Enable/disable streaming, subscribe to event types, set filters.

#### Inputs:
- action: "streaming"
- enabled (boolean) — Turn streaming on/off
- subscribe (array of event types) — Which events to stream
- filters (object) — Criteria for what to stream (severity, URLs, patterns)

#### Event types:
- `error` — Console errors and exceptions
- `network_failure` — 4xx/5xx HTTP responses
- `network_slow` — Requests exceeding performance budget
- `websocket_close` — WebSocket disconnections
- `performance_degradation` — Core Web Vitals violations
- `security_violation` — CSP violations, mixed content warnings
- `accessibility_violation` — a11y audit failures (if running)
- `all` — All event types (use with caution, high volume)

#### Filters:
- severity (enum: critical, high, medium, low) — Minimum severity for errors
- url_pattern (regex) — Only stream events matching URL pattern
- exclude_pattern (regex) — Exclude events matching pattern (noise filtering)
- rate_limit (integer) — Max events per second (prevent flood)

#### Example request:
```json
{
  "tool": "configure",
  "arguments": {
    "action": "streaming",
    "enabled": true,
    "subscribe": ["error", "network_failure"],
    "filters": {
      "severity": "high",
      "url_pattern": "^https://api\\.example\\.com",
      "rate_limit": 5
    }
  }
}
```

### 2. Notification Transport (MCP Protocol)

**Protocol:** MCP notifications use JSON-RPC 2.0 `notifications/message` method.

**Direction:** Server → Client (one-way, no response expected)

#### Format:
```json
{
  "jsonrpc": "2.0",
  "method": "notifications/message",
  "params": {
    "level": "error",
    "logger": "kaboom",
    "data": {
      "event_type": "error",
      "timestamp": "2026-01-28T10:00:00Z",
      "message": "TypeError: Cannot read property 'x' of null",
      "url": "https://example.com",
      "severity": "high",
      "stack_trace": "...",
      "tabId": 101
    }
  }
}
```

#### MCP notifications:
- Sent over stdio (same channel as requests/responses)
- No response required (fire-and-forget)
- Order not guaranteed (TCP guarantees delivery but not processing order)
- Client must handle out-of-order notifications

### 3. Event Dispatcher (Server-Side)

**Purpose:** Monitor ring buffers, detect new events, send notifications to subscribed clients.

#### Architecture:
- Background goroutine runs on server startup (when streaming enabled)
- Watches ring buffer writes (triggered on POST /logs, /network-bodies, etc.)
- For each new entry, checks if it matches any client subscriptions
- If match, serializes event and sends MCP notification to client
- Applies filters (severity, URL pattern, rate limits) before sending
- Maintains per-client subscription state (what they subscribed to, filters)

#### Concurrency:
- Uses sync.RWMutex for subscription state
- Event dispatch is async (doesn't block ring buffer writes)
- If notification send fails (client disconnected), unsubscribe automatically

#### Rate limiting:
- Tracks events per second per client
- If rate limit exceeded, queue events or drop (configurable)
- Sends "rate_limit_exceeded" notification when throttling occurs

### 4. Event Matching Engine

**Purpose:** Determine if an event matches a client's subscription and filters.

#### Matching logic:
1. Check event_type against client's subscribe array
2. If subscribed, check severity threshold (error severity >= client min severity)
3. If severity passes, check URL pattern (regex match against event.url)
4. If URL passes, check exclude pattern (inverse match)
5. If all pass, event is eligible for streaming
6. Check rate limit (events sent to this client in last second)
7. If under rate limit, dispatch notification; else queue or drop

#### Performance:
- Event matching is O(N) where N = number of subscribed clients (typically 1 in single-user mode)
- Regex evaluation cached (compiled patterns stored per client)
- Target < 1ms per event matching

### 5. Client State Management

**Purpose:** Track which clients are subscribed, their filters, and connection state.

#### State stored per client:
- client_id (derived from stdio connection or session token)
- subscriptions (array of event types)
- filters (severity, URL patterns, rate limit)
- last_notification_sent (timestamp for rate limiting)
- event_count_this_second (counter for rate limiting)

#### Lifecycle:
- Client subscribes: configure({what: "streaming", enabled: true}) → Server creates subscription state
- Client unsubscribes: configure({what: "streaming", enabled: false}) → Server removes subscription state
- Client disconnects: Server detects broken pipe → removes subscription state
- Server restart: All subscription state lost (clients must re-subscribe)

## Data Flows

### Subscription Flow
```
AI client → MCP request: configure({what: "streaming", enabled: true, subscribe: ["error"]})
→ Server receives request
→ Server creates subscription state for this client
→ Server registers event dispatcher (if not already running)
→ Server responds: {"configured": true, "streaming_enabled": true}
→ Event dispatcher starts monitoring ring buffers
```

### Event Streaming Flow (Error Example)
```
Page → console.error("fail") → Extension captures → POST /logs to server
→ Server stores in logs ring buffer → triggers event dispatcher
→ Dispatcher checks: new log entry, type: error, severity: high
→ Dispatcher finds client subscribed to "error" events
→ Dispatcher checks filters: severity: high >= client min (high), URL matches pattern
→ Dispatcher checks rate limit: 3 events sent this second, limit is 5 (OK)
→ Dispatcher serializes notification: {method: "notifications/message", params: {...}}
→ Server writes notification to stdio → AI client receives
→ AI client processes notification in < 100ms (vs 2-10s polling delay)
```

### Rate Limiting Flow
```
10 errors occur in 1 second → All match client subscription
→ Dispatcher sends first 5 (rate limit: 5/sec)
→ Dispatcher queues remaining 5 or drops (configurable)
→ Dispatcher sends "rate_limit_exceeded" notification: {throttled: 5 events}
→ After 1 second, counter resets → Queued events sent
```

### Unsubscribe Flow
```
AI client → MCP request: configure({what: "streaming", enabled: false})
→ Server removes subscription state for this client
→ Event dispatcher checks: no active subscriptions → dispatcher pauses or exits
→ Server responds: {"configured": true, "streaming_enabled": false}
→ No more notifications sent to this client
```

## Implementation Strategy

### Phase 1: Subscription Management
1. Add "streaming" action to configure tool schema
2. Implement subscription state storage (in-memory map, keyed by client ID)
3. Implement subscribe/unsubscribe handlers (create, update, delete state)
4. Add session tracking (identify which stdio connection is which client)
5. Handle client disconnect (cleanup subscription state)

### Phase 2: Event Dispatcher
1. Create background goroutine for event dispatching
2. Watch ring buffer writes (hook into POST handlers for /logs, /network-bodies, etc.)
3. For each new entry, trigger matching engine
4. Serialize matched events to MCP notification format
5. Write notifications to stdio (client's output stream)

### Phase 3: Event Matching Engine
1. Implement event type matching (check if client subscribed)
2. Implement severity filtering (compare severity levels)
3. Implement URL pattern matching (regex evaluation)
4. Implement exclude pattern matching (inverse regex)
5. Cache compiled regex patterns per client

### Phase 4: Rate Limiting
1. Track events sent per client per second (sliding window counter)
2. Implement rate limit checks before sending notification
3. Implement queuing or dropping for rate-limited events (configurable)
4. Send "rate_limit_exceeded" meta-notification when throttling

### Phase 5: Notification Transport
1. Implement MCP notification serialization (JSON-RPC 2.0 format)
2. Write notifications to stdio alongside responses (non-blocking)
3. Handle write failures (broken pipe → unsubscribe)
4. Add notification counters for observability (metrics)

### Phase 6: Integration with Existing Features
1. Hook dispatcher into existing ring buffer writes
2. Ensure streaming doesn't break existing observe tool (backward compatibility)
3. Test streaming with all event types (errors, network, WebSocket, performance)
4. Document AI client implementation (how to receive notifications)

## Edge Cases & Assumptions

### Edge Case 1: Client Disconnects Mid-Stream
**Handling:** Server detects broken pipe when attempting to write notification. Removes subscription state, stops sending notifications to that client. No crash or resource leak.

### Edge Case 2: Event Storm (100+ Events Per Second)
**Handling:** Rate limiting kicks in. Server throttles to configured limit (default 5/sec), queues or drops excess. Sends "rate_limit_exceeded" notification to client. Prevents flooding stdio.

### Edge Case 3: Notification Write Blocks
**Handling:** Use non-blocking write (buffered channel). If buffer full, drop notification and log warning. Don't block ring buffer writes (streaming is best-effort, not guaranteed delivery).

### Edge Case 4: Multiple Clients Subscribe
**Handling:** Each client has separate subscription state. Events matched independently for each client. One client's rate limit doesn't affect another.

### Edge Case 5: Notification Sent Before Client Ready
**Handling:** Client must be ready to receive notifications immediately after subscribe. If client not ready, notifications may be lost (MCP notifications are fire-and-forget, no retry).

### Edge Case 6: Client Subscribes to "all" (High Volume)
**Handling:** Allow but warn in response. Apply aggressive rate limiting (default 10/sec). Log warning that "all" can overwhelm client. Document recommended subscriptions (specific event types).

### Assumption 1: Single Client in Local Development
We assume one AI agent connected at a time (typical for local development). Multi-client support exists but is less common.

### Assumption 2: stdio Reliable
We assume stdio is the reliable transport. TCP-based stdio connections guarantee delivery order (within limits).

### Assumption 3: Client Can Handle Async Notifications
We assume AI clients are built to handle unsolicited notifications. Some MCP clients may need updates to support notifications.

## Risks & Mitigations

### Risk 1: Notification Flood Overwhelms Client
**Mitigation:** Rate limiting (default 5/sec). Client can set lower rate_limit. Use filters to reduce noise (severity, URL patterns). Recommend subscribing to specific event types, not "all".

### Risk 2: Event Matching Adds Latency to Capture
**Mitigation:** Dispatcher runs async (doesn't block POST handlers). Matching is fast (< 1ms). If dispatcher lags, events queue in memory (bounded queue, drop oldest if full).

### Risk 3: stdio Buffer Full (Notifications Dropped)
**Mitigation:** Use buffered channel for notifications (default 100 events). If buffer full, drop oldest notification, log warning, send "buffer_full" meta-notification to client.

### Risk 4: Client Doesn't Process Notifications (Backpressure)
**Mitigation:** Server doesn't wait for client to process (fire-and-forget). If client falls behind, stdio buffer fills, OS blocks writes. Server detects slow writes, pauses streaming, sends "streaming_paused" notification.

### Risk 5: Regex Denial of Service (Complex Patterns)
**Mitigation:** Limit regex complexity (max 100 characters, no exponential backtracking patterns). Validate patterns on subscribe. If evaluation takes > 10ms, reject pattern with error.

## Dependencies

### Depends On (Existing Features)
- **Ring buffers** — Dispatcher watches ring buffer writes
- **POST endpoints** — /logs, /network-bodies, etc. trigger event dispatch
- **configure tool** — Add streaming action to existing tool
- **MCP protocol** — Use standard MCP notifications mechanism

### No New Capture
- Streaming uses existing captured data (no new capture mechanisms)

## Performance Considerations

- Event matching: < 1ms per event
- Notification serialization: < 1ms per notification
- stdio write: < 1ms (non-blocking)
- Total latency (event → notification): < 100ms
- Memory per client subscription: < 1KB (state storage)
- Dispatcher overhead: < 1% CPU (idle when no events)
- Rate limiting overhead: < 0.1ms per event (counter increment)

## Security Considerations

- **No new attack surface:** Streaming uses existing MCP protocol over stdio (localhost-only)
- **No data leak:** Notifications contain same data as observe responses (already redacted)
- **DoS via subscription flood:** Limit subscriptions per client (max 10 event types). Reject new subscriptions if limit exceeded.
- **Regex injection:** Validate patterns on subscribe. Reject unsafe patterns (exponential backtracking, excessive length).
- **Resource exhaustion:** Rate limiting prevents event storms. Bounded queues prevent memory exhaustion.

## Test Plan Reference

See qa-plan.md for detailed testing strategy. Key test scenarios:
1. Subscribe to errors → console.error occurs → notification received < 100ms
2. Subscribe to network failures → 500 response → notification received
3. Rate limiting: 10 events/sec, limit 5 → first 5 sent, rest throttled
4. Filters: severity "high" → only high/critical errors streamed
5. URL pattern filter: "^https://api" → only API events streamed
6. Unsubscribe → no more notifications received
7. Client disconnect → subscription cleaned up, no resource leak
8. Backward compatibility: observe tool still works when streaming enabled
