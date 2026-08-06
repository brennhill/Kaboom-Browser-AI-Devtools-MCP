---
status: proposed
scope: feature/memory-enforcement/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-memory-enforcement
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-memory-enforcement.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Memory Enforcement Review](./index.md).

# Technical Spec: Memory Enforcement

## Purpose

The Kaboom server holds captured data in memory ring buffers. Without enforcement, a long-running session on a busy app can grow memory until the process is killed by the OS. The existing `isMemoryExceeded()` function detects the condition but does nothing about it — it only prevents new ingest, which means memory stays high forever until a restart.

Memory enforcement adds active eviction: when memory pressure rises, the server progressively sheds data to stay within safe limits. This keeps the server running indefinitely without manual intervention, even under sustained load.

---

## How It Works

### Three Thresholds

The server monitors total buffer memory (WebSocket events + network bodies + enhanced actions) and applies pressure responses at three levels:

| Threshold | Value | Response |
|-----------|-------|----------|
| Soft limit | 20MB | Evict oldest 25% of each buffer |
| Hard limit | 50MB | Evict oldest 50% of each buffer, reject new network bodies |
| Critical limit | 100MB | Clear all buffers completely, enter minimal mode |

### Eviction Strategy

When a threshold is crossed during an ingest operation, the server immediately evicts before processing the new data:

1. Calculate current total memory across all buffers
2. If above a threshold, evict from each buffer proportionally:
   - WebSocket events: remove oldest N entries
   - Network bodies: remove oldest N entries (these are the largest)
   - Enhanced actions: remove oldest N entries
3. Recalculate memory after eviction
4. If still above, repeat with a more aggressive ratio (up to 50%)
5. Process the incoming data normally

Eviction always removes from the oldest end. Newer data is more valuable to the AI agent.

### Minimal Mode

At the critical limit (100MB), something is seriously wrong — likely a pathological app behavior or a bug. The server:

1. Clears ALL buffers completely (WS, network, actions)
2. Sets a flag: `minimalMode = true`
3. In minimal mode, buffer capacities are halved:
   - WebSocket: 250 events (normally 500)
   - Network bodies: 50 entries (normally 100)
   - Enhanced actions: 100 entries (normally 200)
4. Minimal mode persists until the server is restarted

This prevents the same pathological pattern from immediately re-filling memory.

### Periodic Check

In addition to checking on ingest, a background goroutine runs every 10 seconds to check memory. This catches cases where memory grows from internal bookkeeping (connection trackers, pending queries) rather than ingest.

If the periodic check finds memory above the soft limit, it triggers eviction. This is a safety net — the primary enforcement happens on ingest.

### Extension-Side Memory

The extension has its own memory enforcement (separate from the server):

| Threshold | Value | Response |
|-----------|-------|----------|
| Soft limit | 20MB | Reduce buffer capacities by 50% |
| Hard limit | 50MB | Disable network body capture entirely, clear network buffer |

The extension estimates memory from buffer sizes (entry count * average entry size). It checks every 30 seconds.

---

## Data Model

### Server State

Added to the `Capture` struct:
- `minimalMode`: Boolean, whether the server entered minimal mode (persists until restart)
- `lastEvictionTime`: When the last eviction occurred (for rate-limiting eviction checks)
- `totalEvictions`: Counter of how many eviction cycles have run (for diagnostics)
- `evictedEntries`: Total entries evicted lifetime (for diagnostics)

### Memory Calculation

Each buffer estimates its memory usage:
- **WebSocket events**: Sum of `len(event.Data)` + 200 bytes overhead per entry (timestamps, metadata fields)
- **Network bodies**: Sum of `len(body.RequestBody) + len(body.ResponseBody)` + 300 bytes overhead per entry
- **Enhanced actions**: 500 bytes per entry (fixed estimate — actions are small but have nested fields)

These are estimates, not exact. The goal is order-of-magnitude correctness for threshold decisions, not precise accounting.

---

## Behavior

### On Ingest

Every time `AddWebSocketEvents`, `AddNetworkBodies`, or `AddEnhancedActions` is called:

1. Check total memory
2. If above soft limit: evict oldest 25% from each buffer
3. If above hard limit: evict oldest 50%, set memory-exceeded flag (rejects future network body POSTs until memory drops)
4. If above critical limit: clear all, enter minimal mode
5. Add the new data to buffers (which may themselves trigger ring-buffer rotation)

The memory-exceeded flag is cleared when memory drops below the hard limit (checked on the next ingest or periodic check).

### Eviction Priority

When evicting, network bodies are targeted first because they're the largest entries. The eviction order:

1. Network bodies (oldest 25-50%)
2. If still over: WebSocket events (oldest 25-50%)
3. If still over: Enhanced actions (oldest 25-50%)

This preserves the most valuable diagnostic data (recent actions and WS messages) while shedding the bulkiest data (network response bodies).

### MCP Tool: Memory Status

The existing `get_browser_logs` or a new diagnostic section exposes memory state:

```json
{
  "memory": {
    "total_bytes": 23456789,
    "websocket_bytes": 8000000,
    "network_bytes": 12000000,
    "actions_bytes": 3456789,
    "soft_limit": 20971520,
    "hard_limit": 52428800,
    "critical_limit": 104857600,
    "minimal_mode": false,
    "total_evictions": 3,
    "evicted_entries": 127
  }
}
```

This lets the AI agent understand memory pressure and adjust its behavior (e.g., calling `get_network_bodies` more frequently to consume data before eviction).

---

## Edge Cases

- **All buffers empty but memory still high**: Shouldn't happen since memory is calculated from buffer contents. If it does (bug in calculation), the periodic check will detect and evict.
- **Single giant network body (10MB response)**: The individual entry exceeds the soft limit by itself. It gets added (ring buffer rotation drops the oldest), and the next check evicts it. The per-entry size limit in network body capture (body truncation at 100KB) prevents this in practice.
- **Rapid eviction cycling**: If load is sustained at exactly the soft limit, eviction triggers every ingest. This is acceptable — eviction is O(n) where n is entries removed, and removing 25% of a 500-entry buffer is ~125 slice operations. To prevent excessive eviction, there's a 1-second cooldown between eviction cycles.
- **Minimal mode with no restart**: The server stays in minimal mode indefinitely. Buffers work at half capacity. This is intentional — if something pushed the server to 100MB, it's unsafe to return to full capacity without operator intervention.
- **Race condition during eviction**: All buffer operations already hold the Capture mutex. Eviction runs under the same write lock as ingest. No additional synchronization needed.
- **Extension and server both evicting**: They operate independently. The extension reduces what it sends; the server evicts what it holds. Both mechanisms are additive — they don't conflict.

---

## Performance Constraints

- Memory calculation: under 1ms (iterate slices, sum lengths)
- Eviction of 25% of 500 entries: under 0.5ms (slice reslicing, not copying)
- Periodic check overhead: under 1ms every 10 seconds (negligible)
- No allocations during eviction (reslicing existing slices)
- Eviction cooldown: max 1 eviction cycle per second

---

## Test Scenarios

1. Memory below soft limit → no eviction occurs
2. Memory at 21MB (above soft limit) → oldest 25% evicted from each buffer
3. Memory at 51MB (above hard limit) → oldest 50% evicted, memory-exceeded flag set
4. Memory at 101MB (above critical) → all buffers cleared, minimal mode activated
5. Minimal mode → buffer capacities halved (250 WS, 50 NB, 100 actions)
6. Minimal mode persists after memory drops → stays in minimal mode
7. Memory-exceeded flag → network body POSTs rejected with 429
8. Memory drops below hard limit → memory-exceeded flag cleared
9. Eviction targets network bodies first (highest memory per entry)
10. Eviction cooldown: two ingests within 1 second → only one eviction
11. Periodic check at soft limit → triggers eviction
12. Periodic check below soft limit → no action
13. calcTotalMemory returns sum of all buffer estimates
14. calcWSMemory estimates 200 bytes + data length per event
15. calcNBMemory estimates 300 bytes + body lengths per entry
16. After eviction, oldest entries are gone, newest preserved
17. Ring buffer rotation still works correctly after eviction
18. AddWebSocketEvents at hard limit → events rejected (not added)
19. Minimal mode + ingest → data added at reduced capacity
20. Extension soft limit (20MB) → buffer capacities halved
21. Extension hard limit (50MB) → network bodies disabled
22. Extension memory check runs every 30 seconds

---

## File Locations

Server implementation: `cmd/browser-agent/memory.go` with tests in `cmd/browser-agent/memory_test.go`.

Extension implementation: memory check in `extension/background.js` with tests in `extension-tests/memory.test.js`.

Note: The existing `isMemoryExceeded()`, `calcTotalMemory()`, `calcWSMemory()`, and `calcNBMemory()` functions in `queries.go` should be moved to `memory.go` as part of this feature.
