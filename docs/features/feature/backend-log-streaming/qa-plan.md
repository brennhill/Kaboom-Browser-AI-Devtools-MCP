---
status: proposed
scope: feature/backend-log-streaming
ai-priority: high
tags: [v7, backend-integration, testing]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-backend-log-streaming
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Backend Log Streaming — QA Plan

## Test Scenarios

### Scenario 1: Single Service Streaming Logs at Normal Rate
#### Setup:
- Start one backend service (api-server) connected to Kaboom
- Service configured to log at INFO level
- ~100 logs per second

#### Steps:
1. Start Kaboom MCP server
2. Connect backend service via gRPC
3. Backend emits logs for 30 seconds
4. Call `observe({what: 'backend-logs'})`

#### Expected Result:
- All logs appear in Kaboom within 100ms of emission
- Logs are searchable by timestamp, service name, level
- No logs are lost or duplicated

#### Acceptance Criteria:
- [ ] Logs appear in real-time stream
- [ ] Timestamp ordering is preserved
- [ ] All 3000 logs are captured (100 logs/sec × 30s)

---

### Scenario 2: High-Volume Log Ingestion (Burst)
#### Setup:
- Backend service sends 10K logs/sec for 5 seconds
- Mix of INFO, WARN, ERROR levels

#### Steps:
1. Connected backend service
2. Trigger bulk operation causing high logging (e.g., batch import)
3. Observe memory usage and latency
4. Query backend-logs for that 5-second window

#### Expected Result:
- All 50K logs are buffered without dropping
- Ingestion latency stays <1ms per log
- Memory usage doesn't exceed 50MB
- No CPU spike >80%

#### Acceptance Criteria:
- [ ] 0% log loss under 10K/sec load
- [ ] Ingestion latency remains <1ms
- [ ] Memory footprint <50MB for 50K logs
- [ ] Query for those logs completes in <100ms

---

### Scenario 3: Connection Drop & Reconnection
#### Setup:
- Backend service connected and streaming logs
- Network connection simulated to drop

#### Steps:
1. Backend streaming logs normally
2. Kill network connection (simulated)
3. Observe error handling in Kaboom
4. Manually reconnect backend
5. Verify queued logs are replayed

#### Expected Result:
- Kaboom detects connection loss
- Backend queues logs locally (in-memory)
- On reconnect, queued logs are replayed in order
- No logs are lost

#### Acceptance Criteria:
- [ ] Connection failure detected within 15s (heartbeat)
- [ ] Backend queues logs during disconnection
- [ ] Reconnection replays all queued logs
- [ ] Logs appear with correct original timestamps

---

### Scenario 4: Correlation ID Extraction & Tracing
#### Setup:
- Frontend user action triggers a request
- Request ID: `req-12345` set in X-Request-ID header
- Backend service logs with this request ID

#### Steps:
1. Trigger a user action in the browser (e.g., "Submit Form")
2. Frontend XHR includes `X-Request-ID: req-12345`
3. Backend receives request, logs with `request_id: req-12345`
4. Query Kaboom for logs with `request_id: req-12345`

#### Expected Result:
- Backend logs are correlated with frontend action
- Timeline shows unified view of request lifecycle
- All backend logs for that request are grouped together

#### Acceptance Criteria:
- [ ] Backend logs have request_id field populated
- [ ] Query `observe({what: 'backend-logs', request_id: 'req-12345'})` returns all related logs
- [ ] Request lifecycle is traceable from frontend to backend and back

---

### Scenario 5: Multiple Services Streaming
#### Setup:
- 3 backend services: api-server, worker, cache
- Each emits logs at different rates
- Services have overlapping request processing

#### Steps:
1. Start all 3 services connected to Kaboom
2. User action triggers requests across all services
3. Query logs by service name
4. Query logs for specific request ID across all services

#### Expected Result:
- Logs from all services are captured
- Filtering by service returns only that service's logs
- Filtering by request_id returns logs from all services involved

#### Acceptance Criteria:
- [ ] Each service's logs are tagged correctly
- [ ] Service-based filtering works accurately
- [ ] Request tracing across services shows complete flow

---

### Scenario 6: Memory Eviction Under Load
#### Setup:
- Kaboom configured with 500MB max memory
- Long-running backend service continuously logging
- 5000 logs/sec for 1 hour

#### Steps:
1. Run backend for 1 hour at 5K logs/sec
2. Monitor memory usage in Kaboom
3. Query for oldest logs (should be evicted)
4. Query for recent logs (should still exist)

#### Expected Result:
- Memory stays within 500MB limit
- Oldest logs are evicted when capacity reached
- Recent logs remain available (within 1-hour TTL)
- Eviction doesn't cause visible pauses

#### Acceptance Criteria:
- [ ] Memory never exceeds 500MB
- [ ] LRU eviction removes oldest logs first
- [ ] Recent logs remain queryable
- [ ] No latency spikes during eviction

---

### Scenario 7: Error Log Capture & Stack Traces
#### Setup:
- Backend service encounters an exception
- Logs error with full stack trace

#### Steps:
1. Trigger error condition (e.g., division by zero)
2. Backend logs ERROR level with stack trace
3. Query Kaboom for ERROR logs
4. Verify stack trace is preserved

#### Expected Result:
- Error logs are captured with full stack trace
- Stack trace is readable in Kaboom UI
- Error context (user_id, operation, etc.) is included

#### Acceptance Criteria:
- [ ] ERROR logs are captured with stack traces
- [ ] Stack traces are not truncated
- [ ] Additional fields (user_id, etc.) are preserved
- [ ] Query filters by level work correctly

---

### Scenario 8: Graceful Shutdown
#### Setup:
- Backend service connected and actively logging
- Kaboom is shut down

#### Steps:
1. Backend service running, streaming logs
2. Initiate graceful shutdown of Kaboom
3. Verify backend doesn't lose logs
4. Restart Kaboom, verify recent logs persist (if applicable)

#### Expected Result:
- Graceful shutdown gives 30s drain time
- All in-flight logs are written
- Backend receives ACK for all logs
- Backend doesn't retry already-acked logs

#### Acceptance Criteria:
- [ ] Graceful shutdown takes ~30s
- [ ] All logs are acknowledged before shutdown
- [ ] Backend doesn't see ERRORs during shutdown
- [ ] No partial/corrupted logs

---

### Scenario 9: Large Field Values & Special Characters
#### Setup:
- Backend logs with very large values (10MB JSON blob, binary data)
- Special characters in message (unicode, newlines, quotes)

#### Steps:
1. Log entry with large fields (10MB)
2. Log entry with special characters
3. Query and verify fields are preserved
4. Check for truncation or encoding issues

#### Expected Result:
- Large fields are handled without truncation (or gracefully truncated with warning)
- Special characters are preserved
- Logs remain queryable

#### Acceptance Criteria:
- [ ] Fields up to 1MB are preserved in full
- [ ] Larger fields are gracefully truncated with marker
- [ ] No encoding corruption
- [ ] Queryable text is extracted correctly

---

## Acceptance Criteria (Overall)
- [ ] All 9 scenarios pass
- [ ] Performance meets requirements (<1ms ingestion, <50ms query)
- [ ] No memory leaks detected over 1-hour test run
- [ ] Backend reconnection doesn't lose logs
- [ ] Correlation ID tracing works end-to-end
- [ ] Error logs are captured with full context

## Test Data

### Fixture: Simple Log Entry
```json
{
  "timestamp": "2026-01-31T10:15:23.456Z",
  "level": "INFO",
  "service": "api-server",
  "message": "User login successful",
  "request_id": "req-12345",
  "trace_parent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
  "span_id": "00f067aa0ba902b7",
  "fields": {
    "user_id": 987,
    "duration_ms": 45,
    "ip_address": "192.0.2.1"
  }
}
```

### Fixture: Error Log with Stack Trace
```json
{
  "timestamp": "2026-01-31T10:15:30.123Z",
  "level": "ERROR",
  "service": "worker",
  "message": "Failed to process job",
  "request_id": "req-12346",
  "span_id": "span-999",
  "fields": {
    "job_id": "job-555",
    "retry_count": 3,
    "error_code": "TIMEOUT"
  },
  "stack_trace": "panic: timeout after 30s\n  at process.go:42\n  at main.go:10"
}
```

### Load Test Data
- Generate 50K logs with realistic variance in fields
- Use timestamps over a 1-hour window
- Mix of services: api-server (40%), worker (35%), cache (25%)
- Log levels: INFO (60%), WARN (25%), ERROR (15%)

## Regression Tests

### Core Functionality
- [ ] Logs stream in real-time without buffering delays
- [ ] Correlation IDs are extracted from all formats
- [ ] Multiple services don't interfere with each other
- [ ] Query filters return correct subsets

### Resilience
- [ ] Connection loss is detected and handled
- [ ] Reconnection replays queued logs
- [ ] Memory limits prevent unbounded growth
- [ ] Graceful shutdown preserves logs

### Data Integrity
- [ ] No duplicate logs appear
- [ ] Log order is preserved
- [ ] Field values are not corrupted
- [ ] Stack traces are complete

### Performance
- [ ] Ingestion stays <1ms under normal load
- [ ] Queries complete <50ms
- [ ] Memory footprint <500MB
- [ ] CPU usage <20% during normal streaming
