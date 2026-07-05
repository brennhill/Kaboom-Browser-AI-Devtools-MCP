---
status: proposed
scope: feature/request-session-correlation
ai-priority: high
tags: [v7, testing]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-request-session-correlation
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Request/Session Correlation — QA Plan

## Test Scenarios

### Scenario 1: Session ID Generation & Persistence
#### Setup:
- Fresh browser session, Kaboom extension installed

#### Steps:
1. Load first page
2. Extension generates session ID
3. Verify stored in localStorage
4. Reload page
5. Verify same session ID persists
6. Close tab, open new tab on same site
7. Verify NEW session ID generated

#### Expected Result:
- Session ID generated on first load
- Same ID persists across reloads
- Fresh tab gets new session ID
- Format: "session-" + 32 hex characters

#### Acceptance Criteria:
- [ ] Session ID created and stored
- [ ] Persists across page reloads
- [ ] New tab = new session ID
- [ ] Format matches pattern

---

### Scenario 2: Session ID in Network Requests
#### Setup:
- Page loaded with session ID
- Multiple network requests made

#### Steps:
1. Load page, capture session ID
2. Make XHR request to backend
3. Verify X-Session-ID header present
4. Verify backend receives correct session ID
5. Make second request
6. Verify same session ID in both requests

#### Expected Result:
- All requests include X-Session-ID header
- Header value matches stored session ID
- Multiple requests share same session ID

#### Acceptance Criteria:
- [ ] X-Session-ID header present in all requests
- [ ] Header value is correct
- [ ] Consistency across multiple requests

---

### Scenario 3: Request ID Generation & Propagation
#### Setup:
- Session established with session ID
- Backend middleware configured to extract request ID

#### Steps:
1. Frontend makes GET /api/data
2. Verify traceparent header includes request ID
3. Backend receives request, extracts request ID
4. Backend logs with request ID
5. Frontend makes second request
6. Verify different request IDs for different requests

#### Expected Result:
- Each request gets unique request ID
- traceparent header includes request ID
- Backend successfully extracts ID
- Request IDs are different for different requests

#### Acceptance Criteria:
- [ ] traceparent header present
- [ ] traceparent format valid (W3C standard)
- [ ] Backend extracts request ID correctly
- [ ] Different requests have different IDs

---

### Scenario 4: Multi-Service Request Propagation
#### Setup:
- Frontend request spawns backend calls to multiple services
- All services configured to propagate request ID

#### Steps:
1. Frontend makes POST /api/checkout (req-001)
2. Backend api-server receives, logs with req-001
3. api-server calls payment-service, includes X-Request-ID: req-001
4. payment-service receives, logs with req-001
5. payment-service calls inventory-service, includes X-Request-ID: req-001
6. inventory-service logs with req-001
7. Query by request_id to get all logs

#### Expected Result:
- Request ID flows through entire call chain
- All services log with same request ID
- Query returns all 3 logs (api-server, payment, inventory)
- Timeline shows complete request flow

#### Acceptance Criteria:
- [ ] Request ID propagated through all services
- [ ] All logs tagged with same ID
- [ ] Query returns all related logs
- [ ] Timeline order is correct

---

### Scenario 5: Session Query - Complete Activity
#### Setup:
- User completes multi-step workflow:
  - Load dashboard
  - Click settings
  - Save preferences
  - Logout

#### Steps:
1. User navigates through workflow
2. Query: `observe({what: 'session-trace', session_id: 'xyz'})`
3. Verify all activity returned

#### Expected Result:
- Dashboard page load event
- Network requests for settings
- Form submission request
- Logout request
- All events ordered by timestamp
- Total count matches expected operations

#### Acceptance Criteria:
- [ ] All events included in session
- [ ] Order is chronological
- [ ] Event types accurate
- [ ] Request/response pairs matched

---

### Scenario 6: Request Query - Full Lifecycle
#### Setup:
- Single request that spawns multiple backend operations

#### Steps:
1. Frontend: POST /api/order (req-123)
2. Backend logs processing start
3. Backend calls inventory service
4. Backend calls payment service
5. Query: `observe({what: 'request-trace', request_id: 'req-123'})`

#### Expected Result:
- Request start event (XHR POST)
- All backend logs for req-123
- Related custom events
- Complete timeline with duration

#### Acceptance Criteria:
- [ ] Request details present
- [ ] All related logs included
- [ ] Duration calculated correctly
- [ ] Child operations identified

---

### Scenario 7: Error Tracking in Session
#### Setup:
- User session includes a failed operation
- One request fails while others succeed

#### Steps:
1. User makes successful requests
2. User makes failed request (e.g., payment decline)
3. User continues with new session (retry)
4. Query session including the failed request

#### Expected Result:
- Failed request identified with error status
- Error message captured
- Request still correlated with session
- Later requests in same session visible

#### Acceptance Criteria:
- [ ] Error request identified (status 4xx or 5xx)
- [ ] Error message included
- [ ] Session continues after error
- [ ] Can filter: "show errors in session"

---

### Scenario 8: Session Spanning Multiple Pages
#### Setup:
- User navigates from page A → page B → page C
- Session ID should persist

#### Steps:
1. Load page A (session generated)
2. Navigate to page B
3. Query session
4. Navigate to page C
5. Query session again

#### Expected Result:
- Same session ID on all pages
- All page load events in session
- All requests across pages grouped

#### Acceptance Criteria:
- [ ] Session ID persists across navigation
- [ ] All pages included in session trace
- [ ] Query returns activity from all pages

---

### Scenario 9: Concurrent Sessions
#### Setup:
- Two browser windows/tabs with different sessions
- Make requests from each simultaneously

#### Steps:
1. Tab 1: Session-A, make request req-001
2. Tab 2: Session-B, make request req-002
3. Queries should not interfere
4. Query Session-A should only show req-001
5. Query Session-B should only show req-002

#### Expected Result:
- Sessions properly isolated
- No event leakage between sessions
- Concurrent queries work correctly
- Each session has correct count of events

#### Acceptance Criteria:
- [ ] Sessions don't mix
- [ ] Query A shows only A's events
- [ ] Query B shows only B's events
- [ ] No race conditions

---

### Scenario 10: Session Timeout & Cleanup
#### Setup:
- Old session (>1 hour of inactivity)

#### Steps:
1. Verify old session still queryable
2. Enable aggressive TTL (for testing)
3. Wait for TTL to expire
4. Attempt to query old session
5. Verify graceful handling

#### Expected Result:
- Recent sessions queryable
- Old sessions may be evicted (configurable)
- No errors on old session query (just returns empty)
- Memory stays within limits

#### Acceptance Criteria:
- [ ] TTL configurable (default 1 hour)
- [ ] Old sessions evicted when TTL expires
- [ ] Memory doesn't grow unbounded
- [ ] Query on evicted session handled gracefully

---

## Acceptance Criteria (Overall)
- [ ] All 10 scenarios pass
- [ ] Session IDs persist across page reloads
- [ ] Request IDs propagate through service chain
- [ ] Session and request queries are accurate
- [ ] No event leakage between sessions/requests
- [ ] Performance: queries <100ms
- [ ] Memory management prevents unbounded growth

## Test Data

### Fixture: Multi-Service Request Flow
```
Session: session-abc123
[10:00:00] Page load dashboard
  Request: req-001, GET /api/dashboard → 200 (45ms)

[10:00:15] User click "checkout"
  Request: req-002, POST /api/checkout → 202 (500ms)
    │
    ├─ Backend api-server: INFO Processing checkout (req-002)
    │
    ├─ Backend payment-service: INFO Authorizing payment (req-002)
    │
    ├─ Backend inventory-service: INFO Reserving items (req-002)
    │
    └─ Backend notification-service: INFO Sending email (req-002)

[10:00:35] Page shows confirmation
  Request: req-003, GET /api/order/summary → 200 (30ms)
```

### Fixture: Error in Session
```
Session: session-xyz789
[10:15:00] Page load (req-100, 200 OK)
[10:15:30] User attempts payment (req-101, 200 OK) → succeeded
[10:15:45] User attempts payment (req-102, 402 Payment Required) → FAILED
[10:16:00] User tries again (req-103, 200 OK) → succeeded
```

## Regression Tests

### Correctness
- [ ] Session ID generated uniquely per session
- [ ] Request ID generated uniquely per request
- [ ] No cross-session event leakage
- [ ] Query results ordered by timestamp
- [ ] Count of events matches actual operations

### Performance
- [ ] Session query <100ms for 1000 events
- [ ] Request query <50ms
- [ ] Index updates <1ms per event
- [ ] Memory per session <20KB (metadata + index)

### Integration
- [ ] Backend logs include session_id and request_id
- [ ] Custom events include session_id and request_id
- [ ] Network events tagged with both IDs
- [ ] All event types correlated correctly

### Edge Cases
- [ ] Handles very long sessions (10K+ events)
- [ ] Handles very wide request tree (100+ child services)
- [ ] Handles circular request patterns gracefully
- [ ] Handles orphaned request IDs (no session)
