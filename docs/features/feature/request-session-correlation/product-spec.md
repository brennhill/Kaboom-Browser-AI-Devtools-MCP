---
status: proposed
scope: feature/request-session-correlation
ai-priority: high
tags: [v7, correlation, observability, eyes]
relates-to: [../backend-log-streaming.md, ../../core/architecture.md]
last-verified: 2026-03-05
doc_type: product-spec
feature_id: feature-request-session-correlation
last_reviewed: 2026-08-04
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# Request/Session Correlation

## Overview
Request/Session Correlation provides a unified view of all activity related to a single user session or request, connecting frontend interactions with backend processing. When a user's browser session generates multiple requests across multiple backend services, Kaboom automatically groups them together using session IDs and request IDs. A developer can ask "Show me everything that happened for user 123 in the last 5 minutes" and get a complete timeline: every page load, every API request, every backend log, every custom event, all correlated and ordered. This eliminates the need to manually stitch together logs from multiple systems.

## Problem
Modern applications span multiple services, but request tracing is fragmented:
- Frontend makes request A to service X, which calls service Y and service Z
- Each service logs independently without understanding the broader context
- Debugging requires manually finding the right logs in each system
- "How many requests did this user make?" requires querying multiple systems and deduplicating manually
- Timeouts and cascading failures are hard to diagnose without seeing the full request graph

## Solution
Request/Session Correlation:
1. **Session ID Management:** Each browser session gets a unique ID, included in all requests
2. **Request ID Propagation:** Each API request gets a unique ID, propagated through all backend services
3. **Automatic Correlation:** Kaboom indexes all events (logs, network, custom events) by session ID and request ID
4. **Unified Query:** Query all activity for a session: `observe({what: 'session-trace', session_id: 'session-abc'})`

## User Stories
- As a backend engineer, I want to see all services involved in processing a request so that I can understand cascading failures
- As a support engineer, I want to show a customer exactly what happened during their transaction so that I can provide detailed explanations
- As a platform engineer, I want to correlate request IDs across services so that I can calculate end-to-end latency
- As a DevOps engineer, I want to identify which user session triggered a service degradation event

## Acceptance Criteria
- [ ] Frontend automatically generates and stores session ID
- [ ] Session ID included in all frontend requests as header
- [ ] Backend services receive and propagate request IDs and session IDs
- [ ] All backend logs include session_id and request_id fields
- [ ] All custom events include session_id (optional) and request_id
- [ ] Query API: `observe({what: 'session-trace', session_id: 'xyz'})` returns all activity
- [ ] Query API: `observe({what: 'request-trace', request_id: 'abc'})` returns request lifecycle
- [ ] Performance: session query <100ms for 10K events
- [ ] Session ID persists across browser restarts (localStorage or cookie)

## Not In Scope
- Cross-origin session tracking (single origin only)
- User identity correlation (sessions are anonymous)
- Session timeout/expiration policy
- Compliance with specific data residency laws

## Data Structures

### Session ID
```go
type SessionID string  // Format: "session-" + hex(16 random bytes) = "session-a7f8e3d9c1b2e4f6"

// Generated on first page load, stored in localStorage
// Included in all subsequent requests via header: X-Session-ID
```

### Request ID
```go
type RequestID string  // Format: "req-" + hex(16 random bytes) = "req-12345a7f8e3d9c1b2e4f6"

// Generated for each XHR, fetch, or backend API request
// Propagated through W3C Trace Context (traceparent header)
```

### Session Trace
```json
{
  "session_id": "session-a7f8e3d9c1b2e4f6",
  "user_agent": "Mozilla/5.0...",
  "ip_address": "192.0.2.1",
  "started_at": "2026-01-31T10:00:00Z",
  "last_activity": "2026-01-31T10:15:30Z",
  "total_requests": 47,
  "services_touched": ["api-server", "auth-service", "payment-service"],
  "events": [
    {
      "timestamp": "2026-01-31T10:00:05.123Z",
      "type": "page:load",
      "url": "http://localhost/dashboard"
    },
    {
      "timestamp": "2026-01-31T10:00:10.456Z",
      "type": "network:request",
      "request_id": "req-123",
      "method": "GET",
      "url": "/api/user/profile"
    }
  ]
}
```

## Examples

### Example 1: User Checkout Flow
#### Frontend:
```
Session: session-xyz
[10:00:00] Page load: /checkout
  → Request ID: req-001
  → GET /api/cart (200 OK, 45ms)

[10:00:30] User input: Amount field
[10:00:45] User click: "Pay Now" button
  → Request ID: req-002
  → POST /api/payments (202 Accepted, 250ms)
    → Backend calls payment processor
    → Calls inventory service to reserve items
    → Calls email service to send confirmation
```

#### Backend logs (all tagged with session: session-xyz, request: req-002):
```
api-server: INFO - Processing payment (request_id: req-002, session_id: session-xyz)
payment-service: INFO - Authorizing with Stripe (request_id: req-002)
inventory-service: INFO - Reserving items (request_id: req-002)
email-service: INFO - Sending confirmation (request_id: req-002)
```

#### Kaboom query:
```javascript
observe({
  what: 'session-trace',
  session_id: 'session-xyz',
  since: '2026-01-31T10:00:00Z'
})
```

**Result:** Complete timeline showing user journey through all systems.

### Example 2: Support Debugging
Support engineer needs to explain transaction failure to customer:
```javascript
observe({
  what: 'request-trace',
  request_id: 'req-002'
})
```

Returns:
```
[10:00:45.000] Frontend: User clicked "Pay Now"
[10:00:45.050] Network: POST /api/payments
[10:00:45.100] Backend api-server: Processing payment
[10:00:45.150] Backend payment-service: Calling Stripe
[10:00:45.200] Backend payment-service: ERROR - Insufficient funds
[10:00:45.250] Backend api-server: Returning 402 Payment Required
[10:00:45.300] Frontend: XHR failed with 402
[10:00:45.350] Frontend: Displayed error message
```

Support says: "Your payment was declined because of insufficient funds. You can try another payment method."

## MCP Changes
```javascript
// Query session activity
observe({
  what: 'session-trace',
  session_id: 'session-xyz',
  since: timestamp,
  include: ['network', 'logs', 'events']
})

// Query request lifecycle
observe({
  what: 'request-trace',
  request_id: 'req-123'
})

// Search sessions by criteria
observe({
  what: 'sessions',
  service: 'payment-service',  // Sessions touching this service
  status: 'error',              // Sessions with errors
  since: timestamp,
  limit: 100
})
```

## Frontend Implementation
```javascript
// Session management
const sessionId = localStorage.getItem('kaboom:session-id')
  || generateSessionID();
localStorage.setItem('kaboom:session-id', sessionId);

// Include in all requests
fetch('/api/endpoint', {
  headers: {
    'X-Session-ID': sessionId,
    'X-Request-ID': generateRequestID()
  }
})
```

## Backend Implementation
```go
// Middleware to extract and propagate IDs
func correlationMiddleware(next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    sessionID := r.Header.Get("X-Session-ID")
    requestID := r.Header.Get("X-Request-ID")

    // Log with IDs
    logger.WithFields(map[string]interface{}{
      "session_id": sessionID,
      "request_id": requestID,
    }).Info("Processing request")

    // Propagate to downstream services
    next.ServeHTTP(w, r)
  })
}
```
