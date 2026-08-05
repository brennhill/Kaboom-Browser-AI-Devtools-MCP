---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-03-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Async Queue-and-Poll Architecture

## Overview Flow

```mermaid
sequenceDiagram
    participant AI as AI Agent (MCP Client)
    participant MCP as MCP Server<br/>(Go Binary)
    participant Queue as Pending Query Queue<br/>(Max 5, 30s TTL)
    participant Ext as Browser Extension<br/>(Polls every 1s)
    participant Browser as Browser Tab

    AI->>MCP: interact({what: 'execute_js', script: '...'})
    Note over MCP: Generate correlation_id<br/>exec_12345_67890

    MCP->>Queue: CreatePendingQuery(query, 30s timeout)
    Note over Queue: Store: {id: "q-42",<br/>correlation_id: "exec_12345_67890"}

    MCP-->>AI: {status: "queued", correlation_id: "exec_12345_67890"}
    Note over AI: Returns IMMEDIATELY<br/>(non-blocking)

    loop Every 1 second
        Ext->>Queue: GET /pending-queries
        Queue-->>Ext: [{id: "q-42", type: "execute", ...}]
    end

    Ext->>Browser: executeScript(script)
    Browser-->>Ext: {result: {...}}

    Ext->>MCP: POST /dom-result {id: "q-42", result: {...}}
    MCP->>Queue: SetQueryResult(id, result)
    Note over Queue: Move to queryResults<br/>Remove from pending<br/>Mark correlation complete

    AI->>MCP: observe({what: 'command_result',<br/>correlation_id: 'exec_12345_67890'})
    MCP->>Queue: GetCommandResult(correlation_id)
    MCP-->>AI: {status: "complete", result: {...}}
```

## Timeout Handling

```mermaid
sequenceDiagram
    participant MCP as MCP Server
    participant Queue as Query Queue
    participant Cleanup as Cleanup Goroutine
    participant Ext as Extension

    MCP->>Queue: CreatePendingQuery(query, 30s)
    MCP->>Cleanup: Schedule expiration (30s)

    Note over Ext,Queue: Extension never polls<br/>(crashed, disconnected, or slow)

    Cleanup->>Cleanup: Sleep 30 seconds
    Cleanup->>Queue: CleanExpiredQueries()
    Note over Queue: Remove from pendingQueries<br/>Mark as "expired"<br/>Move to failedCommands

    Note over MCP: AI can still check status
    MCP->>Queue: GetCommandResult(correlation_id)
    Queue-->>MCP: {status: "expired", error: "..."}
```

## Queue States

```mermaid
stateDiagram-v2
    [*] --> Pending: CreatePendingQuery()

    Pending --> Complete: Extension posts result
    Pending --> Expired: 30s timeout

    Complete --> [*]: 60s TTL cleanup
    Expired --> [*]: Moved to failedCommands<br/>(ring buffer, max 100)

    note right of Pending
        In pendingQueries array
        Max 5 commands (FIFO)
        Polled by extension every 1s
    end note

    note right of Complete
        In queryResults map
        In completedResults map
        60s TTL before cleanup
    end note

    note right of Expired
        In failedCommands ring buffer
        AI can still query status
        Eventually evicted (max 100)
    end note
```

## Multi-Client Isolation

```mermaid
graph TB
    subgraph "MCP Server (localhost:7890)"
        Queue[Pending Query Queue<br/>Max 5, FIFO]
        Results[Query Results<br/>60s TTL]
        Correlation[Correlation Tracking<br/>Global, not isolated]
    end

    subgraph "Client A (Claude Code)"
        MCP_A[MCP Connection A<br/>clientID: agent-a]
    end

    subgraph "Client B (Cursor)"
        MCP_B[MCP Connection B<br/>clientID: agent-b]
    end

    subgraph "Extension (Single Instance)"
        Poll[Polls /pending-queries<br/>X-Kaboom-Client header]
    end

    MCP_A -->|CreatePendingQuery<br/>clientID: agent-a| Queue
    MCP_B -->|CreatePendingQuery<br/>clientID: agent-b| Queue

    Queue -->|GetPendingQueriesForClient<br/>Filter by clientID| Poll

    Poll -->|POST /dom-result<br/>clientID in body| Results

    Results -->|TakeQueryResult<br/>Check clientID match| MCP_A
    Results -->|TakeQueryResult<br/>Check clientID match| MCP_B

    Correlation -.->|Global lookup<br/>No client isolation| MCP_A
    Correlation -.->|Global lookup<br/>No client isolation| MCP_B

    style Correlation fill:#fde047
    style Queue fill:#fb923c
    style Results fill:#3fb950
```

## Performance Characteristics

| Metric | Value | Notes |
|--------|-------|-------|
| **Latency** | <1ms | Localhost-only, no network |
| **Throughput** | ~1000 commands/sec | Limited by extension poll rate |
| **Queue Limit** | 5 pending | FIFO eviction prevents memory leaks |
| **Timeout** | 30s | 15x extension poll interval (safety margin) |
| **Memory** | ~200 bytes/command | Negligible for typical usage |
| **Reliability** | 100%* | *Given extension is polling |

## Failure Modes

```mermaid
graph LR
    Start[AI sends command] --> Queue{Queue full?}
    Queue -->|Yes| Drop[Drop oldest<br/>FIFO eviction]
    Queue -->|No| Add[Add to queue]

    Add --> Poll{Extension polls?}
    Poll -->|Yes, within 30s| Execute[Execute in browser]
    Poll -->|No, timeout| Expire[Mark as expired]

    Execute --> Result{Result posted?}
    Result -->|Yes| Complete[Status: complete]
    Result -->|No| Timeout[Network error]

    Drop --> FailedCommands
    Expire --> FailedCommands[Failed Commands<br/>Ring buffer, max 100]
    Complete --> Success[AI retrieves result]
    Timeout --> FailedCommands

    style Success fill:#3fb950
    style FailedCommands fill:#f85149
    style Complete fill:#fde047
```

## Key Design Decisions

### Why Queue-and-Poll?

**Alternative 1: Request/Response (Rejected)**
- ❌ Blocks MCP thread waiting for extension
- ❌ Timeout causes MCP hang
- ❌ No way to cancel or check status

**Alternative 2: WebSocket Push (Rejected)**
- ❌ Extension can't initiate connections to localhost
- ❌ Requires server to track extension state
- ❌ Complex reconnection logic

**Chosen: Queue-and-Poll** ✅
- ✅ MCP never blocks (returns immediately)
- ✅ Extension polls at its own pace
- ✅ Graceful degradation (extension offline = queue fills, eventually evicts)
- ✅ Simple reconnection (just resume polling)
- ✅ Correlation ID allows async status tracking

### Why 30 Seconds?

Extension polls every **1 second** in ideal conditions.

**Observed timing jitter:**
- Extension reload: ~2-5s gap
- Browser tab switch: ~0.5-2s delay
- Network hiccup: ~1-3s delay
- Extension crash → restart: ~5-10s

**30 seconds provides:**
- ~30 polling opportunities
- Buffer for extension restart
- Buffer for browser resource constraints
- Still feels responsive to users

**Rejected alternatives:**
- 2s (old value): ❌ 50%+ timeouts in production
- 5s: ❌ Still too tight with jitter
- 10s: ❌ Marginal improvement
- 60s: ❌ Unnecessarily long, masks real issues

## References

- [queries.go](../../internal/capture/queries.go) - Queue implementation
- [handlers.go](../../internal/capture/httpingest/handlers.go) - HTTP endpoints
- [tools_core.go](../../cmd/browser-agent/tools_core.go) - MCP tool handlers
- [ADR-001: Async Queue Pattern](../ADR-001-async-queue-pattern.md)
- [ADR-002: Async Queue Immutability](../ADR-002-async-queue-immutability.md)
