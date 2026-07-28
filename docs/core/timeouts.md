---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Timeout Reference

All timeouts in the Kaboom MCP stack, organized by layer. Understanding the timeout hierarchy is critical — an outer timeout shorter than an inner wait causes EOF/connection-reset errors.

## HTTP Server (`cmd/browser-agent/main_connection_mcp.go`)

| Timeout | Value | Purpose |
|---------|-------|---------|
| `ReadTimeout` | 5s | Max time to read request headers + body |
| `WriteTimeout` | 40s | Max time from request headers read to response write complete. Must exceed the longest blocking tool handler. |
| `IdleTimeout` | 120s | Keep-alive connection idle timeout |

**Why WriteTimeout is 40s:** Tool handlers block while waiting for extension round-trips. The slowest paths are annotation polling (55s, but served via streaming), interact commands (up to 35s), and screenshots (up to 20s). WriteTimeout must exceed these or the server kills the connection mid-flight, producing EOF errors on the client side.

## Bridge / stdio Proxy (`cmd/browser-agent/bridge.go`)

The bridge forwards JSON-RPC from stdin to the HTTP daemon. Each request gets a context timeout based on the tool type:

| Category | Timeout | Tools |
|----------|---------|-------|
| Fast | 10s | `observe` (most), `generate`, `configure`, `resources/read`, non-tool calls |
| Slow | 35s | `analyze`, `interact`, `observe screenshot` |
| Blocking poll | 65s | `observe command_result` with `ann_*` correlation ID (annotation polling) |

**Classification logic:** `toolCallTimeout()` inspects the JSON-RPC params to determine category. Screenshot observe is classified as slow because it round-trips to the extension (sync poll + capture + upload).

## Extension Sync Client (`src/background/sync/sync-client.ts`)

| Timeout | Value | Purpose |
|---------|-------|---------|
| Fetch `AbortController` | 8s | Max time for a single `/sync` POST. Server holds up to 5s for long-poll + 3s margin. |
| Retry delay | 1s | Fixed delay between sync attempts after failure |
| Disconnect threshold | 2 failures | Consecutive failures before marking disconnected (prevents flapping from single transient timeouts) |

## Server-Side Query/Command Timeouts

| Timeout | Value | Location | Purpose |
|---------|-------|----------|---------|
| Default query timeout | 2s | `internal/queries/types.go` | Generic extension queries |
| Async command timeout | 60s | `internal/queries/types.go` | Browser actions, execute_js, etc. |
| Screenshot query timeout | 20s | `tools_observe_analysis.go` | Screenshot capture + upload round-trip |
| Sync long-poll hold | 5s | `internal/capture/sync.go` | Server holds `/sync` response waiting for pending queries |
| Query result TTL | 60s | `internal/capture/queries.go` | Cleanup interval for uncollected query results |

## Extension Disconnect Detection (`internal/capture/`)

| Threshold | Value | Field | Location |
|-----------|-------|-------|----------|
| `extensionDisconnectThreshold` | 10s | `lastSyncSeen` | `constants.go` — used by `IsExtensionConnected()` and `GetPilotStatus()` |
| Reconnect detection | 3s | `lastPollAt` | `sync.go` — detects reconnection after gap |

## Body Size Limits

| Endpoint | Limit | Location |
|----------|-------|----------|
| `/mcp` | 10 MB | `handler.go` |
| `/screenshots` | 10 MB | `server.go` |
| `/sync` | 5 MB | `constants.go` |
| `/logs` | 10 MB | `server.go` |

## Timeout Hierarchy Rule

For any blocking tool call path, the timeouts must satisfy:

```
WriteTimeout > Bridge timeout > Handler wait timeout > Extension round-trip time
     40s            35s              20s (screenshot)     ~7-10s typical
```

If an outer timeout fires before an inner wait completes, the connection is killed and the client sees an EOF error. When adding new blocking tool handlers, ensure they fit within this hierarchy.
