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
 |                              |-- ApplyCommandResult(ann_X, "complete", result, "") ->|
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

1. **Server** (`cmd/browser-agent/internal/toolanalyze/annotationanalysis/handler.go`): When blocking annotation retrieval has no ready data, generates an `ann_<timestamp>_<random>` correlation ID, registers it as a pending command and annotation waiter, then returns the recovery handle.

2. **AnnotationStore** (`internal/annotation/store.go`): Maintains annotation waiters. When `StoreSession()` or `AppendToNamedSession()` receives annotations, it completes matching waiters through the command-completion callback.

3. **CommandTracker** (`internal/queries/dispatcher_commands.go`): Provides `WaitForCommand(correlationID, timeout)` which blocks using a `commandNotify` channel. `ApplyCommandResult(correlationID, status, result, err)` closes the channel to wake all waiters. The waiter registration uses a 10-minute TTL to give users ample drawing time.

4. **Observe handler** (`cmd/browser-agent/internal/asynccommand/handler.go`): Waits through the canonical command tracker and returns either the terminal result or the current pending lifecycle state.

5. **Bridge** (`cmd/browser-agent/internal/bridge/`): Applies the bounded MCP request timeout policy while the daemon owns command waiting and lifecycle state.

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
3. When data arrives, complete the command via `capture.ApplyCommandResult(correlationID, "complete", result, "")`.
4. LLM polls via `observe({what: "command_result", correlation_id: "..."})` which blocks for a reasonable duration

Do NOT use long timeouts or fully blocking waits for user-facing operations. The LLM should always be in control of when and how long to wait.
