---
status: draft
priority: tier-1
phase: v6.0-sprint-1
relates-to: [browser-extension-enhancement, ring-buffer]
blocks: [query-service, correlation-engine]
last-updated: 2026-01-31
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Normalized Event Schema — Technical Specification

**Goal:** Design unified JSON event format across all sources (browser, backend, tests, git) for AI-native querying and correlation.

---

## Problem Statement

v5.3 sends disparate telemetry:
- Browser console: `{level: string, args: any[], timestamp: number}`
- Browser network: `{method: string, url: string, status: number, duration: number}`
- Backend logs: Plain text or JSON with no standard structure
- Tests: Test runner output (Jest JSON, pytest output, etc.)
- Git: Commit messages, file changes

**Challenge:** AI can't reason about cross-system causality when each source has different schemas.

**Solution:** Single `NormalizedEvent` interface that all sources emit to, enabling:
- LLM to query all events uniformly via MCP
- Correlation engine (v7) to link events across systems
- Regression detection to compare before/after states
- Test generation to include context from multiple sources

---

## Core Event Schema

```typescript
interface NormalizedEvent {
  // Unique identification
  id: string;                    // UUID, e.g. "550e8400-e29b-41d4-a716-446655440000"

  // Temporal
  timestamp: number;             // milliseconds since epoch, e.g. 1704067200000

  // Source identification
  source: string;                // "browser", "backend", "test", "git"
  source_version: string;        // e.g. "v5.3.0" for browser extension

  // Severity/log level
  level: string;                 // "debug", "info", "warn", "error", "critical"

  // Correlation (v6+)
  correlation_id?: string;       // Optional, links related events

  // Trace context (v7+)
  trace_id?: string;             // W3C Trace Context, e.g. "4bf92f3577b34da6a3ce929d0e0e4736"
  span_id?: string;              // W3C Trace Context span
  parent_span_id?: string;       // Parent span for causality

  // Message content
  message: string;               // Primary message, max 1KB

  // Extensible metadata
  metadata: Record<string, any>; // Source-specific details

  // Categorization
  tags: string[];                // e.g. ["form", "validation", "error"]

  // Optional: source-specific details
  source_details?: {
    [key: string]: any;
  };
}
```

---

## Source-Specific Variants

### Browser Events

```typescript
interface BrowserNormalizedEvent extends NormalizedEvent {
  source: "browser";

  metadata: {
    // Event type classification
    event_type: "log" | "network" | "exception" | "action" | "snapshot" | "accessibility" | "websocket";

    // For console logs
    console_level?: "log" | "info" | "warn" | "error";
    console_args?: any[];
    console_stacktrace?: string;

    // For network events
    network?: {
      method: string;            // GET, POST, etc.
      url: string;               // Full URL
      status?: number;           // HTTP status code
      status_text?: string;       // e.g. "Not Found"
      duration_ms: number;        // Request duration
      request_headers?: Record<string, string>;
      response_headers?: Record<string, string>;
      request_body?: string;      // First 1KB
      response_body?: string;     // First 1KB
      content_type?: string;
      error?: string;             // If request failed
    };

    // For user actions
    action?: {
      type: "click" | "type" | "navigate" | "scroll" | "focus" | "blur";
      element_selector: string;
      smart_selector?: {
        primary: string;
        fallback: string;
        semantic: string;
        confidence_pct: number;
      };
      element_text?: string;
      element_coordinates?: [number, number];  // x, y
      timing_ms?: number;
      dom_changes?: string[];
    };

    // For exceptions
    exception?: {
      message: string;
      name: string;
      stack: string;
      source: string;
      lineno: number;
      colno: number;
    };

    // For DOM snapshots
    snapshot?: {
      snapshot_id: string;
      action_id?: string;
      direction: "before" | "after";
      html: string;               // Compressed HTML
      element_count: number;
      checksum: string;
      diff?: Array<{
        type: string;
        description?: string;
      }>;
    };

    // For accessibility context
    accessibility?: {
      element_role: string;
      aria_label?: string;
      aria_description?: string;
      aria_disabled?: boolean;
      violations: Array<{
        criterion: string;
        level: "A" | "AA" | "AAA";
        severity: "critical" | "major" | "minor";
        message: string;
      }>;
    };

    // For WebSocket events
    websocket?: {
      url: string;
      event_type: "connect" | "message" | "error" | "close";
      message?: string;
      status_code?: number;
    };

    // Browser context
    page_url: string;
    page_title: string;
    user_agent: string;
    window_size?: [number, number];
  };
}
```

### Backend Events

```typescript
interface BackendNormalizedEvent extends NormalizedEvent {
  source: "backend";

  metadata: {
    // Event type classification
    event_type: "log" | "network" | "error" | "metric";

    // For logs
    log_level?: "trace" | "debug" | "info" | "warn" | "error" | "fatal";
    logger?: string;            // e.g. "auth", "db", "api"

    // For structured logs
    structured?: {
      [key: string]: any;       // User ID, request ID, etc.
    };

    // For network events (incoming requests)
    incoming_request?: {
      method: string;
      path: string;
      query?: Record<string, any>;
      headers?: Record<string, string>;
      body?: string;
      duration_ms: number;
    };

    // For network events (outgoing requests)
    outgoing_request?: {
      method: string;
      url: string;
      status: number;
      duration_ms: number;
      error?: string;
    };

    // For errors
    error?: {
      type: string;
      message: string;
      stack?: string;
    };

    // For metrics
    metric?: {
      name: string;
      value: number;
      unit: string;             // e.g. "ms", "mb", "percent"
    };

    // Backend context
    service_name: string;       // e.g. "auth-service", "db-service"
    service_version: string;    // e.g. "1.2.3"
    environment: string;        // "dev", "staging", "prod"
    instance_id?: string;
    request_id?: string;        // Correlation with other services
  };
}
```

### Test Events

```typescript
interface TestNormalizedEvent extends NormalizedEvent {
  source: "test";

  metadata: {
    // Event type classification
    event_type: "test_start" | "test_pass" | "test_fail" | "test_skip" | "suite_start" | "suite_pass" | "suite_fail";

    // Test identification
    test_name: string;
    test_file: string;
    test_framework: string;     // "jest", "pytest", "mocha", "go", etc.
    test_suite?: string;
    test_id?: string;

    // Test results
    result?: {
      status: "pass" | "fail" | "skip" | "error";
      duration_ms: number;
      error_message?: string;
      error_stack?: string;
      stdout?: string;           // Test output
      stderr?: string;
      attachments?: string[];    // Paths to screenshots, videos, etc.
    };

    // Test coverage
    coverage?: {
      files: string[];
      statements: number;
      branches: number;
      functions: number;
      lines: number;
    };

    // Test context
    command: string;            // e.g. "npm test", "pytest", "go test"
    environment?: Record<string, string>;
  };
}
```

### Git Events

```typescript
interface GitNormalizedEvent extends NormalizedEvent {
  source: "git";

  metadata: {
    // Event type classification
    event_type: "commit" | "branch" | "merge" | "tag" | "file_change";

    // Commit information
    commit?: {
      sha: string;
      message: string;
      author: string;
      timestamp: number;
      files_changed: Array<{
        path: string;
        status: "added" | "modified" | "deleted" | "renamed";
        additions?: number;
        deletions?: number;
        patch?: string;           // First 1KB of diff
      }>;
    };

    // Branch information
    branch?: {
      name: string;
      ahead_of_main?: number;
      behind_main?: number;
    };

    // Tag information
    tag?: {
      name: string;
      message?: string;
      commit_sha: string;
    };

    // File change detection
    file_change?: {
      path: string;
      type: "added" | "modified" | "deleted";
      before_lines?: number;
      after_lines?: number;
      language?: string;        // "typescript", "go", "python"
    };
  };
}
```

### Custom Events

```typescript
interface CustomNormalizedEvent extends NormalizedEvent {
  source: "custom";

  metadata: {
    // Custom event type
    event_type: string;

    // Custom data (user-defined)
    custom_data?: Record<string, any>;
  };
}
```

---

## Event Routing & Tagging

### Automatic Tagging Rules

Events automatically get tags for LLM filtering:

```typescript
const TAG_RULES = [
  // Feature area tags
  { match: {metadata.network?.url: /checkout/i}, tags: ["checkout", "shopping"] },
  { match: {metadata.network?.url: /auth|login/i}, tags: ["authentication", "security"] },
  { match: {metadata.network?.url: /payment/i}, tags: ["payment", "billing"] },

  // Severity tags
  { match: {level: "error"}, tags: ["error", "issue"] },
  { match: {level: "critical"}, tags: ["critical", "blocker"] },
  { match: {level: "warn"}, tags: ["warning", "caution"] },

  // Performance tags
  { match: {metadata.network?.duration_ms: ">1000"}, tags: ["slow", "performance"] },
  { match: {metadata.action?.timing_ms: ">500"}, tags: ["slow_ui", "performance"] },

  // Type tags
  { match: {metadata.event_type: "exception"}, tags: ["error", "javascript"] },
  { match: {metadata.event_type: "test_fail"}, tags: ["test", "failure"] },
];
```

### User-Defined Tagging via API

```typescript
// Developer can add context tags
window.__kaboom.annotate({
  message: "User logged in successfully",
  level: "info",
  tags: ["authentication", "user_flow"],
  metadata: {
    user_id: "user_123",
    session_duration_ms: 1234,
  },
});

// Emits NormalizedEvent with custom tags
```

---

## Serialization & Storage

### JSON Serialization

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": 1704067200000,
  "source": "browser",
  "source_version": "5.3.0",
  "level": "error",
  "correlation_id": "req_abc123",
  "message": "Network request failed: POST /api/checkout",
  "tags": ["checkout", "error", "network"],
  "metadata": {
    "event_type": "network",
    "network": {
      "method": "POST",
      "url": "http://localhost:3000/api/checkout",
      "status": 500,
      "duration_ms": 1234,
      "response_body": "{\"error\":\"Database connection failed\"}"
    },
    "page_url": "http://localhost:3000/checkout",
    "page_title": "Checkout"
  }
}
```

### Ring Buffer Storage

Events stored as binary in Go:

```go
// Using encoding/json (standard library, no deps)
jsonBytes, _ := json.Marshal(event)
buffer.Push(event)  // Stores in circular array
```

### Size Estimates

```
Average NormalizedEvent JSON: ~500 bytes
With network body (1KB): ~1.5KB
Average: ~2KB per event

Ring buffer capacity breakdown:
- Browser logs (10K events):     20MB
- Browser network (5K events):   10MB
- Backend logs (50K events):    100MB
- Total:                       ~200MB

Fits comfortably in memory, queryable in <50ms
```

---

## Query Interface via MCP

LLM queries events using MCP observe() tool:

```typescript
// Example 1: Get all errors in past hour
observe({
  what: "logs",
  filter: {
    level: "error",
    since_minutes: 60,
  },
})
// Returns: All error events from past hour across all sources

// Example 2: Get checkout flow
observe({
  what: "timeline",
  filter: {
    tags: "checkout",
    limit: 100,
  },
})
// Returns: Last 100 events tagged with "checkout" in chronological order

// Example 3: Get network requests
observe({
  what: "network_waterfall",
  filter: {
    url_contains: "/api",
    status_min: 400,
  },
})
// Returns: All failed API requests with timing

// Example 4: Correlation (v7+)
observe({
  what: "correlation",
  trace_id: "4bf92f3577b34da6a3ce929d0e0e4736",
})
// Returns: All events linked by trace ID
```

---

## Schema Versioning

### Version 1.0 (v6.0)

Core fields:
- id, timestamp, source, level, correlation_id
- message, metadata, tags

Supported sources:
- browser (logs, network, actions, snapshots)
- backend (logs)
- test (test events)
- git (commits)

### Version 2.0 (v7.0)

Additional fields:
- trace_id, span_id, parent_span_id (W3C Trace Context)
- source_version improvements
- Enhanced correlation metadata

### Compatibility

```go
// Migration: If event is missing v2.0 fields, treat as v1.0
if event.TraceID == "" {
  // v1.0 event, use correlation_id instead
  event.TraceID = event.CorrelationID
}
```

---

## Conversion Rules

### From Browser Console (v5.3 legacy)

```typescript
// v5.3:
{
  level: "error",
  args: ["Network failed", {status: 500}],
  timestamp: 1704067200000,
}

// → v6.0 NormalizedEvent:
{
  id: "uuid...",
  timestamp: 1704067200000,
  source: "browser",
  level: "error",
  message: "Network failed",
  metadata: {
    event_type: "log",
    console_level: "error",
    console_args: ["Network failed", {status: 500}],
  },
  tags: ["console", "error"],
}
```

### From Backend Plaintext Log

```typescript
// Plaintext:
"[2024-01-01T12:00:00Z] ERROR auth-service: Failed to authenticate user user_123: Invalid token"

// → v6.0 NormalizedEvent:
{
  id: "uuid...",
  timestamp: 1704067200000,
  source: "backend",
  level: "error",
  message: "Failed to authenticate user user_123: Invalid token",
  metadata: {
    event_type: "log",
    logger: "auth-service",
    service_name: "auth-service",
  },
  tags: ["authentication", "error"],
}
```

### From Jest Test Output

```typescript
// Jest JSON:
{
  testResults: [{
    title: "should validate 8-char password",
    fullName: "LoginForm should validate 8-char password",
    status: "failed",
    duration: 1234,
    failureMessages: ["AssertionError: expected..."],
  }]
}

// → v6.0 NormalizedEvent:
{
  id: "uuid...",
  timestamp: 1704067200000,
  source: "test",
  level: "error",
  message: "Test failed: should validate 8-char password",
  metadata: {
    event_type: "test_fail",
    test_name: "should validate 8-char password",
    test_framework: "jest",
    result: {
      status: "fail",
      duration_ms: 1234,
      error_message: "AssertionError: expected...",
    },
  },
  tags: ["test", "failure"],
}
```

---

## Implementation Checklist

### Phase 1: Core Schema (v6.0 Sprint 1)

- [ ] Define TypeScript interfaces for all event types
- [ ] Create JSON schema for validation
- [ ] Implement NormalizedEvent type in Go
- [ ] Create conversion functions (browser console → NormalizedEvent)
- [ ] Test serialization/deserialization
- [ ] Document all fields and metadata keys
- [ ] Performance test: Serialize 10K events in <100ms

### Phase 2: Integration (v6.0 Sprint 2)

- [ ] Update browser extension to emit NormalizedEvent
- [ ] Update backend log streaming to normalize events
- [ ] Update test capture to emit NormalizedEvent
- [ ] Implement automatic tagging rules
- [ ] Add window.__kaboom.annotate() for custom events
- [ ] Test MCP observe() filtering

### Phase 3: Correlation (v7.0)

- [ ] Add W3C Trace Context fields
- [ ] Implement trace ID propagation
- [ ] Build correlation engine
- [ ] Test cross-system event linking

---

## Related Documents

- **Browser Enhancement:** [browser-extension-enhancement/TECH_SPEC.md](../browser-extension-enhancement/TECH_SPEC.md)
- **Ring Buffer:** [ring-buffer/TECH_SPEC.md](../ring-buffer/TECH_SPEC.md)
- **Query Service:** [query-service/TECH_SPEC.md](../query-service/index.md) (Sprint 2)
- **Architecture:** [360-observability-architecture.md](../../../core/product/README.md#normalized-event-schema)

---

**Status:** Ready for implementation
**Estimated Effort:** 2 days (Sprint 1)
**Dependencies:** None (pure data structure)
