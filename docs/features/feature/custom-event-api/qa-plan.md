---
status: proposed
scope: feature/custom-event-api
ai-priority: high
tags: [v7, testing]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-custom-event-api
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Custom Event API — QA Plan

## Test Scenarios

### Scenario 1: Simple Event Emission & Query
#### Setup:
- Kaboom MCP server running
- Simple backend service ready to emit events

#### Steps:
1. Emit event: `{type: "user:login", service: "auth", fields: {user_id: 123}}`
2. Query: `observe({what: 'custom-events', type: 'user:login'})`
3. Verify event appears in results

#### Expected Result:
- Event is persisted and queryable
- Fields are preserved exactly as sent
- Timestamp is accurate

#### Acceptance Criteria:
- [ ] Event appears in query results within 100ms
- [ ] All fields are present and correct
- [ ] Timestamp is within 1 second of query time

---

### Scenario 2: Wildcard Type Filtering
#### Setup:
- Multiple events with hierarchical types:
  - `payment:authorized`
  - `payment:failed`
  - `payment:refunded`
  - `order:created`
  - `order:shipped`

#### Steps:
1. Emit all 5 events
2. Query: `observe({what: 'custom-events', type: 'payment:*'})`
3. Verify 3 payment events returned, 2 order events excluded

#### Expected Result:
- Wildcard prefix matching works correctly
- Only matching events returned
- Order preserved (most recent first)

#### Acceptance Criteria:
- [ ] `payment:*` returns exactly 3 events
- [ ] `order:*` returns exactly 2 events
- [ ] `*` or no type filter returns all 5
- [ ] Query completes in <50ms

---

### Scenario 3: Event Correlation with Trace ID
#### Setup:
- Backend and frontend emitting correlated events
- Same trace_id across events

#### Steps:
1. Frontend emits: `{type: "checkout:started", trace_id: "trace-999", fields: {...}}`
2. Backend emits: `{type: "payment:processing", trace_id: "trace-999", fields: {...}}`
3. Backend emits: `{type: "payment:authorized", trace_id: "trace-999", fields: {...}}`
4. Query: `observe({what: 'custom-events', trace_id: 'trace-999'})`

#### Expected Result:
- All 3 events returned in timeline order
- Events are linked via trace_id
- Complete flow visible in chronological order

#### Acceptance Criteria:
- [ ] Query by trace_id returns all 3 events
- [ ] Events are ordered by timestamp
- [ ] Correlation is unambiguous

---

### Scenario 4: Large Payload Events
#### Setup:
- Event with many fields (50-100 fields)
- Large field values (up to 1KB each)
- Total payload near 10KB limit

#### Steps:
1. Emit event with 80 fields, ~9KB total
2. Query for that event
3. Verify all fields are preserved
4. Emit event exceeding 10KB (should be rejected or truncated)

#### Expected Result:
- Events up to 10KB are fully preserved
- Oversized events are rejected with clear error
- Field count limit (100) is enforced

#### Acceptance Criteria:
- [ ] 10KB payload accepted and queryable
- [ ] All 80 fields preserved
- [ ] >10KB rejected with "payload too large"
- [ ] >100 fields rejected with "too many fields"

---

### Scenario 5: High-Volume Event Ingestion
#### Setup:
- Backend emitting 5K events/sec for 10 seconds
- All events must be stored and queryable

#### Steps:
1. Start high-volume emission (5K/sec × 10s = 50K events)
2. Monitor memory and CPU
3. Query for events during burst
4. Verify no loss

#### Expected Result:
- All 50K events stored
- No memory overflow
- Queries still responsive
- Ingestion latency stays <1ms

#### Acceptance Criteria:
- [ ] 0% event loss at 5K/sec
- [ ] Ingestion latency <1ms per event
- [ ] Memory <100MB for 50K events
- [ ] Query latency <100ms during burst

---

### Scenario 6: Memory Eviction (LRU)
#### Setup:
- Kaboom configured with 200MB max memory
- 1-hour TTL for events
- Backend emitting continuously at 2K/sec for 2 hours

#### Steps:
1. Run emission for 2 hours at 2K/sec
2. Monitor memory usage
3. Query for oldest events (should be evicted)
4. Query for recent events (should exist)

#### Expected Result:
- Memory stays within 200MB limit
- Old events are evicted (LRU)
- Recent events remain available
- No pauses during eviction

#### Acceptance Criteria:
- [ ] Memory never exceeds 200MB
- [ ] LRU eviction removes oldest first
- [ ] Events >1 hour old are not queryable
- [ ] Recent events remain fully accessible

---

### Scenario 7: Service Filtering
#### Setup:
- Multiple services emitting events:
  - `checkout-service` (payment events)
  - `auth-service` (login events)
  - `inventory-service` (stock events)

#### Steps:
1. Each service emits 10 events
2. Query: `observe({what: 'custom-events', service: 'checkout-service'})`
3. Verify only checkout events returned

#### Expected Result:
- Service filtering isolates events correctly
- Other services' events excluded
- All 10 checkout events returned

#### Acceptance Criteria:
- [ ] Service filter returns only matching events
- [ ] Different services don't interfere
- [ ] Filtering works with wildcard type matching

---

### Scenario 8: Timestamp Range Query
#### Setup:
- Events emitted over 1-minute window
- Events at specific timestamps: 0s, 15s, 30s, 45s, 60s

#### Steps:
1. Emit 5 events at 15-second intervals
2. Query: `observe({what: 'custom-events', since: '30s', until: '60s'})`
3. Verify only events at 30s, 45s, 60s returned

#### Expected Result:
- Timestamp-based filtering works correctly
- Boundary events included
- Old events excluded

#### Acceptance Criteria:
- [ ] `since` parameter filters correctly
- [ ] `until` parameter filters correctly
- [ ] Boundary events included
- [ ] Query completes in <50ms

---

### Scenario 9: REST Endpoint vs. gRPC
#### Setup:
- Both REST and gRPC endpoints enabled
- Emit events via both

#### Steps:
1. Emit event via REST: `POST /events` with JSON
2. Emit event via gRPC: `EmitEvent()` call
3. Query both events
4. Verify they're identical in storage

#### Expected Result:
- Both endpoints work identically
- Events are indistinguishable in storage
- Query returns both

#### Acceptance Criteria:
- [ ] REST endpoint accepts valid JSON
- [ ] gRPC endpoint works with streaming
- [ ] Both produce identical stored events
- [ ] Concurrent use doesn't cause conflicts

---

### Scenario 10: Field Type Validation
#### Setup:
- Events with various field types:
  - String, number, boolean, null
  - Arrays, nested objects
  - Special characters, unicode

#### Steps:
1. Emit event with mixed field types
2. Query event
3. Verify types are preserved

#### Expected Result:
- All JSON types are supported
- No type corruption
- Special characters preserved

#### Acceptance Criteria:
- [ ] String fields: preserved exactly
- [ ] Number fields: no precision loss
- [ ] Boolean, null: preserved
- [ ] Nested objects: preserved
- [ ] Unicode: no encoding issues

---

## Acceptance Criteria (Overall)
- [ ] All 10 scenarios pass
- [ ] Emission latency <1ms
- [ ] Query latency <50ms for 1000 events
- [ ] Memory eviction works under sustained load
- [ ] Wildcard type matching performs well
- [ ] Trace correlation is accurate
- [ ] REST and gRPC endpoints equivalent

## Test Data

### Fixture: Payment Event
```json
{
  "type": "payment:authorized",
  "service": "checkout-service",
  "trace_id": "trace-12345",
  "fields": {
    "payment_id": "pay-999",
    "amount": 99.99,
    "currency": "USD",
    "user_id": 456,
    "processor": "stripe",
    "latency_ms": 245
  }
}
```

### Fixture: Test Event
```json
{
  "type": "test:assertion",
  "service": "jest-runner",
  "fields": {
    "test_name": "should calculate total",
    "assertion": "expect(total).toBe(100)",
    "passed": true,
    "duration_ms": 2
  }
}
```

### Load Test Data
- Generate 50K events with realistic distribution
- Mix of types: `payment:*` (40%), `order:*` (30%), `test:*` (20%), `system:*` (10%)
- 3-4 services emitting
- Fields: 5-20 per event
- Timestamps: 1-hour window

## Regression Tests

### Correctness
- [ ] All events stored are queryable
- [ ] No event loss under any load
- [ ] Wildcard matching is accurate (no false positives/negatives)
- [ ] Timestamp filtering is exclusive/inclusive as documented

### Performance
- [ ] Emission latency remains <1ms at scale
- [ ] Query latency <50ms even with 10K events
- [ ] Memory stays within configured limit
- [ ] No garbage collection pauses >100ms

### Integration
- [ ] Events correlate with backend logs via trace_id
- [ ] Events correlate with network requests via trace_id
- [ ] Frontend and backend events appear in unified timeline
- [ ] Event order is consistent with timestamps

### Resilience
- [ ] Oversized payloads are rejected safely
- [ ] Invalid JSON in REST rejected with 400
- [ ] Concurrent emission from multiple services works
- [ ] Graceful degradation if memory full (LRU eviction)
