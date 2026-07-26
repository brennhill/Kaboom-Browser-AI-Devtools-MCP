---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-03-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Data Capture Pipeline: Continuous Telemetry Flow

## Overview

The data capture pipeline is a continuous, event-driven system where the page injection script observes browser events and streams them to the server via batched HTTP requests.

---

## Complete Data Flow Architecture

```mermaid
graph TB
    subgraph PageLevel["🔍 Page Level (src/inject/)"]
        direction TB
        Console["console observer<br/>- console.log<br/>- console.error<br/>- console.warn"]
        Fetch["fetch observer<br/>- request headers<br/>- response status<br/>- response body"]
        XHR["XHR observer<br/>- XMLHttpRequest<br/>- request/response"]
        WS["WebSocket observer<br/>- open/message/close<br/>- message data"]
        Perf["PerformanceObserver<br/>- LCP, FCP, CLS<br/>- Resource timing<br/>- Navigation timing"]
        Error["Error listener<br/>- uncaughtError<br/>- unhandledRejection<br/>- error stacks"]
        Actions["Click/Input/Nav listener<br/>- click events<br/>- input changes<br/>- navigation"]
    end

    subgraph Serialization["📦 Serialization (src/lib/)"]
        direction LR
        Serialize["serialize.ts<br/>Circular ref handling<br/>Large object limits<br/>JSON-safe conversion"]
        Redact["redaction.ts<br/>PII masking<br/>Password removal<br/>Token redaction"]
        Enrich["ai-context.ts<br/>Error enrichment<br/>Stack parsing<br/>Context annotation"]
    end

    subgraph Memory["💾 Memory (src/background/)"]
        direction LR
        StateMemory["State Manager<br/>chrome.storage.local<br/>Pending events<br/>Session state"]
        Batchers["Event Batchers<br/>Accumulate events<br/>Debounce<br/>Max batch size"]
    end

    subgraph Transport["🚀 Transport (src/background/sync/sync-client.ts)"]
        direction TB
        BatchAssemble["Assemble Batch<br/>- Collect pending events<br/>- Tab metadata<br/>- Timestamps"]
        RetryLogic["Retry Logic<br/>- Exponential backoff<br/>- Max 3 attempts<br/>- Fallback to cache"]
        HTTPPost["POST /sync<br/>- Content-Type: JSON<br/>- Authorization header<br/>- Batch size: < 10MB"]
    end

    subgraph ServerSide["🟠 Server (internal/capture/)"]
        direction TB
        SyncHandler["Sync Handler<br/>(handlers.go)"]
        Router["Router<br/>By event type"]

        subgraph Buffers["Ring Buffers"]
            LogBuf["logs buffer<br/>Max 1000 entries<br/>Max 50MB"]
            NetworkBuf["network_bodies<br/>Max 100 entries<br/>Max 200MB"]
            WaterfallBuf["network_waterfall<br/>Max 500 entries<br/>Max 20MB"]
            ActionsBuf["actions buffer<br/>Max 5000 entries<br/>Max 30MB"]
            WSBuf["websocket buffer<br/>Max 500 entries<br/>Max 50MB"]
            PerfBuf["performance buffer<br/>Max 500 entries<br/>Max 20MB"]
        end

        subgraph Management["Resource Management"]
            Memory_["Memory tracking<br/>Per-buffer sizing"]
            TTL["TTL eviction<br/>Older entries first"]
            Limits["Size enforcement<br/>FIFO cleanup"]
        end

        SyncHandler --> Router
        Router --> LogBuf
        Router --> NetworkBuf
        Router --> WaterfallBuf
        Router --> ActionsBuf
        Router --> WSBuf
        Router --> PerfBuf

        Buffers --> Management
    end

    subgraph Queries["🔎 Query Layer (internal/capture/)"]
        PendingQueries["Pending Queries<br/>Max 5 per client<br/>30s timeout"]
        CompletedResults["Completed Results<br/>Max 100 per client<br/>60s expiry"]
        Dispatcher["Query Dispatcher<br/>Route to extension<br/>Track correlation_id"]
    end

    subgraph Analysis["🧠 Analysis (internal/analysis/)"]
        Clustering["Error Clustering<br/>Group by 2-of-3 signals<br/>Root cause detection"]
        APISchema["API Schema Learning<br/>Infer from requests<br/>Contract validation"]
    end

    %% Data flows
    Console --> Serialize
    Fetch --> Serialize
    XHR --> Serialize
    WS --> Serialize
    Perf --> Serialize
    Error --> Serialize
    Actions --> Serialize

    Serialize --> Redact
    Redact --> Enrich

    Enrich --> StateMemory
    Enrich --> Batchers

    Batchers --> BatchAssemble
    StateMemory --> BatchAssemble

    BatchAssemble --> RetryLogic
    RetryLogic --> HTTPPost

    HTTPPost -->|POST /sync| SyncHandler

    SyncHandler --> Router

    LogBuf -.->|Cache| StateMemory
    NetworkBuf -.->|Cache| StateMemory
    ActionsBuf -.->|Cache| StateMemory

    PendingQueries -.->|Route to| Transport
    CompletedResults -.->|Storage| SyncHandler

    Buffers -.->|Input| Clustering
    Buffers -.->|Input| APISchema

    %% Styling
    classDef page fill:#a371f7,stroke:#8957e5,stroke-width:2px,color:#fff
    classDef serialize fill:#fde047,stroke:#d29922,stroke-width:2px,color:#000
    classDef memory fill:#79c0ff,stroke:#388bfd,stroke-width:2px,color:#000
    classDef transport fill:#58a6ff,stroke:#1f6feb,stroke-width:2px,color:#fff
    classDef server fill:#fb923c,stroke:#f97316,stroke-width:2px,color:#fff
    classDef buffer fill:#3fb950,stroke:#2ea043,stroke-width:2px,color:#fff
    classDef query fill:#79c0ff,stroke:#388bfd,stroke-width:2px,color:#000
    classDef analysis fill:#f85149,stroke:#da3633,stroke-width:2px,color:#fff

    class Console,Fetch,XHR,WS,Perf,Error,Actions page
    class Serialize,Redact,Enrich serialize
    class StateMemory,Batchers memory
    class BatchAssemble,RetryLogic,HTTPPost transport
    class SyncHandler,Router server
    class LogBuf,NetworkBuf,WaterfallBuf,ActionsBuf,WSBuf,PerfBuf buffer
    class PendingQueries,CompletedResults,Dispatcher query
    class Clustering,APISchema analysis
```

---

## Step-by-Step Example: Network Request Capture

```mermaid
sequenceDiagram
    participant User as User
    participant Page as Web Page<br/>(JavaScript)
    participant Inject as Injection<br/>(inject.ts)
    participant BG as Background<br/>Worker
    participant Server as Go Server<br/>(Capture)

    User->>Page: Clicks checkout button
    Page->>Page: fetch('/api/checkout', {...})

    Note over Inject: Fetch observer intercepts

    Inject->>Inject: 1. Original fetch() continues<br/>to network

    par Network Request
        Page->>Page: Network request in progress
    and Capture Request
        Inject->>Inject: 2a. Extract request headers<br/>method, URL, body (first 1MB)<br/>timestamp

        Inject->>Inject: 2b. Wait for response<br/>in .then()

        Page-->>Inject: Response received<br/>status 200

        Inject->>Inject: 2c. Extract response headers<br/>Content-Type, Content-Length<br/>timestamp, status

        Inject->>Inject: 2d. Clone response<br/>response.clone().arrayBuffer()<br/>(up to 5MB)

        Inject->>Inject: 2e. Parse body<br/>JSON if JSON type<br/>Text if HTML<br/>Binary if image

        Inject->>Inject: 3. Create network event<br/>{<br/>type: 'network',<br/>method: 'POST',<br/>url: '/api/checkout',<br/>status: 200,<br/>duration_ms: 234,<br/>request_body: {...},<br/>response_body: {...},<br/>timestamp: 1707346800000<br/>}

        Inject->>BG: 4. Store in memory<br/>events.push(networkEvent)
    end

    Note over Inject,BG: Network request completes<br/>Capture completes in parallel

    BG->>BG: 5. Batch accumulation<br/>Wait up to 1 second or<br/>until 100 events

    BG->>BG: 6. Assemble /sync POST<br/>{<br/>events: [<br/>  {network event},<br/>  {other events from past 1s}<br/>],<br/>tab_id: 123,<br/>url: 'https://example.com'<br/>}

    BG->>Server: 7. POST /sync (JSON batch)

    Server->>Server: 8a. Parse batch

    Server->>Server: 8b. For each network event:<br/>- Extract correlation ID<br/>- Create waterfall entry<br/>- Store full body<br/>- Index by URL/status

    Server->>Server: 9a. Store in buffers:<br/>- network_waterfall[]<br/>  {url, method, status,<br/>   timing, timestamp}<br/>- network_bodies[url]<br/>  {req_headers, req_body,<br/>   resp_headers, resp_body}

    Server->>Server: 9b. Check analysis:<br/>- Cluster if error<br/>- Learn API schema<br/>- Detect 3rd-party

    Server-->>BG: 10. 200 OK<br/>{received: true,<br/>ttl_seconds: 60}

    BG->>BG: 11. Clear event batch<br/>Continue polling

    Note over Server: Buffered for 60s<br/>Available to:<br/>- observe({what: 'network'})<br/>- generate({format: 'har'})<br/>- API schema learning
```

---

## Buffer Management: Memory Enforcement

```mermaid
stateDiagram-v2
    [*] --> Adding: Event arrives

    Adding --> Checking: Add to buffer
    Checking --> OK: Size < limit?

    OK --> Storing: Store event

    Checking --> Evicting: Size > limit
    Evicting --> TTLCheck: Check oldest<br/>TTL expired?

    TTLCheck --> Remove1: Remove oldest<br/>Check size again
    Remove1 --> Storing: Size OK?

    TTLCheck --> Size: Still too large?
    Size --> Remove2: Remove oldest<br/>regardless of TTL
    Remove2 --> Storing: Forced cleanup

    Storing --> [*]

    note right of Checking
        Ring buffer FIFO
        Max size: varies by buffer
        - logs: 50MB
        - network_bodies: 200MB
        - actions: 30MB
    end note

    note right of TTLCheck
        First strategy:
        Remove entries older than:
        - logs: 5 minutes
        - network: 10 minutes
        - actions: 10 minutes
    end note

    note right of Remove2
        Force cleanup if
        TTL removal not enough
        Remove oldest 10% of buffer
        Preserve most recent
    end note
```

---

## Event Type Details

### 1. Console Events
```json
{
  "type": "console",
  "level": "log|warn|error|info|debug",
  "timestamp": 1707346800000,
  "args": ["message", {object}, 123],
  "stack": null,
  "context": {
    "url": "https://example.com",
    "line": 42,
    "source": "app.js"
  }
}
```

**Captured From:** console.log/warn/error/info/debug
**Size Limit:** Each arg up to 1KB, total 10KB per entry
**Buffer Size:** Max 1000 entries, 50MB total
**TTL:** 5 minutes

---

### 2. Network Events (Waterfall)
```json
{
  "type": "network",
  "method": "POST|GET|PUT|DELETE|PATCH",
  "url": "https://api.example.com/checkout",
  "status": 200,
  "duration_ms": 234,
  "timestamp": 1707346801000,
  "request_size_bytes": 512,
  "response_size_bytes": 1024,
  "initiator": "fetch|xhr|img|script|link",
  "protocol": "http/1.1|h2|http3"
}
```

**Captured From:** fetch() and XMLHttpRequest
**Size Limit:** Max 10KB per entry
**Buffer Size:** Max 500 entries, 20MB total
**TTL:** 10 minutes

**Separate Storage (network_bodies):**
```json
{
  "url": "https://api.example.com/checkout",
  "request_headers": {...},
  "request_body": "{\"email\": \"...\"}" (first 1MB),
  "response_headers": {...},
  "response_body": "{\"order_id\": \"...\"}" (first 5MB)
}
```

**Body Storage:** Max 100 entries, 200MB total
**TTL:** 10 minutes

---

### 3. Action Events
```json
{
  "type": "action",
  "action_type": "click|input|change|navigation",
  "target": "button[type=submit]",
  "x": 100,
  "y": 50,
  "text": "Submit",
  "timestamp": 1707346802000,
  "selectors": {
    "css": "button[type=submit]",
    "xpath": "//button[@type='submit']",
    "text": "contains(., 'Submit')",
    "data_testid": "checkout-button"
  }
}
```

**Captured From:**
- click events with selector analysis
- input/change events with target data
- navigation with URL tracking

**Size Limit:** Max 2KB per entry
**Buffer Size:** Max 5000 entries, 30MB total
**TTL:** 10 minutes

---

### 4. WebSocket Events
```json
{
  "type": "websocket",
  "event": "open|message|close|error",
  "url": "wss://socket.example.com",
  "data_size": 512,
  "data_preview": "{\"type\": \"update\", ...}" (first 1KB),
  "timestamp": 1707346803000,
  "direction": "sent|received"
}
```

**Captured From:** WebSocket API
**Size Limit:** Max 1KB preview, 100KB full
**Buffer Size:** Max 500 entries, 50MB total
**TTL:** 10 minutes

---

### 5. Performance Events
```json
{
  "type": "performance",
  "metric": "LCP|FCP|CLS|TTFB",
  "value": 1234,
  "unit": "ms|number",
  "timestamp": 1707346804000,
  "context": {
    "element": "img#hero",
    "url": "https://cdn.example.com/image.webp",
    "loadTime": 1000
  }
}
```

**Captured From:**
- PerformanceObserver('largest-contentful-paint')
- PerformanceObserver('cumulative-layout-shift')
- PerformanceObserver('first-input')
- PerformanceResourceTiming

**Size Limit:** Max 1KB per entry
**Buffer Size:** Max 500 entries, 20MB total
**TTL:** 10 minutes

---

### 6. Error Events
```json
{
  "type": "error",
  "message": "Cannot read property 'foo' of undefined",
  "stack": "Error: ...\n  at checkout...",
  "source": "app.js",
  "line": 42,
  "column": 10,
  "timestamp": 1707346805000,
  "context": {
    "url": "https://example.com/checkout",
    "user_id": "redacted",
    "request_id": "abc123"
  },
  "enrichment": {
    "root_cause": "null_pointer_dereference",
    "affected_feature": "checkout_flow",
    "severity": "high"
  }
}
```

**Captured From:**
- window.onerror
- unhandledrejection
- ErrorEvent listeners

**Size Limit:** Stack max 5KB, context 2KB
**Buffer Size:** Max 200 entries, 20MB total
**TTL:** 15 minutes (longer for errors)

---

## Memory Accounting

Each buffer tracks:
```go
type BufferStats struct {
  EntryCount       int64
  OldestTimestamp  int64
  NewestTimestamp  int64
  EstimatedBytes   int64
  TTLExpiredCount  int64
  EvictedCount     int64
  LastClearedAt    int64
}
```

**Automatic Calculation:**
- Per-entry size estimation
- Running total per buffer
- Global total (cap at 500MB)
- Alert when > 80% full

**Enforcement:**
- Evict oldest entries first (FIFO)
- Respect TTL before size (prefer time over space)
- Hard limit: 500MB total, 200MB per buffer
- Graceful degradation: drop new entries if full

---

## References

### Implementation Files

**Page-Level Capture:**
- `src/inject/observers.ts` - All event observers setup
- `src/lib/page/console.ts` - Console capture
- `src/lib/net/network.ts` - Fetch/XHR capture
- `src/lib/net/websocket.ts` - WebSocket capture
- `src/lib/page/actions.ts` - Action capture
- `src/lib/analysis/performance.ts` - Performance capture
- `src/lib/page/exceptions.ts` - Error capture

**Serialization & Enrichment:**
- `src/lib/page/serialize.ts` - Safe JSON serialization
- `src/background/dom/dom-queries.ts` - Selector generation
- `src/lib/ai-context/ai-context.ts` - Error enrichment
- `internal/redaction/redaction.go` - PII masking

**Batching & Transport:**
- `src/background/sync/batchers.ts` - Event batching
- `src/background/sync/sync-client.ts` - HTTP POST /sync
- `src/background/state-manager.ts` - State storage

**Server-Side Storage:**
- `internal/capture/types.go` - Capture struct
- `internal/capture/websocket.go` - WS buffer
- `internal/capture/network_waterfall.go` - Waterfall buffer
- `internal/capture/network_bodies.go` - Body storage
- `internal/capture/enhanced_actions.go` - Actions buffer
- `internal/capture/extension_logs.go` - Logs buffer
- `internal/capture/handlers.go` - /sync endpoint
- `internal/capture/memory.go` - Memory tracking
- `internal/capture/ttl.go` - TTL eviction
- `internal/capture/buffer_clear.go` - Cleanup logic

### Related Diagrams
- [Extension Message Protocol](extension-message-protocol.md) - HTTP messages
- [Request-Response Cycle](request-response-cycle.md) - MCP flow
- [C2: Containers](c2-containers.md) - Component overview
- [Query System](query-system.md) - Query routing

### Documentation
- [Timestamp Standard](../../core/timestamp-standard.md) - Time format
- [CSP Execution Strategies](../../core/csp-execution-strategies.md) - Observer limitations
- [Error Recovery](../../core/error-recovery.md) - Error handling
