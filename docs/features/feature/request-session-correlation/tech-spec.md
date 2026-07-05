---
status: proposed
scope: feature/request-session-correlation
ai-priority: high
tags: [v7, correlation, observability]
relates-to: [product-spec.md, ../backend-log-streaming/tech-spec.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-request-session-correlation
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Request/Session Correlation — Technical Specification

## Architecture

### Data Flow
```
Frontend (page load)
    ↓ Generate session ID
Browser Storage (localStorage)
    ↓
Frontend XHR/Fetch
    ↓ Include session ID, request ID in headers
Backend Services
    ↓ Extract and log IDs
Backend Logs + Custom Events
    ↓ Indexed by session_id, request_id
Kaboom Event Store
    ↓ on observe('session-trace', session_id)
Unified Timeline
```

### Components
1. **Session ID Manager** (`extension/session-manager.js`)
   - Generate session ID on first page load
   - Store in localStorage with key `kaboom:session-id`
   - Persist across page reloads, cleared on browser close or manual logout
   - Include in all XHR/fetch requests

2. **Request ID Manager** (`extension/request-manager.js`)
   - Generate unique request ID for each network request
   - Propagate via W3C Trace Context (traceparent header)
   - Track request-to-response mapping

3. **Session Indexer** (`server/index/sessions.go`)
   - Maintain index of sessions by session_id
   - For each session: user_agent, ip_address, timestamps, touched services
   - Link all events (logs, network, custom) by session_id
   - Maintain index of requests by request_id

4. **Session Query Handler** (`server/handlers.go`)
   - MCP handler: `observe({what: 'session-trace', ...})`
   - Return all events for session in chronological order
   - Support filtering by event type, time range, service

## Implementation Plan

### Phase 1: Session ID Management (Week 1)
1. Implement session ID generation (browser extension)
2. Store in localStorage with TTL handling
3. Include in all network requests (via fetch/XHR interceptors)
4. Verify session ID persists across page reloads

### Phase 2: Request ID Management (Week 2)
1. Generate request ID for each network operation
2. Implement W3C Trace Context (traceparent header)
3. Track request IDs in extension state
4. Forward to MCP server for logging

### Phase 3: Server-Side Indexing (Week 3)
1. Extract session_id and request_id from incoming logs/events
2. Create session index (map[sessionID]*SessionRecord)
3. Create request index (map[requestID]*RequestRecord)
4. Maintain metadata: timestamps, services, error status

### Phase 4: Query & Visualization (Week 4)
1. Implement MCP handler for session-trace queries
2. Implement MCP handler for request-trace queries
3. Support filtering and pagination
4. Performance testing at scale

## API Changes

### Session ID Header
```
X-Session-ID: session-a7f8e3d9c1b2e4f6
```

### Request ID / Trace Context
```
traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
trace-state: kaboom-session-xyz
```

### Session Index Structure
```go
type SessionRecord struct {
    SessionID     string
    UserAgent     string
    IPAddress     string
    StartedAt     time.Time
    LastActivity  time.Time
    RequestIDs    []string         // Associated request IDs
    Services      map[string]bool  // Services touched
    EventCount    int
    HasErrors     bool
    FirstURL      string           // First page loaded
}

type RequestRecord struct {
    RequestID     string
    SessionID     string
    Method        string           // GET, POST, etc.
    URL           string
    Status        int
    StartedAt     time.Time
    CompletedAt   time.Time
    DurationMS    int64
    ChildRequests []string         // Requests spawned by this request
    ServiceCalls  []string         // Services called
}
```

### MCP Query Handlers
```go
// Session trace query
type SessionTraceRequest struct {
    SessionID   string
    Since       *time.Time
    Until       *time.Time
    EventTypes  []string  // Optional: filter by type
    Limit       int       // Max events
    Cursor      string    // For pagination
}

type SessionTraceResponse struct {
    SessionID   string
    Events      []ObservableEvent
    NextCursor  string
    Total       int
}

// Request trace query
type RequestTraceRequest struct {
    RequestID   string
}

type RequestTraceResponse struct {
    Request      RequestRecord
    RelatedLogs  []LogEntry
    RelatedEvents []CustomEvent
    Timeline     []Event
}
```

## Code References
- **Session manager:** `/Users/brenn/dev/kaboom/extension/session-manager.js` (new)
- **Request manager:** `/Users/brenn/dev/kaboom/extension/request-manager.js` (new)
- **Session indexer:** `/Users/brenn/dev/kaboom/server/index/sessions.go` (new)
- **MCP handlers:** `/Users/brenn/dev/kaboom/server/handlers.go` (modified)
- **Tests:** `/Users/brenn/dev/kaboom/server/index/sessions_test.go` (new)

## Performance Requirements
- **Session query:** <100ms for 10K events per session
- **Request query:** <50ms
- **Index maintenance:** <0.5ms per new event
- **Memory per session:** <10KB (metadata) + events
- **Concurrent sessions:** Support 10K+ active sessions

## Testing Strategy

### Unit Tests
- Session ID generation and persistence
- Request ID generation and propagation
- Session index lookup by ID
- Request index lookup and relationship tracking

### Integration Tests
- Multiple requests in single session
- Multiple sessions concurrently
- Cross-service request propagation
- Query latency with large event counts

### E2E Tests
- Real browser session with multiple page loads
- Real multi-service request flow
- Verify session/request correlation in timeline
- Performance testing at scale (10K sessions)

## Dependencies
- **Browser Storage API:** localStorage
- **W3C Trace Context:** traceparent header format
- **Backend SDKs:** Must extract and log session_id, request_id
- **HTTP Interception:** Extension must intercept all network requests

## Risks & Mitigation
1. **Session ID leakage across origins**
   - Mitigation: Only used for same-origin requests
2. **Session fixation attacks**
   - Mitigation: Generate fresh ID if user logs out
3. **Large index memory consumption**
   - Mitigation: LRU eviction of old sessions (>1 hour)
4. **Request ID collision**
   - Mitigation: Use cryptographically secure random with large space

## Backend SDK Requirements
Backend services MUST:
1. Extract `X-Session-ID` from incoming requests
2. Extract `traceparent` header for request ID
3. Include session_id in all logs
4. Propagate to downstream services
5. Include in error reporting

Example middleware:
```go
func extractCorrelationIDs(r *http.Request) (sessionID, requestID string) {
    sessionID = r.Header.Get("X-Session-ID")

    // Extract request ID from traceparent
    tp := r.Header.Get("traceparent")
    if tp != "" {
        parts := strings.Split(tp, "-")
        if len(parts) >= 2 {
            requestID = parts[1]  // trace-id portion
        }
    }
    return
}
```

## Backward Compatibility
- Session/Request correlation is opt-in (requires header support)
- Services without header support work without correlation
- Gradual rollout: add correlation to one service at a time
- No breaking changes to existing query APIs
