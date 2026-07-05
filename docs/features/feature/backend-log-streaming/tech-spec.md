---
status: proposed
scope: feature/backend-log-streaming
ai-priority: high
tags: [v7, backend-integration, logging]
relates-to: [product-spec.md, ../../core/architecture.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-backend-log-streaming
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Backend Log Streaming — Technical Specification

## Architecture

### Data Flow
```
Backend Service
    ↓ gRPC/WS stream
    ↓ (BackendLogEntry)
MCP Server (logs.go)
    ↓ index & buffer
Kaboom In-Memory Store
    ↓ on observe('backend-logs')
Frontend Extension
```

### Components
1. **Backend Log Receiver** (`server/logs.go`)
   - gRPC service with streaming endpoint
   - Accepts `StreamLogRequest` stream
   - Validates log entries, extracts correlation IDs
   - Buffers in circular buffer (1M entries, 1GB max)

2. **Log Indexer** (`server/index/logs.go`)
   - Maintains in-memory indexes:
     - By service name (map[string][]*LogEntry)
     - By request ID (map[string][]*LogEntry for tracing)
     - By timestamp (for time-based queries)
   - Updates on each incoming log
   - LRU eviction when memory exceeds threshold

3. **Log Query Handler** (`server/handlers.go`)
   - New MCP handler: `tools/backend-logs`
   - Supports filtering: service, level, request_id, timestamp range
   - Returns paginated results with cursor for streaming large result sets

### Connection Management
- Backend initiates gRPC connection on startup
- Exponential backoff reconnection (max 30s retry interval)
- Heartbeat every 15 seconds (detect dead connections)
- Graceful shutdown: drain 30s before closing (avoid log loss)

## Implementation Plan

### Phase 1: Infrastructure (Week 1)
1. Define protobuf for `BackendLogEntry` and `StreamLogRequest`
2. Implement gRPC server in `server/logs.go` with streaming handler
3. Add circular buffer data structure (memory-efficient)
4. Implement log entry validation and correlation ID extraction

### Phase 2: Indexing & Storage (Week 2)
1. Implement in-memory indexing (service, request_id, timestamp)
2. Add LRU eviction policy with configurable memory cap (default 500MB)
3. Implement log expiration (1 hour TTL by default)
4. Add telemetry: log ingestion rate, buffer saturation

### Phase 3: Query & Observation (Week 3)
1. Implement MCP handler for `observe({what: 'backend-logs', ...})`
2. Add filtering logic (by service, level, time range)
3. Implement cursor-based pagination for large result sets
4. Add MCP event subscription for real-time log streaming

### Phase 4: Testing & Reliability (Week 4)
1. Test with concurrent services sending logs
2. Test memory eviction under high load (10K logs/sec)
3. Test reconnection scenarios
4. Test correlation ID extraction from various formats

## API Changes

### New gRPC Service (proto)
```protobuf
service BackendLogService {
  rpc StreamLogs(stream LogEntry) returns (stream LogAck);
}

message LogEntry {
  int64 timestamp_unix_nanos = 1;
  string level = 2;               // "DEBUG", "INFO", "WARN", "ERROR"
  string service = 3;              // Service identifier
  string message = 4;
  string request_id = 5;
  string trace_parent = 6;          // W3C Trace Context
  string span_id = 7;
  map<string, string> fields = 8;   // Key-value fields
  string stack_trace = 9;
}

message LogAck {
  bool success = 1;
  string error = 2;
}
```

### New MCP Tool Handler
```go
// In handlers.go
func handleBackendLogs(req *BackendLogsRequest) (*BackendLogsResponse, error) {
    // Filtering:
    // - service: "api-server", "worker", etc.
    // - level: "ERROR", "WARN", etc.
    // - request_id: specific request tracing
    // - since: timestamp for range query
    // - limit: max results (default 100, max 1000)

    // Return logs matching criteria with pagination cursor
}
```

## Code References
- **gRPC handler:** `/Users/brenn/dev/kaboom/server/logs.go` (new)
- **Indexing:** `/Users/brenn/dev/kaboom/server/index/logs.go` (new)
- **Buffer:** `/Users/brenn/dev/kaboom/server/buffer/circular.go` (new)
- **Proto definitions:** `/Users/brenn/dev/kaboom/proto/logs.proto` (new)
- **Tests:** `/Users/brenn/dev/kaboom/server/logs_test.go` (new)

## Performance Requirements
- **Ingestion latency:** <1ms per log entry (before buffering)
- **Query latency:** <50ms for filtered query (1000 logs)
- **Memory footprint:** 1KB per log entry, max 500MB total
- **Throughput:** Support 10K logs/sec per service, 50K total
- **Connection overhead:** <100 goroutines per connected service

## Testing Strategy

### Unit Tests
- Test log entry parsing and validation
- Test correlation ID extraction (X-Request-ID, traceparent, custom)
- Test circular buffer eviction and memory limits
- Test index lookups (by service, request_id, time range)

### Integration Tests
- Test gRPC stream with multiple concurrent services
- Test log ingestion under load (10K logs/sec burst)
- Test reconnection scenarios (kill connection, verify replay)
- Test MCP handler filtering and pagination

### E2E Tests
- Real backend service sending logs
- Verify logs appear in Kaboom observation API
- Verify correlation with frontend events (same request_id)

## Dependencies
- **Protobuf runtime:** (already in Go stdlib)
- **gRPC:** (already used in Kaboom)
- **Backend SDK:** Backend service must include log streaming client (separate repo)
- **Correlation IDs:** Frontend must set X-Request-ID header (already implemented)

## Risks & Mitigation
1. **Network saturation from verbose logs**
   - Mitigation: Implement sampling, configurable log levels
2. **Memory overflow from long-running services**
   - Mitigation: Aggressive TTL (1h), LRU eviction, configurable cap
3. **Lost logs on connection drop**
   - Mitigation: Backend-side buffering, replay on reconnect
4. **Correlation ID extraction failures**
   - Mitigation: Support multiple ID formats, fallback to timestamp + service
