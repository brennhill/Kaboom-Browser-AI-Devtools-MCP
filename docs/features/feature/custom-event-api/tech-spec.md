---
status: proposed
scope: feature/custom-event-api
ai-priority: high
tags: [v7, backend-integration, events]
relates-to: [product-spec.md, ../backend-log-streaming/tech-spec.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-custom-event-api
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Custom Event API — Technical Specification

## Architecture

### Data Flow
```
Backend Service (gRPC)  Frontend (MCP)
    ↓                       ↓
Event Collector (server/events.go)
    ↓
Event Indexer (server/index/events.go)
    ↓
Kaboom Event Store
    ↓ on observe('custom-events')
MCP Handler
    ↓
Frontend Extension / Tests
```

### Components
1. **Event Collector** (`server/events.go`)
   - gRPC endpoint `EmitEvent(StreamEventRequest) returns (StreamEventAck)`
   - HTTP endpoint `POST /events` for convenience
   - Validates event type, service name, payload size
   - Assigns timestamp, incremental event ID
   - Forwards to indexer

2. **Event Indexer** (`server/index/events.go`)
   - Maintains in-memory event store (circular buffer)
   - Indexes by:
     - Event type (with wildcard support)
     - Service name
     - Trace ID (for correlation)
     - Timestamp (for time-based queries)
   - Supports prefix matching: `payment:*` matches `payment:authorized`, `payment:failed`
   - LRU eviction when capacity exceeded

3. **Event Query Handler** (`server/handlers.go`)
   - MCP tool: `observe({what: 'custom-events', ...})`
   - Filtering by type, service, trace_id, timestamp
   - Returns events with pagination support

### Type System
```go
type CustomEvent struct {
    ID         int64
    Timestamp  time.Time
    Type       string                 // Must match [a-z0-9:_]+ pattern
    Service    string                 // Must match [a-z0-9\-]+ pattern
    TraceID    string                 // Optional: for correlation
    Hostname   string                 // Optional: source host
    Fields     map[string]interface{} // Validated below
    Size       int                    // Cached size for quota tracking
}

// Validation rules:
// - Type: max 256 chars, matches [a-z0-9:_]+
// - Service: max 128 chars, matches [a-z0-9\-]+
// - TraceID: max 128 chars (W3C trace format)
// - Fields: max 100 keys, max 10KB serialized JSON
// - Any field value: max 1MB (logged as violation)
```

## Implementation Plan

### Phase 1: Core API & Storage (Week 1)
1. Define protobuf for `CustomEvent` and validation
2. Implement in-memory circular buffer for events
3. Implement gRPC streaming endpoint
4. Add HTTP REST endpoint for convenience (non-streaming)
5. Basic validation: type format, payload size

### Phase 2: Indexing & Querying (Week 2)
1. Implement event indexer with multiple indexes
2. Implement wildcard type matching (prefix trees)
3. Implement MCP handler for `observe({what: 'custom-events', ...})`
4. Add filtering: by type, service, trace_id, timestamp
5. Implement pagination for large result sets

### Phase 3: Frontend Integration (Week 3)
1. Add MCP tool for frontend to emit events
2. Add automatic trace context propagation
3. Test browser-to-backend event correlation
4. Document SDK for both Go and Node.js backends

### Phase 4: Testing & Performance (Week 4)
1. Load test: 10K events/sec
2. Test wildcard matching performance
3. Test correlation with other event types
4. Memory profiling and optimization

## API Changes

### gRPC Service (proto)
```protobuf
service CustomEventService {
  rpc StreamEvents(stream EventRequest) returns (stream EventAck);
}

message EventRequest {
  string type = 1;                    // e.g., "payment:authorized"
  string service = 2;                 // e.g., "checkout-service"
  string trace_id = 3;                // Optional: W3C trace or request_id
  string hostname = 4;                // Optional
  map<string, google.protobuf.Value> fields = 5; // Event data
}

message EventAck {
  int64 event_id = 1;
  bool success = 2;
  string error = 3;
}
```

### REST Endpoint
```
POST /events
Content-Type: application/json

{
  "type": "payment:authorized",
  "service": "checkout-service",
  "trace_id": "abc123",
  "fields": {
    "amount": 99.99,
    "currency": "USD"
  }
}

Response (202 Accepted):
{
  "event_id": 12345,
  "timestamp": "2026-01-31T10:15:23.456Z"
}
```

### MCP Tool Handler
```go
// In handlers.go
func handleCustomEvents(req *CustomEventsRequest) (*CustomEventsResponse, error) {
    // Supports filters:
    // - type: "payment:*", "payment:authorized", etc. (wildcard support)
    // - service: "checkout-service"
    // - trace_id: "abc123" (for correlation)
    // - since: timestamp
    // - limit: max results (default 100, max 1000)

    // Returns matching events with pagination cursor
}
```

## Code References
- **Event collector:** `/Users/brenn/dev/kaboom/server/events.go` (new)
- **Event indexer:** `/Users/brenn/dev/kaboom/server/index/events.go` (new)
- **Proto definitions:** `/Users/brenn/dev/kaboom/proto/events.proto` (new)
- **MCP handler:** `/Users/brenn/dev/kaboom/server/handlers.go` (modified)
- **Tests:** `/Users/brenn/dev/kaboom/server/events_test.go` (new)

## Performance Requirements
- **Emission latency:** <1ms per event (async, no database)
- **Query latency:** <50ms for 1000 events with filtering
- **Memory:** 500B per event, max 500MB total (circular buffer)
- **Throughput:** 10K events/sec per service, 50K total across all services
- **Wildcard matching:** <1ms for type prefix matching

## Testing Strategy

### Unit Tests
- Validate event type and service name format
- Test field validation (size, count, type constraints)
- Test wildcard type matching (prefix tree)
- Test circular buffer eviction
- Test indexing by type, service, trace_id

### Integration Tests
- Test gRPC streaming with concurrent clients
- Test HTTP REST endpoint
- Test event ingestion under load (10K/sec)
- Test wildcard queries performance
- Test correlation with backend logs via trace_id

### E2E Tests
- Real backend service emitting events
- Frontend emitting events via MCP
- Verify events appear in Kaboom timeline
- Verify correlation with other observables

## Dependencies
- **Protobuf runtime:** (Go stdlib)
- **gRPC:** (already in Kaboom)
- **Backend SDKs:** Go and Node.js packages for event emission (separate repos)
- **Trace context:** Must propagate W3C trace format or custom trace_id

## Risks & Mitigation
1. **Type explosion** (unbounded event types)
   - Mitigation: Document standard event types, enforce format validation
2. **Memory overflow from high-volume events**
   - Mitigation: Circular buffer with aggressive TTL (1 hour), configurable cap
3. **Wildcard query performance** (e.g., `*` matching all events)
   - Mitigation: Limit wildcard expansion, enforce prefix matching minimum
4. **Field validation bypass** (binary data, deeply nested objects)
   - Mitigation: Serialize to JSON, enforce size limits, log violations

## Backward Compatibility
- REST endpoint has no version, extensible via JSON
- gRPC uses protobuf `Any` for forward compatibility
- New field types automatically supported via JSON deserialization
