---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-03-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Correlation ID Lifecycle

## Command Status Tracking

```mermaid
stateDiagram-v2
    [*] --> Pending: RegisterCommand()<br/>when query created

    Pending --> Complete: CompleteCommand()<br/>when extension posts result
    Pending --> Expired: ExpireCommand()<br/>after 30s timeout

    Complete --> Cleaned: 60s TTL cleanup
    Expired --> Cleaned: Evicted from ring buffer<br/>(max 100 failed commands)

    Cleaned --> [*]

    note right of Pending
        Location: completedResults map
        Status: "pending"
        Created timestamp stored
        AI can poll anytime
    end note

    note right of Complete
        Location: completedResults map
        Status: "complete"
        Result + timestamps stored
        Available for 60s
    end note

    note right of Expired
        Location: failedCommands ring buffer
        Status: "expired"
        Error message stored
        Eventually evicted (FIFO)
    end note
```

## AI Polling Pattern

```mermaid
sequenceDiagram
    participant AI as AI Agent
    participant MCP as MCP Server
    participant Tracker as Correlation Tracker

    AI->>MCP: interact({what: 'execute_js', ...})
    MCP-->>AI: {status: "queued", correlation_id: "exec_123"}

    Note over AI: Wait 1-2 seconds<br/>(extension poll + execute time)

    loop Every 2 seconds until complete
        AI->>MCP: observe({what: 'command_result',<br/>correlation_id: 'exec_123'})
        MCP->>Tracker: GetCommandResult(correlation_id)

        alt Command still pending
            Tracker-->>MCP: {status: "pending", created_at: "..."}
            MCP-->>AI: Keep waiting...
        else Command complete
            Tracker-->>MCP: {status: "complete", result: {...}}
            MCP-->>AI: Success! ✅
        else Command expired
            Tracker-->>MCP: {status: "expired", error: "..."}
            MCP-->>AI: Timeout - retry or abort ❌
        end
    end
```

## Data Structures

```mermaid
classDiagram
    class CommandResult {
        +string CorrelationID
        +string Status
        +json.RawMessage Result
        +string Error
        +time.Time CompletedAt
        +time.Time CreatedAt
    }

    class Capture {
        +map completedResults
        +[]CommandResult failedCommands
        +sync.RWMutex resultsMu
        +RegisterCommand()
        +CompleteCommand()
        +ExpireCommand()
        +GetCommandResult()
        +GetPendingCommands()
        +GetCompletedCommands()
        +GetFailedCommands()
    }

    Capture --> CommandResult : stores

    note for CommandResult "Status values:\n- pending\n- complete\n- expired\n- timeout"
    note for Capture "Separate lock (resultsMu)\nfrom main capture mu\nto avoid blocking"
```

## Status Transition Matrix

| From | To | Trigger | Action |
|------|----|---------|-
| — | `pending` | CreatePendingQueryWithTimeout() | RegisterCommand() called |
| `pending` | `complete` | Extension posts result | CompleteCommand() called |
| `pending` | `expired` | 30s timeout | ExpireCommand() called, move to failedCommands |
| `complete` | [deleted] | 60s TTL | Background cleanup removes from completedResults |
| `expired` | [evicted] | Ring buffer full | Oldest entry removed from failedCommands (FIFO) |

## Lock Hierarchy

```mermaid
graph TD
    A[Main Lock: c.mu] -->|protects| B[pendingQueries]
    A -->|protects| C[queryResults]

    D[Results Lock: c.resultsMu] -->|protects| E[completedResults]
    D -->|protects| F[failedCommands]

    A -.->|never held together| D

    style A fill:#fb923c
    style D fill:#fde047
    style B fill:#3fb950
    style C fill:#3fb950
    style E fill:#58a6ff
    style F fill:#f85149

    note1[Why separate locks?<br/>- mu: High-frequency event ingest<br/>- resultsMu: Low-frequency command tracking<br/>- Prevents command status checks from blocking capture]

    D -.-> note1
```

## Example Usage

### AI Checking Command Status

```typescript
// AI sends command
const result = await mcp.callTool('interact', {
  action: 'execute_js',
  script: 'document.title'
});

// Returns immediately with correlation_id
console.log(result);
// {status: "queued", correlation_id: "exec_12345_67890"}

// Poll for completion
while (true) {
  const status = await mcp.callTool('observe', {
    what: 'command_result',
    correlation_id: 'exec_12345_67890'
  });

  if (status.status === 'complete') {
    console.log('Result:', status.result);
    break;
  } else if (status.status === 'expired') {
    console.error('Command timed out:', status.error);
    break;
  }

  // Still pending, wait and retry
  await sleep(2000);
}
```

### Listing All Commands

```typescript
// Get overview of all async commands
const commands = await mcp.callTool('observe', {
  what: 'pending_commands'
});

console.log(`Pending: ${commands.pending.length}`);
console.log(`Completed: ${commands.completed.length}`);
console.log(`Failed: ${commands.failed.length}`);

// Inspect failed commands
const failed = await mcp.callTool('observe', {
  what: 'failed_commands'
});

failed.commands.forEach(cmd => {
  console.log(`${cmd.correlation_id}: ${cmd.error}`);
});
```

## Performance Implications

### Memory Usage
- **completedResults map:** ~200 bytes × active commands (typical: 1-10)
- **failedCommands ring:** ~200 bytes × 100 = 20KB max
- **Total:** <50KB for typical workloads

### CPU Usage
- **Map lookup:** O(1), <1μs
- **Cleanup goroutine:** Runs every 10s, <1ms duration
- **Lock contention:** None (separate resultsMu)

### Network
- **Zero** (all localhost, in-process)

## Troubleshooting

### Command stuck in "pending"

**Possible causes:**
1. Extension crashed
2. Extension not polling (check GET /pending-queries logs)
3. Tab navigated away (extension lost reference)
4. Command in queue position 6+ (only top 5 are kept)

**Check:**
```bash
# View pending queries
curl http://localhost:7890/pending-queries

# View failed commands
observe({what: 'failed_commands'})
```

### Commands immediately expiring

**Possible causes:**
1. System clock skew (check timestamps)
2. Extension polling >30s intervals (shouldn't happen)
3. Queue overflow (>5 commands/sec)

**Check AsyncCommandTimeout:**
```bash
grep AsyncCommandTimeout internal/queries/types.go
# Should show: AsyncCommandTimeout = 30 * time.Second
```

## References

- [Implementation: queries.go](../../internal/capture/queries.go)
- [Type Definition: CommandResult](../../internal/queries/types.go)
- [Async Queue Flow Diagram](async-queue-flow.md)
- [Architecture Enforcement](../ARCHITECTURE-ENFORCEMENT.md)
