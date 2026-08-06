---
title: "WebSocket Debugging"
description: "Debug WebSocket connections with KaBOOM. Monitor connection lifecycle, inspect messages by direction and connection, track message rates, and diagnose real-time application issues."
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['guides', 'websocket', 'debugging']
---

KaBOOM gives your AI full visibility into WebSocket traffic — connection lifecycle, message payloads, per-connection rates, and schema detection. This is critical for debugging real-time applications: chat systems, live dashboards, collaborative editors, trading platforms, and notification streams.

## See Active Connections

```js
observe({what: "websocket_status"})
```

Returns all tracked WebSocket connections with:

| Field | Description |
|-------|-------------|
| Connection ID | Unique identifier for filtering |
| URL | WebSocket endpoint |
| State | `connecting`, `open`, `closed`, `error` |
| Message rate | Messages per second (incoming/outgoing) |
| Total messages | Count since connection opened |
| Schema | Inferred message structure (if JSON) |

This is your starting point. See how many connections are open, whether any have errored, and which ones are active.

---

## Inspect Messages

```js
observe({what: "websocket_events"})
```

Returns captured WebSocket messages with:

| Field | Description |
|-------|-------------|
| Timestamp | When the message was sent/received |
| Connection ID | Which connection it belongs to |
| Direction | `incoming` (server → browser) or `outgoing` (browser → server) |
| Type | `message`, `open`, `close`, `error` |
| Data | Message payload (truncated at 4KB) |

### Filter by Connection

When you have multiple WebSocket connections (common in complex apps), filter to a specific one:

```js
observe({what: "websocket_events", connection_id: "ws-3"})
```

### Filter by Direction

See only what the server is sending:

```js
observe({what: "websocket_events", direction: "incoming"})
```

Or only what the client is sending:

```js
observe({what: "websocket_events", direction: "outgoing"})
```

### Limit Results

```js
observe({what: "websocket_events", last_n: 20})
```

Returns only the most recent 20 events.

---

## Common Debugging Scenarios

### Connection Drops

**Symptom**: Real-time features stop updating. Users report stale data.

**Diagnosis**:

```js
observe({what: "websocket_status"})
```

Check if the connection state is `closed` or `error`. Then look at the events:

```js
observe({what: "websocket_events", connection_id: "ws-1", last_n: 10})
```

The last events before the close reveal why — did the server send a close frame with a reason code? Did the connection error out? Was there a network interruption?

Common close codes:
- `1000` — Normal closure (intentional)
- `1001` — Going away (server shutting down, page navigating)
- `1006` — Abnormal closure (no close frame — connection dropped)
- `1008` — Policy violation
- `1011` — Server error
- `1013` — Try again later (server overloaded)

### Message Format Mismatches

**Symptom**: Client sends a message but nothing happens. No error, no response.

**Diagnosis**:

```js
observe({what: "websocket_events", direction: "outgoing", last_n: 5})
```

See exactly what the client is sending. Then check incoming messages:

```js
observe({what: "websocket_events", direction: "incoming", last_n: 5})
```

Compare the message formats. Common issues:
- Client sends JSON but server expects a different structure
- Missing required fields in the message payload
- Wrong message type or action identifier
- String where the server expects a number (or vice versa)

### Reconnection Loops

**Symptom**: The WebSocket repeatedly connects and disconnects. Network waterfall shows many short-lived connections.

**Diagnosis**:

```js
observe({what: "websocket_events", last_n: 50})
```

Look for a pattern: `open` → `close` → `open` → `close`. The close events will have reason codes. Common causes:
- Authentication failure on the WebSocket handshake
- Server rejecting the connection due to rate limiting
- Client-side reconnection logic with no backoff
- Incompatible protocol versions

### Missing Messages

**Symptom**: Some real-time updates arrive, others don't.

**Diagnosis**:

```js
observe({what: "websocket_status"})
```

Check the message rate. If the server is sending 100+ messages/second, KaBOOM's adaptive sampling may be active. Check if the missing messages are being filtered.

Then look at the actual message stream:

```js
observe({what: "websocket_events", direction: "incoming", last_n: 50})
```

Are all message types present? Is one subscription topic missing? The AI can check your subscription logic against what's actually arriving.

### Performance Issues

**Symptom**: The real-time UI is laggy or the page slows down when WebSocket traffic is high.

**Diagnosis**: Combine WebSocket data with performance metrics:

```js
observe({what: "websocket_status"})  // Check message rates
observe({what: "vitals"})             // Check INP and long tasks
observe({what: "timeline"})           // See WS events alongside long tasks
```

High message rates with high INP usually means the message handler is doing too much work on the main thread. The AI can find the handler and suggest batching, throttling, or moving processing to a Web Worker.

---

## Workflow: Full WebSocket Debug Session

### 1. Overview

```
"Show me all active WebSocket connections."
```

```js
observe({what: "websocket_status"})
```

### 2. Identify the Problem Connection

Look for connections in `error` or `closed` state, or connections with unusual message rates.

### 3. Inspect Events

```
"Show me the last 20 events on connection ws-3."
```

```js
observe({what: "websocket_events", connection_id: "ws-3", last_n: 20})
```

### 4. Correlate with Errors

```
"Are there any console errors related to WebSocket?"
```

```js
observe({what: "errors"})
```

### 5. Check the Timeline

```
"Show me the timeline — I want to see WebSocket events alongside network requests and errors."
```

```js
observe({what: "timeline", include: ["websocket", "errors", "network"]})
```

This reveals causation — did a failed API call trigger the WebSocket disconnect? Did a server restart close all connections?

### 6. Fix and Verify

After the AI fixes the issue:

```
"Clear the WebSocket buffer and let's watch the new connection."
```

```js
configure({what: "clear", buffer: "websocket"})
```

Then monitor the fresh connection to confirm the fix.

---

## Tips

**Use `websocket_status` first, `websocket_events` second.** Status gives you the overview — which connections exist, their state, and message rates. Events gives you the detail — individual messages and payloads.

**Filter aggressively on high-traffic connections.** A chat application might have thousands of messages. Use `connection_id`, `direction`, and `last_n` to focus on what matters.

**Correlate with the timeline.** WebSocket issues rarely happen in isolation. The timeline shows what else was happening — API failures, user actions, page navigations — when the connection dropped.

**Clear buffers between debug attempts.** After making a fix, `configure({what: "clear", buffer: "websocket"})` gives you a clean slate to verify the new behavior.
