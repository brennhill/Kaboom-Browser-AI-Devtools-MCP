---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-03-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# C3: Component Architecture (Go Packages)

## Overview

The Go server is organized into 5 logical layers: Foundation Types, Domain Packages, Tools, HTTP Server, and Utilities.

## C3 Architecture Diagram

```mermaid
graph TB
    subgraph Foundation["🔹 Layer 1: Foundation Types (internal/types/)"]
        direction LR
        Protocol["protocol.go<br/>JSON-RPC types<br/>- Request/Response<br/>- Error handling"]
        Network["network.go<br/>Network telemetry<br/>- HTTP timing<br/>- Body capture"]
        LogT["log.go<br/>Log types<br/>- Server logs<br/>- Extension logs"]
        Alert["alert.go<br/>Alert types"]
        Security["security.go<br/>Security types"]
        Snapshot["snapshot.go<br/>State snapshot"]
        Buffer["buffer.go<br/>Buffer types<br/>- Cursors<br/>- Metadata"]

        Protocol ~~~ Network ~~~ LogT ~~~ Alert ~~~ Security ~~~ Snapshot ~~~ Buffer
    end

    subgraph Domain["🟠 Layer 2: Domain Packages"]
        direction TB

        subgraph Capture["**Capture** (internal/capture/)<br/>Telemetry buffering"]
            CTypes["types.go<br/>Capture struct"]
            WS["websocket.go<br/>WS events"]
            NetBody["network_bodies.go<br/>HTTP bodies"]
            NetWater["network_waterfall.go<br/>HTTP timing"]
            Actions["enhanced_actions.go<br/>User actions"]
            Logs["extension_logs.go<br/>Console logs"]
            Recording["recording_manager.go<br/>Recording lifecycle"]
            Handlers["handlers.go<br/>HTTP endpoints"]
            Queries["queries.go<br/>Pending query queue"]
            Dispatcher["query_dispatcher.go<br/>Query routing"]
            Sync["sync.go<br/>/sync endpoint"]
            Memory["memory.go<br/>Memory tracking"]
            TTL["ttl.go<br/>TTL eviction"]
            Settings["settings.go<br/>Capture settings"]

            CTypes --> WS
            CTypes --> NetBody
            CTypes --> NetWater
            CTypes --> Actions
            CTypes --> Logs
            CTypes --> Recording
            Handlers --> Queries
            Queries --> Dispatcher
            Dispatcher --> Sync
            CTypes --> Memory
            CTypes --> TTL
            CTypes --> Settings
        end

        subgraph Session["**Session** (internal/session/)<br/>Client management"]
            STypes["types.go"]
            Registry["client_registry.go<br/>Multi-client isolation"]
            Verify["verify.go<br/>Token verification"]
            Sessions["sessions.go<br/>Session lifecycle"]
            Verify --> Sessions
            Sessions --> Registry
        end

        subgraph Analysis["**Analysis** (internal/analysis/)<br/>Data intelligence"]
            Clustering["clustering.go<br/>Error clustering<br/>2-of-3 signal matching"]
            APIContract["api_contract.go<br/>API schema inference"]
            ThirdParty["thirdparty.go<br/>3rd-party detection"]
            Clustering -.-> APIContract
        end

        subgraph Security["**Security** (internal/security/)<br/>Threat detection"]
            SecurityCore["security.go<br/>Core analysis"]
            CSP["csp.go<br/>CSP violation detection"]
            SRI["sri.go<br/>SRI validation"]
            Flagging["security_flagging.go<br/>Threat flagging"]
            SecurityCore --> CSP
            SecurityCore --> SRI
            SecurityCore --> Flagging
        end

        subgraph Pagination["**Pagination** (internal/pagination/)<br/>Cursor-based pagination"]
            Cursor["cursor.go<br/>Encoding/decoding"]
            PaginationL["pagination.go<br/>RFC-compliant"]
            Cursor --> PaginationL
        end

        subgraph Buffers["**Buffers** (internal/buffers/)<br/>Data structures"]
            RingBuf["ring_buffer.go<br/>FIFO circular buffer<br/>Cursor-based reads"]
        end

        Capture --> Session
        Capture --> Analysis
        Capture --> Security
        Analysis --> Pagination
        Capture --> Buffers
    end

    subgraph Tools["🔧 Layer 3: MCP Tools (cmd/browser-agent/tools_*.go)"]
        direction LR
        Observe["tools_observe.go<br/>observe(what)"]
        Generate["internal/toolgenerate/dispatcher.go<br/>generate(format)"]
        Interact["tools_interact.go<br/>interact(action)"]
        Configure["tools_configure.go<br/>configure(action)"]

        Observe ~~~ Generate ~~~ Interact ~~~ Configure

        Observe --> Capture
        Generate --> Capture
        Interact --> Capture
        Configure --> Session

        Observe --> Analysis
        Generate --> Analysis
        Generate --> Security
    end

    subgraph HTTPServer["🌐 Layer 4: HTTP Server (cmd/browser-agent/)"]
        direction LR
        Main["main.go<br/>Entry point<br/>Lifecycle"]
        Handler["handler.go<br/>JSON-RPC routing"]
        Server["server.go<br/>HTTP setup<br/>Log storage"]
        Middleware["server_middleware.go<br/>CORS, auth<br/>Origin validation"]
        Routes["server.go<br/>REST endpoints"]
        Health["health.go<br/>Diagnostics"]

        Main --> Handler
        Main --> Server
        Handler --> Routes
        Server --> Middleware
        Routes --> Middleware
        Routes --> Health

        Handler --> Tools
        Routes --> Capture
        Routes --> Session
    end

    subgraph Utilities["🔧 Layer 5: Utilities"]
        direction LR
        RateLimit["rate_limit.go<br/>Rate limiting"]
        CircuitBreaker["circuit_breaker.go<br/>Resilience"]
        Redaction["redaction.go<br/>PII masking"]
        Export["export_sarif.go<br/>SARIF report"]
        Perf["performance/<br/>Metrics & diff"]
        Util["util/<br/>Binary detect<br/>Process mgmt"]

        RateLimit ~~~ CircuitBreaker ~~~ Redaction ~~~ Export ~~~ Perf ~~~ Util

        Tools --> RateLimit
        Capture --> CircuitBreaker
        Generate --> Export
        Tools --> Redaction
    end

    Foundation -.-> Domain
    Domain -.-> Tools
    Tools -.-> HTTPServer
    Domain -.-> HTTPServer
    HTTPServer -.-> Utilities

    %% Styling
    classDef layer1 fill:#3fb950,stroke:#2ea043,stroke-width:2px,color:#fff
    classDef layer2 fill:#fb923c,stroke:#f97316,stroke-width:2px,color:#fff
    classDef layer3 fill:#fde047,stroke:#d29922,stroke-width:2px,color:#000
    classDef layer4 fill:#58a6ff,stroke:#1f6feb,stroke-width:2px,color:#fff
    classDef layer5 fill:#a371f7,stroke:#8957e5,stroke-width:2px,color:#fff

    class Foundation layer1
    class Capture,Session,Analysis,Security,Pagination,Buffers layer2
    class Observe,Generate,Interact,Configure layer3
    class Main,Handler,Server,Middleware,Routes,Health layer4
    class RateLimit,CircuitBreaker,Redaction,Export,Perf,Util layer5
```

---

## Layer Responsibilities

### Layer 1: Foundation Types (`internal/types/`)
**Purpose:** Zero-dependency type definitions for all layers
- **protocol.go** - JSON-RPC 2.0 message types
- **network.go** - Network telemetry (PerformanceResourceTiming, HTTP timing)
- **log.go** - Log entry types (server logs, extension logs)
- **alert.go** - Alert types (immediate, CI, perf regression)
- **security.go** - Security threat flags
- **snapshot.go** - State snapshot types
- **buffer.go** - Buffer cursors and metadata

**Dependencies:** None
**Used By:** All other layers

### Layer 2: Domain Packages

#### **Capture** (`internal/capture/`)
**Purpose:** Receive and buffer browser telemetry
- **Ring Buffers:** logs, network_bodies, network_waterfall, websocket_events, actions
- **Memory Enforcement:** TTL eviction, max size limits
- **Query Queue:** Pending queries from AI, dispatched to extension
- **Sync Endpoint:** Batched telemetry ingestion from extension
- **Recording:** Video/audio lifecycle management

**Key Files:**
- `types.go` - Capture struct (central hub)
- `websocket.go`, `network_*.go`, `enhanced_actions.go`, `extension_logs.go` - Event-specific buffers
- `queries.go`, `query_dispatcher.go` - Query queueing
- `sync.go` - Batched telemetry reception
- `recording_manager.go` - Recording persistence
- `handlers.go` - HTTP endpoint handlers
- `memory.go`, `ttl.go` - Resource management

**Responsible For:**
- Receiving telemetry from extension
- Buffering with circular FIFO semantics
- TTL-based eviction
- Memory limit enforcement
- Query queueing and routing
- Correlation ID tracking for results

#### **Session** (`internal/session/`)
**Purpose:** Multi-client isolation and verification
- **Client Registry** - Track connected clients
- **Token Verification** - Constant-time token comparison
- **Session Lifecycle** - Create, maintain, clean up
- **Rate Limiting** - Per-client limits
- **Circuit Breaker** - Detect misbehaving clients

**Responsible For:**
- Isolating clients from each other
- Preventing token replay
- Enforcing rate limits per client
- Managing session timeouts

#### **Analysis** (`internal/analysis/`)
**Purpose:** Derive insights from captured data
- **Error Clustering** - Group errors by root cause (2-of-3 signal matching)
- **API Contract Analysis** - Infer API schemas from network traffic
- **Third-Party Detection** - Identify 3rd-party resources

**Used By:**
- `generate` tool (for test generation, error summaries)
- Security analysis (for threat context)

#### **Security** (`internal/security/`)
**Purpose:** Threat detection and compliance. Split into focused subpackages:
- **`scan/`** - Aggregate audit: credentials, PII, headers, cookies, transport, auth
- **`csp/`** - Detect CSP violations, suggest policies from observed traffic
- **`sri/`** - Generate Subresource Integrity hashes
- **`netflag/`** - Flag suspicious origins (abusive TLDs, typosquatting, mixed content)
- **`diff/`** - Compare security posture over time via named snapshots
- **`policy/`** - MCP-mode trust boundary and the manual-only config guards
- **`httpsec/`** - Shared URL/cookie primitives (leaf, no sibling imports)

**Used By:**
- `generate` tool (for SARIF reports, CSP recommendations)
- Alerts (for security notifications)

#### **Pagination** (`internal/pagination/`)
**Purpose:** RFC-compliant cursor-based pagination
- **Cursor Encoding** - timestamp:sequence format
- **Eviction Handling** - Automatic restart when cursor expires
- **Pagination Logic** - Offset, limit, has_more

**Used By:** All observe queries with pagination

#### **Buffers** (`internal/buffers/`)
**Purpose:** Generic ring buffer data structure
- **Ring Buffer** - Fixed-size circular FIFO
- **Cursor-based Reads** - Safe concurrent reads
- **Timestamp Tracking** - Creation time metadata

**Used By:** Capture (all event buffers)

---

### Layer 3: MCP Tools (`cmd/browser-agent/tools_*.go`)

**4 Tool Implementations:**

| Tool | File | Purpose | Uses |
|------|------|---------|------|
| **observe** | tools_observe.go | Query buffered telemetry | Capture, Analysis, Security |
| **generate** | internal/toolgenerate/dispatcher.go | Generate artifacts (tests, SARIF, HAR, CSP) | Capture, Analysis, Security |
| **interact** | tools_interact.go | DOM queries, browser control | Capture, Session |
| **configure** | tools_configure.go | Noise rules, state persistence | Session, Capture |

**Orchestration:** `tools_core.go` provides common utilities (rate limiting, response formatting, error handling)

---

### Layer 4: HTTP Server (`cmd/browser-agent/`)

| Component | Purpose |
|-----------|---------|
| **main.go** | Entry point, argument parsing, lifecycle management |
| **handler.go** | JSON-RPC 2.0 request parsing and routing |
| **server.go** | HTTP server setup, log rotation, file persistence |
| **server_middleware.go** | CORS, authentication, origin validation |
| **server.go** | REST endpoint registration |
| **health.go** | Health checks, diagnostics |

**HTTP Endpoints:**
- `POST /mcp` - JSON-RPC 2.0 requests
- `GET /health` - Server health check
- `GET /pending-queries` - Extension polls for work
- `POST /dom-result` - Extension posts DOM query results
- `POST /sync` - Extension posts telemetry batch
- `POST /recordings/save` - Extension posts recording video

---

### Layer 5: Utilities

| Package | Purpose |
|---------|---------|
| **rate_limit.go** | Per-client rate limiting |
| **circuit_breaker.go** | Detect and isolate misbehaving clients |
| **redaction.go** | PII and sensitive data masking |
| **export_sarif.go** | SARIF security report generation |
| **performance/** | Metrics calculation and diff |
| **util/** | Binary format detection, process management |

---

## Dependency Flow

```
Foundation Types
    ↓
Capture ← → Session ← → Analysis ← → Security
    ↓            ↓          ↓           ↓
Pagination ← Buffers   (Intelligence)
    ↓
Tools (observe, generate, interact, configure)
    ↓
HTTP Server (handler, routes, middleware)
    ↓
Utilities (rate limit, circuit breaker, etc.)
```

**Key Principle:** Lower layers don't depend on upper layers. All dependencies flow downward.

---

## Interaction Patterns

### Pattern 1: Observe Tool
```
AI calls observe({what: 'logs'})
  → handler.go routes to tools_observe.go
  → tools_observe.go queries Capture.logs buffer
  → Pagination adds cursor handling
  → Analysis enriches with context (if needed)
  → Security filters (redaction if needed)
  → Response returned to AI
```

### Pattern 2: Interact Tool (Query)
```
AI calls interact({action: 'execute_js'})
  → handler.go routes to tools_interact.go
  → tools_interact.go creates PendingQuery in Capture.queries
  → Query assigned correlation_id
  → Response returned with correlation_id (non-blocking)
  → Extension polls Capture.queries every 1s
  → Extension executes, posts result to /dom-result
  → Capture.completedResults stores result
  → AI polls for result using correlation_id
  → Session verifies token, returns result
  → Redaction filters sensitive data
```

### Pattern 3: Generate Tool
```
AI calls generate({format: 'test'})
  → handler.go routes to internal/toolgenerate/dispatcher.go
  → internal/toolgenerate/dispatcher.go queries Capture buffers
  → Analysis clusters errors, infers API schemas
  → testgen.go classifies and heals selectors
  → Pagination handles large datasets
  → Export formats as Playwright test
  → Security removes threats from output
  → Response returned to AI
```

### Pattern 4: Configure Tool
```
AI calls configure({action: 'store', data: {...}})
  → handler.go routes to tools_configure.go
  → tools_configure.go persists to disk
  → No query to extension needed
  → Response immediately returned
```

---

## References

### Implementation Files
- **Foundation:** `internal/types/*.go`
- **Capture:** `internal/capture/*.go` (29 files)
- **Session:** `internal/session/*.go` (6 files)
- **Analysis:** `internal/analysis/{apicontract,apischema,clustering,thirdparty}/*.go` (24 files)
- **Security:** `internal/security/{scan,csp,sri,diff,policy,netflag,httpsec}/*.go` (38 files)
- **Tools:** `cmd/browser-agent/tools_*.go` (8 files)
- **HTTP Server:** `cmd/browser-agent/{main,handler,server,middleware,routes,health}.go`
- **Utilities:** `internal/{rate_limit,circuit_breaker,redaction,export}` + `internal/util/`

### Related Diagrams
- [C2: Containers](c2-containers.md) - Container boundaries
- [Request-Response Cycle](request-response-cycle.md) - MCP command flow
- [Data Capture Pipeline](data-capture-pipeline.md) - Telemetry flow
- [Query System](query-system.md) - Query routing details

### Documentation
- [Codebase Index](../../core/code-index.md)
- [Feature Strategy](../../core/feature-to-strategy.md)
- [MCP Correctness](../../core/mcp-correctness.md)
