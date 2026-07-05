---
status: proposed
scope: feature/custom-event-api
ai-priority: high
tags: [v7, backend-integration, events, ears]
relates-to: [../backend-log-streaming.md, ../../core/architecture.md]
last-verified: 2026-03-05
doc_type: product-spec
feature_id: feature-custom-event-api
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Custom Event API

## Overview
Custom Event API allows backend services and frontend JavaScript code to emit arbitrary events into Kaboom's observation timeline. These events are first-class citizens in Kaboom's event system—not just logs or network traffic, but semantic business events that carry structured data. A payment processor can emit `payment:authorized`, an API can emit `rate-limit:triggered`, a test framework can emit `test:started`, or a feature flag service can emit `feature:toggled`. Developers query these events alongside browser events, backend logs, and network requests to understand their system's complete behavior.

## Problem
Kaboom captures logs and network traffic automatically, but doesn't capture application-specific semantics:
- When does a critical business operation complete? (Payment processing, order fulfillment, user signup)
- When do guard rails trigger? (Rate limiting, feature flags, circuit breakers)
- When does a test actually start vs. when the test framework is loaded?
- Which specific code path executed? (A/B test variant, feature flag state)

Without semantic events, developers must infer behavior from logs ("I see INFO: Creating order" vs. "I see ERROR: Order creation failed") or from state changes. This is lossy and slow.

## Solution
Custom Event API provides a simple HTTP/gRPC endpoint where services emit typed events with payloads. Events are:
1. **Ingested** with automatic timestamp, source attribution
2. **Indexed** by event type, service, and custom fields
3. **Exposed** through MCP observation API with filtering and search
4. **Correlated** with other events (frontend, backend logs, network) via trace context

Example: Backend emits `event:type=payment:authorized, amount=99.99, currency=USD, user_id=123, trace_id=abc`. Frontend simultaneously emits `event:type=checkout:success`. Kaboom aligns them on the timeline.

## User Stories
- As a backend engineer, I want to emit semantic events (payments authorized, orders shipped) so that tests and debugging can react to business outcomes
- As a test automation engineer, I want to emit test lifecycle events from my test runner so that Kaboom can correlate test execution with observed behavior
- As a frontend developer, I want to emit custom analytics events so that I can trace user workflows across the application
- As a platform engineer, I want to see rate limiting and circuit breaker activations as events so that I can identify bottlenecks
- As a QA engineer, I want to query events by type and properties so that I can verify feature flags and experiments applied correctly

## Acceptance Criteria
- [ ] HTTP/gRPC endpoint accepts `CustomEvent` with type, service, fields, trace_id
- [ ] Events are persisted in memory with 1-hour TTL
- [ ] Events are queryable via MCP: `observe({what: 'custom-events', type: 'payment:*'})`
- [ ] Wildcard filtering supported: `type: 'payment:*'` matches `payment:authorized`, `payment:failed`, etc.
- [ ] Events can be emitted from backend (via SDK) and frontend (via MCP tool)
- [ ] Max payload: 10KB per event, max 100 fields per event
- [ ] Performance: event emission <1ms, query <50ms for 1000 events
- [ ] Events can be correlated with logs/requests via trace_id or request_id

## Not In Scope
- Long-term persistence (ephemeral for dev/test only)
- Event transformation or routing
- Authentication for event emission (assumes trusted network)
- Event replay or audit trail
- Complex filtering operators (only simple equality and wildcards)

## Data Structures

### Custom Event
```go
type CustomEvent struct {
    Timestamp  time.Time
    Type       string                 // "payment:authorized", "test:started"
    Service    string                 // "checkout-service", "test-runner"
    TraceID    string                 // W3C trace context or request_id
    Hostname   string                 // Source host (optional)
    Fields     map[string]interface{} // Event data (max 100 fields, 10KB total)
}
```

### Common Event Types (Examples)
```
# Business events
payment:authorized
payment:failed
order:created
order:shipped
user:signup
user:login

# Infrastructure events
rate-limit:exceeded
circuit-breaker:opened
cache:invalidated
database:connection_lost

# Test events
test:started
test:completed
test:failed
test:assertion
```

## Examples

### Example 1: Payment Authorization
#### Backend (Go):
```go
kaboom.EmitEvent(ctx, "payment:authorized", map[string]interface{}{
    "payment_id": "pay-999",
    "amount": 99.99,
    "currency": "USD",
    "user_id": 123,
    "processor": "stripe",
    "latency_ms": 245,
})
```

#### Kaboom View:
```
[10:15:23.100] Frontend: Click "Complete Purchase"
[10:15:23.120] Frontend: XHR POST /api/payments
[10:15:23.200] Backend: Backend Log: Processing payment
[10:15:23.350] Custom Event: payment:authorized (amount: 99.99)
[10:15:23.400] Frontend: XHR Response: 200 OK
[10:15:23.420] Frontend: Page rendered: Order Confirmation
```

### Example 2: Test Lifecycle
#### Test Runner (Node.js):
```javascript
kaboom.emit({
    type: "test:started",
    fields: { test_name: "auth.spec.js", suite: "authentication" }
});

// ... test runs ...

kaboom.emit({
    type: "test:completed",
    fields: {
        test_name: "auth.spec.js",
        passed: true,
        duration_ms: 2340
    }
});
```

#### Kaboom Timeline:
```
[10:30:00.100] Custom Event: test:started (test_name: auth.spec.js)
[10:30:00.150] Frontend: Page load
[10:30:01.200] Frontend: User input
[10:30:02.300] Backend: Auth check
[10:30:02.400] Custom Event: test:completed (passed: true)
```

### Example 3: Feature Flag Evaluation
#### Frontend (JavaScript):
```javascript
const variant = await featureFlags.get("checkout_redesign");
kaboom.emit({
    type: "feature:toggled",
    fields: {
        flag: "checkout_redesign",
        variant: variant,
        user_id: userId
    }
});
```

Developers query: `observe({what: 'custom-events', type: 'feature:*'})` to see all feature flag decisions.

## MCP Tool Changes
New modes for `observe()`:
```javascript
observe({
    what: 'custom-events',
    type: 'payment:*',           // Wildcard matching
    service: 'checkout-service', // Filter by emitting service
    field: 'user_id:123',        // Simple field matching
    limit: 100,
    since: '2026-01-31T10:00:00Z'
})
```

New tool for emission:
```javascript
// Frontend can emit events via MCP
generateCustomEvent({
    type: 'user:action',
    fields: { action: 'button_click', button_id: 'checkout' }
})
```
