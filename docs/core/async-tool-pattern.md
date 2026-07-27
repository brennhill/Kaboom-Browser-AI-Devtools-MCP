---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Async Tool Pattern: Correlation ID Polling

## Problem

Some MCP tool calls involve user interaction that takes an unpredictable amount of time (e.g., draw mode annotations). The bridge between the LLM and the daemon has a 30-second HTTP timeout per request. Blocking calls that exceed this timeout will fail.

## Pattern

Instead of blocking, the server returns immediately with a `correlation_id`. The LLM polls for results using the existing `observe({what: "command_result"})` mechanism.

### Flow

```
LLM                          Server                      Extension
 |                              |                            |
 |-- analyze(wait:true) ------->|                            |
 |<-- {status: waiting,         |                            |
 |     correlation_id: ann_X}   |                            |
 |                              |                            |
 |  (LLM does other work or    |                            |
 |   polls periodically)        |                            |
 |                              |                            |
 |                              |<-- /draw-mode/complete ----|
 |                              |    (annotations + corrID)  |
 |                              |                            |
 |                              |-- CompleteCommand(ann_X) ->|
 |                              |   (stored in CommandTracker)|
 |                              |                            |
 |-- observe(command_result, -->|                            |
 |   correlation_id: ann_X)     |                            |
 |<-- {status: complete,        |                            |
 |     annotations: [...]}      |                            |
```

### Response States

When calling `observe({what: "command_result", correlation_id: "ann_..."})`:

The call **blocks for up to 55 seconds** waiting for annotations to arrive. This is token-efficient — the LLM makes one call and waits instead of rapid polling.

| Status | Meaning | Action |
|--------|---------|--------|
| `complete` | Annotations ready | Process results |
| `pending` | Still waiting (55s elapsed) | Re-issue the same observe call to wait another 55s |
| `expired` | Command TTL exceeded (10 min) | User didn't finish; retry or abort |
| Not found | Invalid or already cleaned up | Check correlation_id |

### Implementation Details

1. **Server** (`tools_analyze_annotations.go`): When `wait: true`, checks if annotations are already available. If not, generates `ann_<timestamp>_<random>` correlation_id, registers it as a pending command in CommandTracker and as a waiter in AnnotationStore, returns immediately.

2. **AnnotationStore** (`internal/annotation/store_wait.go`): Maintains annotation waiters. When `StoreSession()` or `AppendToNamedSession()` receives annotations, it completes matching waiters through the command-completion callback.

3. **CommandTracker** (`internal/capture/queries.go`): Provides `WaitForCommand(correlationID, timeout)` which blocks using a `commandNotify` channel. `CompleteCommand()` closes the channel to wake all waiters. The waiter registration uses a 10-minute TTL to give users ample drawing time.

4. **Observe handler** (`tools_observe_analysis.go`): When `correlation_id` starts with `ann_`, calls `WaitForCommand(55s)` instead of returning immediately. Returns `pending` if still waiting after 55s, or the completed result if annotations arrived.

5. **Bridge** (`bridge.go`): Detects annotation observe calls (`observe` + `command_result` + `ann_*` correlation_id) and gives them a 65s timeout (55s server wait + 10s buffer). All other calls use standard 10s/35s timeouts.

### LLM Usage Patterns

**Fire-and-forget (LLM has other work to do):**
```
analyze({what: "annotations", wait: true})  → gets correlation_id
... do other work ...
observe({what: "command_result", correlation_id: "ann_..."})  → blocks 55s or returns result
```

**Active wait (LLM wants to wait for user):**
```
analyze({what: "annotations", wait: true})  → gets correlation_id
observe({what: "command_result", correlation_id: "ann_..."})  → blocks 55s
  → if pending: re-issue same observe call
  → if complete: process annotations
```

### Applying This Pattern to New Features

Any tool that depends on user interaction should follow this pattern:

1. Return immediately with `{status: "waiting_for_user", correlation_id: "..."}`
2. Register the correlation_id in both CommandTracker and the relevant store
3. When data arrives, complete the command via `capture.CompleteCommand()`
4. LLM polls via `observe({what: "command_result", correlation_id: "..."})` which blocks for a reasonable duration

Do NOT use long timeouts or fully blocking waits for user-facing operations. The LLM should always be in control of when and how long to wait.
