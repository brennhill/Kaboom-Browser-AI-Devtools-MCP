---
status: proposed
scope: feature/timeline-search
ai-priority: high
tags: [v7, analysis, hands, debugging]
relates-to: [../backend-control/product-spec.md, ../code-navigation-modification/product-spec.md, ../../core/architecture.md]
last-verified: 2026-03-05
doc_type: product-spec
feature_id: feature-timeline-search
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Timeline & Search

## Overview
Timeline & Search enables developers and AI to construct unified, searchable timelines of all events across the full stack—frontend user actions, network requests, backend logs, code modifications, and environment changes. A timeline visually shows causality: "User clicked X → frontend sent request Y → backend processed Z → response failed → frontend showed error → developer fixed code." This feature answers the critical debugging question: "What exactly happened and in what order?"

## Problem
Currently, debugging requires jumping between multiple tools:
- Chrome DevTools for frontend events
- Network tab for HTTP traffic
- Backend log aggregator (CloudWatch, Datadog, ELK) for server logs
- Git history for code changes
- Manual notes for environment changes

Each tool has its own timeline, making root cause analysis tedious. A click in frontend DevTools might be timestamped 10:15:23, but matching backend logs requires cross-referencing request IDs and guessing which log line corresponds to the failure.

## Solution
Timeline & Search provides:
1. **Unified Timeline** — All events from all layers on single timeline with microsecond precision
2. **Correlation** — All events linked by correlation_id, trace parent, request ID, user ID, session ID
3. **Rich Search** — Query events across layers: "Show me all errors correlated with user 12345 in the last 5 minutes"
4. **Causality Visualization** — Show dependency chains: "This error was caused by this click which triggered this request which failed because..."
5. **Event Streaming** — Real-time event stream, or historical reconstruction for past sessions
6. **Export & Share** — Timeline snapshots for sharing with team members or AI models

All operations support:
- **Correlation IDs** — Every event tagged with IDs that link it to other events
- **Performance** — Query <100ms for typical debugging windows (1 hour)
- **Searchability** — Full-text search, regex patterns, semantic queries
- **Context** — Each event includes enough context to understand it without source lookup

## User Stories
- As a developer, I want to see all events (user actions, network requests, backend logs, errors) on a single timeline so that I can understand the sequence of events leading to a failure
- As an AI agent, I want to search for "all requests that took >5s" so that I can identify performance bottlenecks
- As a developer, I want to filter timeline to show only errors to narrow down debugging focus
- As a developer, I want to export a timeline snapshot so that I can share it with my team for collaborative debugging
- As an AI agent, I want to trace a user action through all layers (frontend → network → backend → database) so that I can identify where the slowdown occurs
- As a developer, I want to replay a timeline from 5 minutes ago so that I can reconstruct what happened during a production issue

## Acceptance Criteria
- [ ] Unified timeline displays events in microsecond-precision order
- [ ] Timeline includes: user actions, network requests, backend logs, code modifications, environment changes
- [ ] All events searchable by correlation_id, user_id, request_id, timestamp, content
- [ ] Search queries complete in <100ms for 1-hour windows
- [ ] Causality chains visible (click → request → response → error)
- [ ] Timeline exportable as JSON with timestamps and metadata
- [ ] Timeline filterable by event type, severity, duration
- [ ] Performance: timeline query <200ms for typical 1-hour window (10K events)
- [ ] Correlation IDs propagate through all layers
- [ ] Support searching historical timelines (past sessions)

## Not In Scope
- Visual UI timeline graph (text-based search results sufficient)
- Real-time video playback/replay (timestamps and snapshots sufficient)
- ML-based anomaly detection (patterns and thresholds only)
- Cross-session correlation (single session only)
- Predictive root cause (only correlations visible)

## Data Structures

### Timeline Event
```json
{
  "event_id": "evt-20260131-101523-001",
  "timestamp": "2026-01-31T10:15:23.456789Z",
  "correlation_id": "test-payment-flow-001",
  "event_type": "user_action|network_request|backend_log|code_modification|environment_change",
  "layer": "frontend|network|backend|code|infrastructure",
  "severity": "info|warning|error|critical",
  "duration_ms": 1234,
  "source": {
    "file": "src/components/PaymentForm.tsx",
    "line": 42,
    "function": "handleSubmit"
  },
  "content": "Click button.checkout",
  "metadata": {
    "user_id": 987,
    "request_id": "req-12345",
    "trace_parent": "00-abc123-def456-01",
    "status_code": 500,
    "error_code": "TIMEOUT"
  },
  "related_events": ["evt-20260131-101523-002", "evt-20260131-101523-003"]
}
```

### Timeline Query Result
```json
{
  "query": "correlation_id:test-payment-flow-001 severity:error",
  "total_events": 47,
  "matching_events": 3,
  "events": [
    {
      "event_id": "evt-20260131-101523-002",
      "timestamp": "2026-01-31T10:15:23.234Z",
      "event_type": "user_action",
      "content": "Click button.checkout",
      "related_count": 12
    }
  ],
  "causality_chain": [
    "User clicked button → Frontend sent request → Backend timeout → Error displayed"
  ]
}
```

### Timeline Export
```json
{
  "export_id": "export-20260131-101600-001",
  "session_id": "session-001",
  "time_range": ["2026-01-31T10:00:00Z", "2026-01-31T10:30:00Z"],
  "total_events": 1247,
  "events": [
    {...event...}
  ],
  "summary": {
    "user_actions": 156,
    "network_requests": 234,
    "backend_logs": 412,
    "errors": 23,
    "avg_request_duration_ms": 245
  }
}
```

## Examples

### Example 1: View All Events for Correlation ID
```javascript
// AI wants to see full flow for a user interaction
const result = await observe({
  what: "timeline",
  correlation_id: "test-payment-flow-001"
});

// Returns chronological events:
// [10:15:23.100] User Action: Click button.checkout
// [10:15:23.120] Network: XHR POST /api/payments (req-12345)
// [10:15:23.200] Backend: INFO api-server: Payment processing started
// [10:15:23.300] Backend: DEBUG api-server: Checking payment gateway
// [10:15:23.500] Backend: ERROR api-server: Gateway timeout (span-789)
// [10:15:23.450] Network: XHR Response: 500 error_code=TIMEOUT
// [10:15:23.500] Frontend: Error message displayed to user
```

### Example 2: Search for Slow Requests
```javascript
// Find all requests that took longer than 5 seconds
const result = await observe({
  what: "timeline",
  filter: "event_type:network_request duration_ms:>5000",
  limit: 50
});

// Returns requests sorted by duration
```

### Example 3: Trace User Through Full Stack
```javascript
// AI wants to understand path of user request
const result = await observe({
  what: "timeline",
  filter: "user_id:987 timestamp:[2026-01-31T10:15:00Z,2026-01-31T10:16:00Z]"
});

// Returns all events for that user in time window:
// - All their clicks
// - All their requests
// - All backend processing for their requests
// - Any errors they encountered
```

### Example 4: Export Timeline for Team Review
```javascript
// Developer wants to share investigation with team
const export_result = await interact({
  action: "timeline_export",
  correlation_id: "bug-payment-timeout-001",
  time_range: ["2026-01-31T10:00:00Z", "2026-01-31T10:30:00Z"]
});

// Returns export_id and file path
// Team members can review the exported timeline
```

### Example 5: Identify Causality Chain
```javascript
// AI wants to understand root cause
const result = await observe({
  what: "timeline",
  correlation_id: "test-payment-flow-001",
  include_causality: true
});

// Response shows:
// "Causality chain: User clicked → Payment request sent →
//  Backend gateway timeout → 500 error returned →
//  Frontend error message shown"
```

## MCP Tool Changes

### New `observe()` mode: timeline
```javascript
observe({
  what: "timeline",
  correlation_id: "test-payment-flow-001",  // Filter by correlation
  filter: "severity:error duration_ms:>1000",  // Query language
  time_range: ["2026-01-31T10:15:00Z", "2026-01-31T10:16:00Z"],  // Optional
  include_causality: true,  // Show causal chains
  limit: 100
})
→ {
    query: "correlation_id:test-payment-flow-001",
    total_events: 47,
    events: [
      {
        event_id: "evt-...",
        timestamp: "...",
        event_type: "user_action",
        content: "Click button.checkout",
        related_events: [...]
      },
      ...
    ],
    causality_chains: [
      "User clicked → Request sent → Gateway timeout → Error displayed"
    ]
  }
```

### New `observe()` mode: timeline_search
```javascript
observe({
  what: "timeline_search",
  query: "severity:error user_id:987",  // Search language
  time_range: ["2026-01-31T10:00:00Z", "2026-01-31T10:30:00Z"],
  event_types: ["user_action", "network_request", "backend_log"],  // Filter
  limit: 100
})
→ {
    total_matching: 23,
    events: [...],
    search_duration_ms: 45
  }
```

### New `interact()` mode: timeline_export
```javascript
interact({
  action: "timeline_export",
  correlation_id: "test-payment-flow-001",
  time_range: ["2026-01-31T10:15:00Z", "2026-01-31T10:16:00Z"]
})
→ {
    export_id: "export-20260131-101600-001",
    file_path: ".kaboom/exports/export-*.json",
    size_bytes: 125000,
    events_count: 47,
    timestamp: "2026-01-31T10:16:00Z"
  }
```

### New `observe()` mode: timeline_stats
```javascript
observe({
  what: "timeline_stats",
  correlation_id: "test-payment-flow-001"
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
    errors: 3,
    warnings: 5,
    duration_ms: 2345,
    critical_path: "user_action → network_request → backend_error"
  }
```
