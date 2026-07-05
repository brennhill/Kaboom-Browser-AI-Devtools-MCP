---
status: proposed
scope: feature/backend-log-streaming
ai-priority: high
tags: [v7, backend-integration, logging, ears]
relates-to: [../custom-event-api.md, ../../core/architecture.md]
last-verified: 2026-03-05
doc_type: product-spec
feature_id: feature-backend-log-streaming
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Backend Log Streaming

## Overview
Backend Log Streaming enables real-time capture of server-side logs through a bidirectional gRPC or WebSocket connection. This feature bridges the gap between frontend telemetry and backend operations, allowing developers to correlate frontend user actions with backend processing, errors, and state changes. By streaming logs directly from the backend service into Kaboom's observation pipeline, developers gain unified visibility across their entire stack.

## Problem
Currently, developers using Kaboom can observe frontend behavior in granular detail—every user click, network request, and DOM change. However, backend logs remain isolated in separate systems (CloudWatch, Datadog, ELK stacks). This creates a blind spot:
- When a user's action fails, developers must manually jump between frontend tools and backend logs
- Backend errors are invisible until they cause user-visible failures
- Debugging multi-service failures requires stitching together multiple log systems
- Performance issues that originate on the backend are impossible to correlate with frontend impact

## Solution
Backend Log Streaming establishes a persistent connection from a Kaboom-compatible backend service to the MCP server. The backend sends structured log entries (with severity, timestamp, service metadata, and request IDs) in real-time. These logs are:
1. **Ingested** into Kaboom's central event stream
2. **Indexed** with correlation IDs (request tracing, session IDs, user IDs)
3. **Exposed** through the MCP API for real-time observation and historical search
4. **Correlated** with frontend events by shared tracing context

## User Stories
- As a full-stack developer, I want to see backend logs stream in real-time alongside frontend events so that I can immediately understand why a network request failed
- As a DevOps engineer, I want to capture backend service startup logs, health checks, and lifecycle events so that I can audit service deployments
- As a QA engineer, I want to verify that error logs appear in Kaboom when I trigger specific error conditions so that I can validate error handling
- As a platform engineer, I want to rate-limit backend log ingestion to prevent flooding so that the observation system remains performant

## Acceptance Criteria
- [ ] Backend can establish persistent connection to Kaboom MCP server via gRPC or WebSocket
- [ ] Backend sends structured log entries with timestamp, level, message, service name, request ID, span ID
- [ ] Logs are deduplicated and buffered to handle burst traffic (max 10K logs/sec per service)
- [ ] Logs expire from memory after 1 hour or when memory threshold is exceeded
- [ ] Correlation IDs (X-Request-ID, trace parent) are extracted and indexed
- [ ] MCP tool `observe({what: 'backend-logs'})` returns streamed logs with filtering
- [ ] Performance: backend log ingestion adds <1ms to log line processing
- [ ] Logs survive service disconnection (queued locally, replayed on reconnect)

## Not In Scope
- Processing or transforming backend logs (we ingest as-is)
- Long-term persistence (logs are ephemeral, for dev/test only)
- Centralized log aggregation across multiple machines
- Authentication/authorization for backend connections (assumes trusted network)
- Multi-tenancy (Kaboom is single-user development tool)

## Data Structures

### Backend Log Entry
```go
type BackendLogEntry struct {
    Timestamp     time.Time
    Level         string                 // "DEBUG", "INFO", "WARN", "ERROR"
    Service       string                 // "api-server", "worker", "cache"
    Message       string
    RequestID     string                 // Correlation with frontend
    TraceParent   string                 // W3C Trace Context
    SpanID        string
    Fields        map[string]interface{} // Key-value pairs (duration_ms, user_id, etc.)
    StackTrace    string                 // Optional, for errors
}
```

### MCP Observable Response
```json
{
  "type": "backend-logs",
  "logs": [
    {
      "timestamp": "2026-01-31T10:15:23.456Z",
      "service": "api-server",
      "level": "ERROR",
      "message": "Failed to process payment",
      "requestId": "req-12345",
      "fields": {
        "userId": 987,
        "paymentId": "pay-99",
        "error": "insufficient_funds"
      }
    }
  ],
  "totalCount": 1250,
  "oldestTimestamp": "2026-01-31T09:15:23.456Z"
}
```

## Examples

### Example 1: User Clicks "Submit Payment" Button
#### Frontend:
```
[10:15:23.100] User Action: Click button.pay
[10:15:23.120] XHR POST /api/payments/process with X-Request-ID: req-12345
[10:15:23.450] XHR Response: 500 (error)
```

#### Backend (now visible in Kaboom):
```
[10:15:23.200] INFO api-server: POST /api/payments/process (req-12345)
[10:15:23.300] DEBUG api-server: User 987 processing payment
[10:15:23.350] ERROR api-server: Payment gateway timeout (req-12345, span-789)
[10:15:23.400] ERROR api-server: Returning 500 to client
```

**Developer's view:** Unified timeline shows exactly where the failure occurred in the backend.

### Example 2: Service Restart
Backend logs appear immediately:
```
[10:30:00.050] INFO api-server: Graceful shutdown initiated
[10:30:00.150] INFO api-server: Closed 247 active connections
[10:30:01.200] INFO api-server: Service started (version 2.5.0)
[10:30:01.250] INFO api-server: Database connected, 50 replicas healthy
```

## MCP Tool Changes
New mode for `observe()`:
```javascript
observe({
  what: 'backend-logs',
  service: 'api-server',          // optional: filter by service
  level: 'ERROR',                 // optional: filter by severity
  limit: 100,
  since: '2026-01-31T10:00:00Z'   // optional: time filter
})
```
