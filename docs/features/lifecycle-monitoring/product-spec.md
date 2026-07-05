---
doc_type: product-spec
feature_id: lifecycle-monitoring
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Comprehensive Lifecycle Monitoring

## Problem

When debugging production issues, operators need to understand:
- **Why did the server restart?** (Was it a crash, kill signal, or intentional stop?)
- **When did the circuit breaker open/close?** (Rate limit or memory pressure?)
- **When did the extension connect/disconnect?** (Network issue or extension crash?)

**Current state**: Limited logging makes root cause analysis difficult.

## User Stories

1. **As an operator**, I want to see why the server shut down (SIGTERM vs SIGKILL vs crash), so I can diagnose unexpected restarts.

2. **As a developer**, I want to correlate extension disconnects with circuit breaker events, so I can understand system health degradation.

3. **As an operator**, I want to see server startup metadata (version, OS, arch), so I can verify deployment state.

4. **As a developer**, I want to detect SIGKILL by observing startup→startup with no shutdown, so I can identify forced terminations.

## Requirements

### Functional

1. **Startup Events**
   - Log: `startup` with version, PID, port, go_version, os, arch, timestamp
   - Log: `loading_settings` when restoring pilot/tracking state
   - Log: `mcp_transport_ready` when MCP transport ready
   - Log: `pid_file_error` if PID file creation fails (non-fatal)

2. **Shutdown Events**
   - Log: `shutdown` with signal, signal_num, shutdown_source, uptime_seconds
   - Map signals to human-readable sources:
     - SIGINT → "Ctrl+C (SIGINT)"
     - SIGTERM → "SIGTERM (likely --stop or kill)"
     - SIGHUP → "SIGHUP (terminal closed)"
   - Log: `stop_command_invoked` when `--stop` flag used (separate entry)

3. **Circuit Breaker Events**
   - Log: `circuit_opened` with reason, streak, rate, threshold, memory_bytes
   - Reasons: "rate_exceeded" (5s over 1000 events/sec) or "memory_exceeded" (>50MB)
   - Log: `circuit_closed` with previous_reason, open_duration_secs, memory_bytes, rate

4. **Extension Events**
   - Log: `extension_connected` with session_id, is_reconnect, disconnect_seconds
   - Trigger on first connect or reconnect after >3s disconnect
   - Track session changes (new session_id → new browser session)

### Non-Functional

- All events logged to `~/kaboom-logs.jsonl` (JSON Lines format)
- Events emitted asynchronously (via goroutine) to avoid blocking
- Lifecycle callback pattern allows cross-package event emission
- SIGKILL detection via heuristic: startup→startup with no shutdown entry

## Event Schema

All lifecycle events share common structure:

```json
{
  "type": "lifecycle",
  "event": "<event_name>",
  "pid": 12345,
  "port": 7890,
  "timestamp": "2025-02-05T12:00:00Z",
  ...additional fields
}
```

### Event Types

| Event | Additional Fields |
|-------|-------------------|
| `startup` | version, go_version, os, arch |
| `shutdown` | signal, signal_num, shutdown_source, uptime_seconds |
| `stop_command_invoked` | source, caller_pid |
| `circuit_opened` | reason, streak, rate, threshold, memory_bytes |
| `circuit_closed` | previous_reason, open_duration_secs, memory_bytes, rate |
| `extension_connected` | session_id, is_reconnect, disconnect_seconds |
| `loading_settings` | (no additional fields) |
| `mcp_transport_ready` | (no additional fields) |
| `pid_file_error` | error |

## Edge Cases

1. **SIGKILL** (cannot be caught): Detect via startup→startup pattern in logs
2. **Rapid restarts**: Each gets unique startup entry with PID
3. **Long-running server**: uptime_seconds as float for precision
4. **Circuit flapping**: Multiple open/close cycles logged separately
5. **Extension session change**: New session_id logged on first poll

## Out of Scope

- Real-time event streaming (log file only, not HTTP endpoint)
- Log rotation (users handle with logrotate or similar)
- Structured error recovery (logging only, no auto-restart)
- Metrics aggregation (raw events only)

## Success Metrics

- 100% of shutdowns have corresponding log entry (except SIGKILL)
- Circuit breaker state changes logged within 1 second
- Extension reconnects detected within 3 seconds
- Zero performance impact on hot paths (async logging)

## Implementation

### Architecture

```
┌─────────────────┐
│  cmd/main.go    │
│  - startup log  │
│  - shutdown log │
│  - callback     │
└────────┬────────┘
         │ SetLifecycleCallback()
         ▼
┌─────────────────┐
│ capture.Capture │
│  - circuit      │
│  - extension    │
│  - emitEvent()  │
└─────────────────┘
```

**Key pattern**: Capture package emits events via callback, main.go logs to file. Decouples telemetry from business logic.

## Testing

1. **Startup**: Verify all startup events present in log
2. **Shutdown**: Send SIGTERM, verify shutdown entry with correct signal
3. **Circuit breaker**: Trigger rate limit, verify open→close cycle logged
4. **Extension**: Connect extension, verify connection event with session_id
5. **SIGKILL detection**: Kill server, restart, verify no shutdown entry before second startup

## Dependencies

- `os.Signal` for signal handling
- `time.Since()` for uptime calculation
- `runtime.Version()`, `runtime.GOOS`, `runtime.GOARCH` for metadata
- Callback pattern for cross-package event emission

## Status

✅ **Implemented** - Commit `7189ff4`
