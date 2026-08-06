---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-07-30
---

# Error Recovery Strategy - Kaboom MCP

**Version:** v5.8+
**Last Updated:** 2026-02-07

---

## Overview

Kaboom implements a multi-layered error recovery strategy to handle transient failures, network timeouts, and server crashes gracefully. The system combines timeout escalation, exponential backoff, circuit breakers, and fallback mechanisms to maintain reliability under adverse conditions.

## Persisted-State Diagnostic Lifecycle

Every persisted-state owner reports recovery through the canonical state
diagnostic collector. A malformed or unreadable value creates an `active`
diagnostic with redacted detail, a deterministic fallback, and an actionable
fix. The same owner must emit `recovered` only after a later read or write
verifies valid fresh state.

System Doctor retains the first and last observation times, recovery time,
occurrence count, and a bounded transition history. Recovered diagnostics remain
visible as historical health information but no longer degrade readiness.
Diagnostic fields must never include raw persisted values, paths containing
secrets, credentials, page content, prompts, URLs, or captured traffic.

---

## Timing Constants

These constants define the timing boundaries for various recovery mechanisms:

### Extension-to-Server Communication

| Constant | Value | Purpose |
|----------|-------|---------|
| `DefaultQueryTimeout` | 2s | Extension query timeout (fast fail for synchronous operations) |
| `AsyncCommandTimeout` | 30s | Async command timeout (execute_js, browser actions) |
| Bridge startup timeout | 4s | Maximum wait for daemon startup |
| Poll interval (base) | 1-2s | Extension polling frequency |
| HTTP client timeout | 500ms | Health checks and quick requests |
| Long-running handler timeout | 35s | Maximum time for expensive operations (a11y audits: 30s) |

### Circuit Breaker Defaults

| Parameter | Default | Customizable |
|-----------|---------|--------------|
| `maxFailures` | 5 | Yes |
| `resetTimeout` | 30s | Yes |
| `initialBackoff` | 1s | Yes |
| `maxBackoff` | 30s | Yes |

---

## Error Recovery Layers

### Layer 1: Bridge Mode Startup Failure

When Kaboom MCP starts, it attempts to connect to or spawn an HTTP server daemon.

#### Fast-Start Strategy

```
Startup Flow:
1. Check if server running (GET /health, timeout: 500ms)
2. If yes: Connect immediately, bridge stdio
3. If no: Spawn daemon in background (async)
4. Wait for daemon ready (timeout: 4s)
5. Start bridging requests
```

#### Failure Scenarios

**Scenario: Port Already Bound**
- Detection: `net.Listen()` fails with "address already in use"
- Recovery: Wait for server on existing port (race-safe)
- User Action: Check for stale process: `lsof -ti :7890 | xargs kill`

**Scenario: Daemon Fails to Start**
- Detection: Daemon starts but doesn't respond to health check after 4s
- Recovery: Mark as "failed" and respond to initialize/tools/list with cached data
- User Action: Check logs for daemon startup errors, retry with `--port` override

**Scenario: Daemon Crashes After Startup**
- Detection: Health check fails after initial success
- Recovery: Circuit breaker opens (see Layer 2)
- User Action: Daemon will restart on next MCP connection

---

### Layer 2: Circuit Breaker Pattern

The extension implements a circuit breaker with three states to prevent cascading failures:

#### Circuit Breaker States

```
CLOSED (normal operation)
  ↓ [after max_failures consecutive failures]
OPEN (fail fast, reject requests)
  ↓ [after reset_timeout (30s)]
HALF_OPEN (test recovery with single probe request)
  ↓ [if probe succeeds]
CLOSED (resume normal operation)
  ↓ [if probe fails]
OPEN (back to failure mode)
```

#### Exponential Backoff Schedule

When the circuit breaker is closed, retry attempts use exponential backoff:

```
Failure 1: 0ms (immediate retry)
Failure 2: 1s backoff (1s * 2^0)
Failure 3: 2s backoff (1s * 2^1)
Failure 4: 4s backoff (1s * 2^2)
Failure 5: 8s backoff (1s * 2^3)
Failure 6+: 30s backoff (capped at maxBackoff)
```

#### Circuit Breaker Events

The circuit breaker emits state change events with reasons:

- `closed` → `open`: `consecutive_failures_5` (after 5 failures)
- `open` → `half_open`: `reset_timeout_elapsed` (after 30s timeout)
- `half_open` → `closed`: `request_success` (probe succeeds)
- `half_open` → `open`: `consecutive_failures_6` (probe fails)
- Any state → `closed`: `manual_reset` (user invokes reset)

---

### Layer 3: WebSocket Timeout Escalation

For long-running browser operations (execute_js, accessibility audits), timeouts escalate:

#### Timeout Progression

```
User Request
  ↓
Extension picks up (DefaultQueryTimeout: 2s)
  ↓ [if extension doesn't respond]
MCP Client retries (escalate to 5s)
  ↓ [if still pending after 5s]
Escalate to 30s (AsyncCommandTimeout)
  ↓ [if still pending after 30s]
Return timeout error, but extension keeps processing
```

This allows long operations (a11y audits: 30s limit) to complete while MCP doesn't hang.

#### Execute JS Timeout Example

```go
// Bridge HTTP timeout for long operations
client := &http.Client{
    Timeout: 35 * time.Second,  // Exceeds longest handler (a11y: 30s)
}

// Extension-side timeout in inject.ts
async function executeJavaScript(script, timeoutMs = 5000) {
    const timeout = setTimeout(() => {
        resolve({ success: false, error: 'execution_timeout' })
    }, timeoutMs)

    try {
        const result = new Function(script)()  // May throw CSP error
        // ...handle result
    } catch (err) {
        if (err.message.includes('Content Security Policy')) {
            resolve({ success: false, error: 'csp_blocked' })
        }
    }
}
```

---

### Layer 4: Client Reconnection

When an MCP client disconnects, the persistent daemon remains available:

#### Disconnect Flow

```
MCP Client closes connection
  ↓
Bridge detects EOF on stdin
  ↓
Bridge detaches from the daemon
  ↓
Daemon keeps HTTP and extension connections available
```

#### Reconnection Strategy

1. A new MCP bridge probes the local daemon.
2. If healthy, it reconnects without restarting the daemon.
3. If unavailable, lifecycle recovery starts a canonical daemon process.
4. The extension detects readiness and resets its circuit breaker.

---

### Layer 5: Async Command Correlation

For operations that return immediately (execute_js, click, navigate), correlation tracking ensures results don't get lost:

#### Correlation Flow

```
MCP Client Request (execute_js)
  ↓
Server sends async command with correlation_id
  ↓
Return immediately with { correlation_id, status: "pending" }
  ↓
Extension executes in background
  ↓
Extension sends result with correlation_id
  ↓
Server stores in pending query buffer (30s TTL)
  ↓
MCP Client polls with command_result { correlation_id }
  ↓
Server returns completed result
```

#### Recovery Hints

| Scenario | Symptom | Recovery |
|----------|---------|----------|
| Command timeout | `status: "timeout"` after 30s | Increase timeout_ms param or break into smaller operations |
| Command expired | `status: "expired"` after 30s | Result was lost; rerun command |
| Command queued | `status: "pending"` | Still processing; wait and poll again |

---

## Connection State Machine

The extension tracks connection state across multiple dimensions:

```
ServerState:   down → booting → up
ExtensionState: disconnected → connected → active
CircuitState:  closed → open ⇄ half-open
PollingState:  stopped ↔ running
```

### State Invariants

Valid state combinations are enforced:

- Cannot be `connected` if server is `down`
- Circuit breaker cannot be `closed` during `booting`
- Polling must be `running` if extension is `connected`

Invariant violations are logged as warnings and trigger diagnostic collection.

---

## Error Type Recovery Hints

### Network Errors

**ECONNREFUSED** (Connection refused)
- Cause: Server not running or port wrong
- Recovery: Check port setting, verify server startup with `ps aux | grep kaboom`
- Timing: Immediate, don't retry

**ECONNRESET** (Connection reset by peer)
- Cause: Server crashed or network unstable
- Recovery: Circuit breaker handles automatically, exponential backoff
- Timing: Escalate from 1s → 30s over 4-5 retries

**ETIMEDOUT** (Request timeout)
- Cause: Server hung, slow network, or blocking operation
- Recovery: If < 2s, quick retry; if 2-30s, wait for exponential backoff
- Timing: Don't retry immediately (use circuit breaker)

### Bridge Mode Errors

**Daemon spawn failure**
- Error: `Failed to start daemon: {reason}`
- Recovery: Node.js wrapper spawns detached process; manual kill if zombie
- Action: `lsof -ti :7890 | xargs kill; kaboom-mcp --port 7891`

**Port already in use**
- Error: `Port already bound` or `Address already in use`
- Recovery: Wait for server, or use different port
- Action: Check with `lsof -i :7890`, kill stale process, retry

**Server not responding after spawn**
- Error: `Daemon started but not responding after 4s`
- Recovery: Assume spawn succeeded but slow; proceed with best-effort
- Action: Check daemon logs, increase startup timeout

### Execution Errors

**CSP_BLOCKED** (Content Security Policy violation)
- Error: Script execution rejected by page CSP
- Recovery: Fallback to `world: "isolated"` for DOM-only queries
- Auto-retry: `world: "auto"` handles this automatically

**EXECUTION_TIMEOUT** (Script exceeded timeout)
- Error: `Script exceeded Nms timeout`
- Recovery: Increase `timeout_ms` param or break into smaller operations
- Best practice: Keep individual operations < 2s

**EXECUTION_ERROR** (Script threw exception)
- Error: `{error_message}` with stack trace
- Recovery: Debug the script logic; test in browser console first
- Pattern: Ensure result is serializable (avoid DOM nodes, functions, symbols)

---

## Recovery Configuration

### Environment Variables

```bash
# Override port (default: 7890)
KABOOM_PORT=7891 npx kaboom-mcp

# Enable debug logging
DEBUG=kaboom:* npx kaboom-mcp
```

### Extension Configuration

Circuit breaker options can be customized when initializing:

```typescript
// In background/init.ts
const circuitBreaker = createCircuitBreaker(sendFn, {
    maxFailures: 3,           // Open after 3 failures (default: 5)
    resetTimeout: 10000,      // Reset after 10s (default: 30s)
    initialBackoff: 500,      // Start with 500ms backoff (default: 1s)
    maxBackoff: 10000,        // Cap backoff at 10s (default: 30s)
})
```

---

## Monitoring & Diagnostics

### Health Check Endpoint

```bash
curl http://localhost:7890/health
# Returns: 200 OK if server ready
# Returns: 503 if starting or shutting down
```

### Extension Debug Mode

Enable in popup → Debugging → Debug Mode to capture:
- Connection state changes
- Circuit breaker transitions
- Polling errors
- Timeout escalations
- Command correlations

### Log Analysis

```bash
# Watch for timeout patterns
grep -i "timeout\|escalate" ~kaboom-logs.jsonl | tail -20

# Count circuit breaker state changes
grep "circuit.*open\|circuit.*closed" ~/.claude/kaboom.log | wc -l

# Find slow requests
grep "duration_ms" -o | sort -n | tail -10
```

---

## Best Practices

### For Tool Implementations

1. **Respect timeout parameters**: Users may set custom `timeout_ms`
2. **Avoid blocking operations**: Long-running scripts should use async/await
3. **Handle partial results**: If timeout occurs mid-operation, return what you have
4. **Serialize carefully**: Use only JSON-safe types in execute_js results

### For Users

1. **Monitor circuit breaker state**: If open, check server health before retrying
2. **Use exponential backoff**: Don't retry immediately; let backoff escalate
3. **Break large operations**: Split a11y audits, large DOM queries into chunks
4. **Check port conflicts**: Before reporting "server won't start", verify port
5. **Enable debug mode**: When troubleshooting connection issues, export debug log

---

## Related Documents

- [Connection State Machine](../../../.claude/refs/architecture.md#connection-state-machine)
- [Async Command Architecture](../../../.claude/refs/async-command-architecture.md)
- [MCP Persistent Server](../../features/mcp-persistent-server/architecture.md)
- [Troubleshooting Guide](../../setup/troubleshooting.md)
