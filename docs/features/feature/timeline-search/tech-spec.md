---
status: proposed
scope: feature/timeline-search
ai-priority: high
tags: [v7, analysis, debugging]
relates-to: [product-spec.md, ../backend-log-streaming/tech-spec.md, ../backend-control/tech-spec.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-timeline-search
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Timeline & Search — Technical Specification

## Architecture

### System Diagram
```
┌─────────────────────────────────────────────────────┐
│  Kaboom MCP Server (Go)                           │
│  ┌───────────────────────────────────────────────┐  │
│  │ Timeline Query Engine                         │  │
│  │ - Parse timeline queries (correlation_id, etc)│  │
│  │ - Execute searches across event buffers       │  │
│  │ - Build causality chains                      │  │
│  │ - Generate export snapshots                   │  │
│  └───────────────────────────────────────────────┘  │
│  ↓                                                   │
│  ┌───────────────────────────────────────────────┐  │
│  │ Event Indexer                                 │  │
│  │ - Index events by correlation_id, timestamp   │  │
│  │ - Index by user_id, request_id, trace parent │  │
│  │ - Maintain reverse index for fast lookup      │  │
│  └───────────────────────────────────────────────┘  │
│  ↓                                                   │
│  ┌───────────────────────────────────────────────┐  │
│  │ Unified Event Stream                          │  │
│  │ - Merge from: frontend logs, network, backend │  │
│  │ - Sort by timestamp (microsecond precision)   │  │
│  │ - Deduplicate by event_id                     │  │
│  └───────────────────────────────────────────────┘  │
│  ↓                                                   │
│  ┌───────────────────────────────────────────────┐  │
│  │ Causality Analyzer                            │  │
│  │ - Link related events by correlation IDs      │  │
│  │ - Build dependency graphs                     │  │
│  │ - Detect causal chains (A→B→C)                │  │
│  └───────────────────────────────────────────────┘  │
│  ↓                                                   │
│  ┌───────────────────────────────────────────────┐  │
│  │ Export Manager                                │  │
│  │ - Snapshot timeline to JSON                   │  │
│  │ - Compress for sharing                        │  │
│  │ - Store exports with TTL                      │  │
│  └───────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────┘
          ↓
┌─────────────────────────────────────────────────────┐
│  Event Sources (Updated)                            │
│  - Frontend: logs buffer                            │
│  - Network: network_waterfall + network_bodies      │
│  - Backend: backend-logs                            │
│  - Code: modification_log                           │
│  - Infra: environment_audit                         │
└─────────────────────────────────────────────────────┘
```

### Data Flow: Build Timeline for Correlation ID
```
1. AI calls: observe({what: "timeline", correlation_id: "test-payment-001"})
2. Timeline engine receives query
3. Event indexer looks up all events with correlation_id
4. Events pulled from:
   - Frontend logs buffer (console logs with correlation_id)
   - Network waterfall (requests tagged with correlation_id)
   - Backend logs (logs with correlation_id)
   - Code modifications (with matching correlation_id)
   - Environment changes (with matching correlation_id)
5. Events merged and sorted by timestamp
6. Causality analyzer links related events
7. Response built: chronological timeline with causality chains
8. Return to AI: unified view of what happened
```

## Implementation Plan

### Phase 1: Event Collection & Indexing (Week 1)
1. **Event Normalization**
   - Define canonical event structure (all events conform to schema)
   - Extract correlation_id from all sources
   - Standardize timestamps (microseconds)
   - Normalize event types and severity

2. **Event Indexer**
   - Create in-memory hash index: correlation_id → [event_ids]
   - Create in-memory hash index: timestamp → [event_ids]
   - Create hash index: user_id → [event_ids]
   - Create hash index: request_id → [event_ids]
   - Update indexes as events arrive

3. **Event Merging**
   - Merge events from all buffers into single stream
   - Sort by timestamp
   - Deduplicate by event_id
   - Maintain causality metadata

### Phase 2: Search & Query (Week 2)
1. **Query Language**
   - Support: `correlation_id:X`, `user_id:Y`, `severity:error`
   - Support: `duration_ms:>5000`, `timestamp:[T1,T2]`
   - Support: `event_type:network_request`
   - Implement simple parser (no complex expressions)

2. **Search Engine**
   - Execute queries against indexes
   - Return matching events sorted by timestamp
   - Support pagination
   - Cache popular queries (1min TTL)

3. **Filter Support**
   - Filter by event_type
   - Filter by layer (frontend, network, backend, code, infrastructure)
   - Filter by severity
   - Filter by duration
   - Combine multiple filters with AND logic

### Phase 3: Causality & Export (Week 3)
1. **Causality Analysis**
   - Link events by correlation_id (primary)
   - Link events by trace_parent (secondary)
   - Link events by request_id (tertiary)
   - Build directed graphs (A causes B, B causes C)
   - Detect critical path (longest chain)

2. **Export Manager**
   - Snapshot timeline to JSON file
   - Compress with gzip
   - Store in `.kaboom/exports/`
   - TTL cleanup (7 days default)
   - Generate shareable export URLs

3. **Performance Optimization**
   - Index hot correlation IDs
   - Cache 1-hour time windows
   - Lazy-load full events (summary first, details on demand)

## API Changes

### New `observe()` mode: timeline
```javascript
observe({
  what: "timeline",
  correlation_id: "test-payment-001",
  include_causality: true,
  limit: 100
})
→ {
    query: "correlation_id:test-payment-001",
    total_events: 47,
    events: [
      {
        event_id: "evt-20260131-101523-001",
        timestamp: "2026-01-31T10:15:23.456789Z",
        event_type: "user_action",
        layer: "frontend",
        severity: "info",
        content: "Click button.checkout",
        related_events: 12,
        duration_ms: 0
      },
      {
        event_id: "evt-20260131-101523-002",
        timestamp: "2026-01-31T10:15:23.470Z",
        event_type: "network_request",
        layer: "network",
        severity: "info",
        content: "POST /api/payments (req-12345)",
        related_events: 8,
        duration_ms: 230
      },
      ...
    ],
    causality_chains: [
      "User clicked → Payment request sent → Backend timeout → Error returned → Error displayed"
    ]
  }
```

### New `observe()` mode: timeline_search
```javascript
observe({
  what: "timeline_search",
  query: "severity:error layer:backend",
  time_range: ["2026-01-31T10:15:00Z", "2026-01-31T10:16:00Z"],
  limit: 50
})
→ {
    total_matching: 12,
    events: [
      {
        event_id: "evt-...",
        timestamp: "...",
        event_type: "backend_log",
        content: "ERROR: Payment gateway timeout",
        severity: "error"
      },
      ...
    ],
    search_duration_ms: 35
  }
```

### New `interact()` mode: timeline_export
```javascript
interact({
  action: "timeline_export",
  correlation_id: "test-payment-001",
  time_range: ["2026-01-31T10:15:00Z", "2026-01-31T10:16:00Z"]
})
→ {
    export_id: "export-20260131-101600-001",
    file_path: ".kaboom/exports/test-payment-001-20260131.json.gz",
    size_bytes: 125000,
    events_count: 47,
    timestamp: "2026-01-31T10:16:00Z"
  }
```

### New `observe()` mode: timeline_stats
```javascript
observe({
  what: "timeline_stats",
  correlation_id: "test-payment-001"
})
→ {
    total_events: 47,
    events_by_type: {
      user_action: 5,
      network_request: 8,
      backend_log: 28,
      code_modification: 0,
      environment_change: 0
    },
    events_by_layer: {
      frontend: 13,
      network: 8,
      backend: 28,
      code: 0,
      infrastructure: 0
    },
    critical_path_length: 8,
    total_duration_ms: 2345,
    error_count: 3
  }
```

## Code References

### New files to create:
- `cmd/server/timeline/query.go` — Query parser and executor
- `cmd/server/timeline/indexer.go` — Event indexer
- `cmd/server/timeline/merger.go` — Event merging and sorting
- `cmd/server/timeline/causality.go` — Causality analyzer
- `cmd/server/timeline/export.go` — Export manager
- `cmd/server/timeline/events.go` — Event normalization types

### Existing files to modify:
- `cmd/server/mcp/observe.go` — Add timeline, timeline_search, timeline_stats modes
- `cmd/server/mcp/server.go` — Add timeline_export action to interact()
- All event sources — Ensure correlation_id propagated to all events

## Performance Requirements
- Timeline query: <200ms for 1-hour window (10K events)
- Search query: <100ms for typical filter
- Export generation: <500ms
- Event indexing: <5ms per new event
- Causality analysis: <50ms for 1K events

## Testing Strategy

### Unit Tests
1. Test event normalization
2. Test event indexing (correlation_id, timestamp)
3. Test query parsing
4. Test causality linking
5. Test export generation

### Integration Tests
1. Ingest events from all sources
2. Query by correlation_id
3. Query by user_id, request_id
4. Verify timestamp ordering
5. Verify causality chains build correctly
6. Export timeline, verify JSON structure
7. Import timeline, verify reconstruction

### E2E Tests
1. Full flow: user action → network request → backend log → error → fix
2. Query timeline for correlation_id
3. Verify all events present and ordered
4. Verify causality chain correct
5. Export timeline
6. Share with team member
7. Verify reconstruction matches original

## Dependencies
- All other features (to receive their events)
- Git (for modification metadata)
- Time precision: microseconds

## Event Normalization

All events must include:
```go
type TimelineEvent struct {
  EventID       string    // Unique ID
  Timestamp     time.Time // Microsecond precision
  CorrelationID string    // Link to other events
  EventType     string    // user_action, network_request, etc.
  Layer         string    // frontend, network, backend, code, infrastructure
  Severity      string    // info, warning, error, critical
  Content       string    // Human-readable description
  Metadata      map[string]interface{}  // Extensible metadata
  DurationMS    int64     // For events with duration
}
```

Event sources provide:
- Frontend logs: console logs with correlation_id
- Network: requests/responses with correlation_id, request_id, trace_parent
- Backend logs: structured logs with correlation_id, request_id, span_id
- Code: modifications with correlation_id, reason
- Infrastructure: environment changes with correlation_id

## Search Query Language

Simple DSL (no complex expressions):
```
correlation_id:test-001          # Exact match
user_id:987                       # Exact match
severity:error                    # Exact match
event_type:network_request        # Exact match
duration_ms:>5000                 # Comparison
timestamp:[2026-01-31T10:15Z,2026-01-31T10:16Z]  # Range
layer:backend                     # Exact match
```

Combining: `severity:error layer:backend` (AND)
