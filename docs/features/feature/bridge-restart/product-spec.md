---
doc_type: product_spec
feature_id: feature-bridge-restart
status: implemented
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Bridge Restart — Product Spec


## Problem

When the kaboom daemon hangs (deadlock, stuck goroutine, resource exhaustion), the LLM cannot call any tools because `tools/call` requests forward to the unresponsive HTTP daemon. The bridge (stdio process) is still alive — it just can't reach the daemon. There is no way to recover without manually killing the process.

## Solution

Add `configure(action="restart")` as a recovery mechanism. The bridge intercepts this specific tool call before checking daemon status, kills the daemon, and respawns a fresh one — all without the daemon needing to respond.

## User Experience

### When the daemon is unresponsive

The LLM detects repeated connection errors or timeouts from tool calls and calls:

```json
{"tool": "configure", "arguments": {"action": "restart"}}
```

The bridge handles this entirely in-process:
1. Kills the hung daemon (SIGCONT + SIGTERM + SIGKILL escalation)
2. Spawns a fresh daemon
3. Returns `{"status": "ok", "restarted": true}` once the new daemon is healthy

### When the daemon is responsive but needs a clean restart

The request reaches the daemon, which sends itself SIGTERM. The bridge detects the daemon died and auto-respawns it.

## Success Criteria

- LLM can recover from a completely hung daemon without human intervention
- Recovery completes within 6 seconds
- Works for frozen processes (SIGSTOP), deadlocked processes, and resource-exhausted processes
- Normal operation is unaffected — only `configure(action="restart")` triggers this path
