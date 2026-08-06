---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-03-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Kaboom System Architecture

## C4 Level 1: System Context

```mermaid
graph TB
    subgraph External["External Systems"]
        Dev["👤 Developer<br/><br/>Writes code, browses<br/>web app being developed"]
        AI["🤖 AI Coding Agent<br/><br/>Claude Code, Cursor<br/>Copilot, Windsurf"]
        Browser["🌐 Web Browser<br/><br/>Chrome or Brave<br/>Renders web application"]
    end

    subgraph Kaboom["⚙️ Kaboom MCP System"]
        Core["Browser Observability Platform<br/><br/>• Captures console logs, network traffic, WebSocket events<br/>• Provides async browser control via MCP<br/>• Zero dependencies, localhost-only"]
    end

    Dev -->|"Natural language<br/>debugging requests"| AI
    Dev -->|"Browses and tests<br/>web application"| Browser

    AI <-->|"MCP Protocol<br/>Query telemetry<br/>Control browser"| Core

    Browser <-->|"HTTP (localhost:7890)<br/>Stream events<br/>Poll commands"| Core

    Core -.->|"Visual feedback<br/>🔥 Flickering favicon<br/>when AI is in control"| Browser

    style Dev fill:#58a6ff,stroke:#1f6feb,stroke-width:3px,color:#fff
    style AI fill:#fde047,stroke:#d29922,stroke-width:3px
    style Browser fill:#3fb950,stroke:#2ea043,stroke-width:3px,color:#fff
    style Core fill:#fb923c,stroke:#f97316,stroke-width:4px,color:#fff
```

## C4 Level 2: Container Diagram

```mermaid
graph TB
    subgraph AI["AI Agent Process"]
        MCP["MCP Server<br/><br/>Go Binary<br/>Handles MCP tools,<br/>manages async queue"]
        Bridge["Bridge Process<br/><br/>Go Binary<br/>Forwards stdio ↔ HTTP"]
    end

    subgraph Server["Kaboom Server (localhost:7890)"]
        HTTP["HTTP Server<br/><br/>Go net/http<br/>Receives telemetry,<br/>serves queries"]

        Queue[("Async Queue<br/><br/>In-Memory<br/>Max 5 pending,<br/>30s timeout")]

        Results[("Query Results<br/><br/>In-Memory<br/>60s TTL")]

        Logs[("Event Buffers<br/><br/>Ring Buffers<br/>Logs, Network, WS")]
    end

    subgraph Browser["Browser Extension"]
        Ext["Extension Service Worker<br/><br/>JavaScript MV3<br/>Captures events,<br/>polls queries"]
    end

    MCP -->|"JSON-RPC<br/>(stdio)"| Bridge
    Bridge -->|"HTTP POST<br/>(localhost:7890)"| HTTP

    HTTP -->|"Enqueue<br/>CreatePendingQuery()"| Queue
    HTTP -->|"Store<br/>SetQueryResult()"| Results
    HTTP -->|"Buffer<br/>Ring buffer append"| Logs

    Ext -->|"Poll every 1s<br/>GET /pending-queries"| HTTP
    Ext -->|"Post results<br/>POST /dom-result"| HTTP
    Ext -->|"Post events<br/>POST /logs, /network-body"| HTTP

    Queue -.->|"Poll response"| Ext
    Results -.->|"observe() returns"| MCP
    Logs -.->|"observe() returns"| MCP

    style MCP fill:#fde047,stroke:#d29922,stroke-width:3px
    style Bridge fill:#fbbf24,stroke:#d29922,stroke-width:2px
    style HTTP fill:#fb923c,stroke:#f97316,stroke-width:3px,color:#fff
    style Ext fill:#3fb950,stroke:#2ea043,stroke-width:3px,color:#fff
    style Queue fill:#58a6ff,stroke:#1f6feb,stroke-width:2px,color:#fff
    style Results fill:#58a6ff,stroke:#1f6feb,stroke-width:2px,color:#fff
    style Logs fill:#58a6ff,stroke:#1f6feb,stroke-width:2px,color:#fff
```

## Data Flow - Event Capture

```mermaid
flowchart TD
    subgraph Browser["Browser Tab"]
        Page[Web Page<br/>Being Debugged]
        Console[console.log]
        Network[fetch/XHR]
        WS[WebSocket]
    end

    subgraph Extension["Kaboom Extension"]
        Inject[inject.js<br/>Injected into page]
        Content[content.js<br/>Content script]
        Background[background.js<br/>Service worker]
    end

    subgraph Server["HTTP Server :7890"]
        Logs[POST /logs<br/>Ring buffer: 1000]
        Bodies[POST /network-body<br/>Ring buffer: 100]
        WSEvents[POST /ws-event<br/>Ring buffer: 500]
    end

    subgraph MCP["MCP Tools"]
        Observe[observe tool<br/>Returns captured data]
    end

    Console --> Inject
    Network --> Inject
    WS --> Inject

    Inject -->|window.postMessage| Content
    Content -->|chrome.runtime.sendMessage| Background

    Background -->|Batch + POST| Logs
    Background -->|Batch + POST| Bodies
    Background -->|Batch + POST| WSEvents

    Observe -->|GET /logs, etc.| Logs
    Observe -->|GET /network-bodies| Bodies
    Observe -->|GET /ws-events| WSEvents

    style Inject fill:#fde047
    style Background fill:#fb923c
    style Observe fill:#3fb950
```

## Data Flow - Browser Control

```mermaid
flowchart LR
    subgraph AI["AI Agent"]
        Interact[interact tool<br/>execute_js, navigate, etc.]
    end

    subgraph MCP["MCP Server"]
        Tools[Tool Handler]
        Queue[Async Queue<br/>Max 5, 30s TTL]
        Tracker[Correlation Tracker]
    end

    subgraph Extension["Extension"]
        Poll[Query Poller<br/>Every 1s]
        Exec[Script Executor]
    end

    subgraph Browser["Browser"]
        DOM[DOM/JavaScript]
    end

    Interact -->|1. Command| Tools
    Tools -->|2. Queue| Queue
    Tools -->|3. Track| Tracker
    Tools -->|4. Return| CorrelID[correlation_id]

    Queue <-->|5. Poll| Poll
    Poll -->|6. Execute| Exec
    Exec -->|7. Run| DOM
    DOM -->|8. Result| Exec
    Exec -->|9. POST| Queue

    Queue -->|10. Complete| Tracker

    CorrelID -.->|11. Check status| Observe[observe tool]
    Observe -->|12. Query| Tracker
    Tracker -->|13. Status| AI

    style Queue fill:#fb923c
    style Tracker fill:#fde047
    style CorrelID fill:#3fb950
```

## Process Architecture

```mermaid
graph TB
    subgraph "MCP Client (AI Agent)"
        Claude[Claude Code<br/>or Cursor/Copilot/Windsurf]
    end

    subgraph "Kaboom Processes"
        Bridge[Bridge Process<br/>PID 12345<br/>stdio ↔ HTTP forwarder]
        Server[HTTP Server<br/>PID 12346<br/>Persistent daemon on :7890]
    end

    subgraph "Browser"
        Ext[Extension Service Worker<br/>MV3 background.js]
        Content[Content Scripts<br/>Per tracked tab]
    end

    Claude -->|stdin/stdout<br/>JSON-RPC| Bridge
    Bridge -->|HTTP POST<br/>localhost:7890| Server

    Server <-->|HTTP GET/POST<br/>localhost:7890| Ext
    Ext <-->|chrome.tabs API| Content
    Content <-->|DOM manipulation| Page[Web Page]

    style Server fill:#fb923c
    style Bridge fill:#fde047
    style Ext fill:#3fb950
```

## Network Topology

```mermaid
graph LR
    subgraph "Localhost Only"
        AI[AI Agent<br/>127.0.0.1:random]
        MCP[MCP Bridge<br/>127.0.0.1:random]
        HTTP[HTTP Server<br/>127.0.0.1:7890]
        Ext[Extension<br/>chrome-extension://...]
    end

    AI -->|stdio| MCP
    MCP <-->|HTTP| HTTP
    HTTP <-->|HTTP| Ext

    Internet[Internet] -.->|❌ Never| HTTP
    Internet -.->|❌ Never| MCP

    style Internet fill:#f85149
    style HTTP fill:#3fb950
    style MCP fill:#3fb950
```

## Security Model

```mermaid
graph TD
    subgraph "Trust Boundaries"
        TB1[Browser Sandbox]
        TB2[Extension Permissions]
        TB3[Localhost Firewall]
    end

    subgraph "Data Flow"
        Page[User's Web Page] -->|Observed by| Inject[inject.js]
        Inject -->|postMessage| Content[content.js]
        Content -->|Validated| Background[background.js]
        Background -->|localhost POST| Server[HTTP Server]
    end

    Server -->|"Customer controlled<br/>--log-file flag"| LocalDisk["💾 Local Disk<br/><br/>~/kaboom-logs.jsonl<br/>Customer's machine"]

    Server -.->|"❌ Never reaches"| Cloud["☁️ Kaboom Cloud<br/><br/>No SaaS service<br/>No external APIs<br/>No telemetry"]

    Server -.->|"🔮 Future: Optional"| DistStorage["🗄️ Distributed Storage<br/><br/>Customer's own infrastructure<br/>(S3, Postgres, etc.)<br/>Customer controls access"]

    TB1 --> Page
    TB2 --> Background
    TB3 --> Server

    style Cloud fill:#f85149,color:#fff,stroke:#d73a49,stroke-width:3px
    style LocalDisk fill:#3fb950,color:#fff,stroke:#2ea043,stroke-width:3px
    style DistStorage fill:#58a6ff,color:#fff,stroke:#1f6feb,stroke-width:2px
    style Server fill:#fb923c,color:#fff

    note1["Key Security Points:<br/>✅ All data stays on customer's infrastructure<br/>✅ Customer chooses storage location<br/>✅ Zero external dependencies<br/>❌ Never sent to Kaboom's servers"]
    Server -.-> note1
```

## Deployment Model

```mermaid
graph TB
    subgraph "Developer Machine"
        subgraph "Installation"
            NPM[npm install<br/>kaboom-mcp@5.4.0]
            Download[Downloads platform binary<br/>~7MB arm64/x64]
            Place[Places in ~/.npm/_npx/]
        end

        subgraph "Extension"
            Store[Chrome Web Store<br/>OR GitHub Release]
            Load[Load unpacked<br/>in chrome://extensions]
        end

        subgraph "Runtime"
            Start[npx kaboom-mcp<br/>--port 7890]
            Fork[Spawns HTTP server daemon]
            MCPConfig[MCP config launches<br/>bridge on demand]
        end
    end

    NPM --> Download --> Place
    Store --> Load

    Start --> Fork
    MCPConfig --> Bridge[Bridge connects<br/>to :7890]

    style Fork fill:#3fb950
    style Bridge fill:#fde047
```

## Scaling Characteristics

```mermaid
graph LR
    subgraph "Single Developer"
        S1[1 AI agent]
        S2[1 browser tab tracked]
        S3[~10 events/sec]
        S4[Memory: <50MB]
        S5[CPU: <1%]
    end

    subgraph "Power User"
        P1[3 AI agents<br/>multi-client]
        P2[5 tabs monitored]
        P3[~100 events/sec]
        P4[Memory: <200MB]
        P5[CPU: <5%]
    end

    subgraph "Limits"
        L1[Max events: 1000/sec<br/>Circuit breaker]
        L2[Max queue: 5 commands<br/>FIFO eviction]
        L3[Max memory: 500MB<br/>Hard limit]
        L4[Max connections: 20<br/>LRU eviction]
    end

    S1 --> S4
    P1 --> P4

    P3 -.->|Approaches| L1
    P4 -.->|Well below| L3

    style L1 fill:#fde047
    style L3 fill:#fde047
    style S4 fill:#3fb950
    style P4 fill:#3fb950
```

## Technology Stack

```mermaid
graph TB
    subgraph "Backend (Go)"
        G1[Go 1.21+<br/>Zero dependencies]
        G2[net/http<br/>Standard library only]
        G3[JSON-RPC 2.0<br/>MCP protocol]
    end

    subgraph "Frontend (Extension)"
        E1[Chrome Extension MV3<br/>Service Worker]
        E2[TypeScript → JavaScript<br/>No bundler dependencies]
        E3[SVG icons<br/>8-frame animation]
    end

    subgraph "Distribution"
        D1[NPM packages<br/>Platform binaries]
        D2[GitHub Releases<br/>Extension zip]
        D3[Chrome Web Store<br/>Future]
    end

    G1 & G2 & G3 --> Build[make build]
    E1 & E2 & E3 --> Compile[npx tsc]

    Build --> D1
    Compile --> D2
    D2 -.-> D3

    style Build fill:#fb923c
    style Compile fill:#fde047
    style D1 fill:#3fb950
    style D2 fill:#3fb950
```

## References

- [Async Queue Flow](async-queue-flow.md)
- [Correlation ID Lifecycle](correlation-id-lifecycle.md)
- [5-Layer Protection](quality/5-layer-protection.md)
- [Flame Flicker Visual](ui/flame-flicker-visual.md)
- [ADR-001: Async Queue Pattern](../ADR-001-async-queue-pattern.md)
